package graph

import (
	"context"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	chconfig "github.com/DIMO-Network/clickhouse-infra/pkg/connect/config"
	"github.com/DIMO-Network/clickhouse-infra/pkg/container"
	"github.com/DIMO-Network/cloudevent"
	chindexer "github.com/DIMO-Network/cloudevent/clickhouse"
	"github.com/DIMO-Network/cloudevent/clickhouse/migrations"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/DIMO-Network/token-exchange-api/pkg/tokenclaims"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLatestCloudEvent_AliasedEmptySourceDoesNotTankSiblings is the regression
// test for the nickname-summary bug: getName batches per-source `latestCloudEvent`
// lookups as GraphQL aliases in one request. When `latestCloudEvent` (and
// `latestIndex`) were non-nullable, a source with no matching event returned an
// error which GraphQL null-propagated up to the root `data`, wiping every
// sibling alias that DID resolve — so a vehicle whose owner had a nickname but
// whose dev-license source did not returned no name at all.
//
// The fields are now nullable and the resolvers return null (not an error) on
// no-rows, so one empty source must leave its siblings intact. Exercised against
// `latestIndex` because it needs only ClickHouse (no blob buckets), but the fix
// and codepath are identical for `latestCloudEvent`.
func TestLatestCloudEvent_AliasedEmptySourceDoesNotTankSiblings(t *testing.T) {
	ctx := context.Background()

	ch, err := container.CreateClickHouseContainer(ctx, chconfig.Settings{User: "default", Database: "dimo"})
	require.NoError(t, err)
	t.Cleanup(func() { ch.Terminate(context.Background()) })

	chDB, err := ch.GetClickhouseAsDB()
	require.NoError(t, err)
	require.NoError(t, migrations.RunGoose(ctx, []string{"up"}, chDB))

	conn, err := ch.GetClickHouseAsConn()
	require.NoError(t, err)

	did := cloudevent.ERC721DID{
		ChainID:         137,
		ContractAddress: common.HexToAddress("0xbA5738a18d83D41847dfFbDC6101d37C69c9B0cF"),
		TokenID:         big.NewInt(22892),
	}.String()

	const (
		nickType    = "dimo.document.nickname"
		ownerSource = "0x3Afb7b82D912743F8Fd15f67a9b2095e9d5AD576" // has an event
		devSource   = "0x299671D2b32ED62Cc61ce65D8f2b9e4f78486B37" // no event
	)

	// Only the owner source has a nickname event.
	ownerEvent := &cloudevent.CloudEventHeader{
		ID:          "owner-nickname-evt",
		Source:      ownerSource,
		Subject:     did,
		Type:        nickType,
		Time:        time.Now(),
		DataVersion: "nickname/v1.0.0",
	}
	require.NoError(t, conn.Exec(ctx, chindexer.InsertStmt, chindexer.CloudEventToSlice(ownerEvent)...))

	svc := eventrepo.New(conn, nil, nil, "", "")
	es := NewExecutableSchema(Config{Resolvers: &Resolver{EventService: svc}})
	srv := handler.New(es)
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})

	gql := client.New(injectClaims(srv, did))

	const query = `
		query($did: String!, $owner: CloudEventFilter, $dev: CloudEventFilter) {
			owner: latestIndex(did: $did, filter: $owner) { header { source type } }
			dev:   latestIndex(did: $did, filter: $dev)   { header { source } }
		}`

	var resp struct {
		Owner *struct {
			Header struct {
				Source string
				Type   string
			}
		}
		Dev *struct {
			Header struct{ Source string }
		}
	}

	err = gql.Post(
		query,
		&resp,
		client.Var("did", did),
		client.Var("owner", map[string]any{"source": ownerSource, "type": nickType}),
		client.Var("dev", map[string]any{"source": devSource, "type": nickType}),
	)

	// Before the fix: the empty `dev` alias erred on a non-null field, the null
	// propagated to root `data`, and this Post returned a GraphQL error with no
	// data — the owner nickname was lost.
	require.NoError(t, err, "empty source must not tank sibling aliases")
	require.NotNil(t, resp.Owner, "owner's event must survive a sibling empty source")
	assert.Equal(t, ownerSource, resp.Owner.Header.Source)
	assert.Equal(t, nickType, resp.Owner.Header.Type)
	assert.Nil(t, resp.Dev, "a source with no event must resolve to null, not error")
}

// injectClaims wraps a handler with the raw-data token the resolvers require,
// scoped to `asset` (the DID under test) so requireSubjectOptsByDID admits it.
func injectClaims(next http.Handler, asset string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := &tokenclaims.Token{
			CustomClaims: tokenclaims.CustomClaims{
				Asset:       asset,
				Permissions: []string{tokenclaims.PermissionGetRawData},
			},
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ClaimsContextKey{}, tok)))
	})
}
