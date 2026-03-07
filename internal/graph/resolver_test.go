package graph

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/graph/model"
	"github.com/DIMO-Network/fetch-api/internal/identity"
	"github.com/DIMO-Network/token-exchange-api/pkg/tokenclaims"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireVehicleOptsByDID(t *testing.T) {
	testDID := cloudevent.ERC721DID{
		ChainID:         137,
		ContractAddress: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		TokenID:         big.NewInt(1),
	}
	didStr := testDID.String()

	t.Run("returns search options with subject DID", func(t *testing.T) {
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetRawData)
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, didStr, nil)
		require.NoError(t, err)
		require.NotNil(t, opts)
		require.NotNil(t, opts.Subject)
		assert.Equal(t, didStr, opts.Subject.Value)
	})

	t.Run("applies filter to search options", func(t *testing.T) {
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetRawData)
		filter := &model.CloudEventFilter{
			Type: ptr("dimo.status"),
		}
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, didStr, filter)
		require.NoError(t, err)
		require.NotNil(t, opts)
		require.NotNil(t, opts.Type)
		assert.Equal(t, "dimo.status", opts.Type.Value)
		assert.Equal(t, didStr, opts.Subject.Value)
	})

	t.Run("unauthorized when token does not match DID", func(t *testing.T) {
		otherDID := cloudevent.ERC721DID{
			ChainID:         137,
			ContractAddress: common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			TokenID:         big.NewInt(999),
		}.String()
		ctx := contextWithToken(otherDID, tokenclaims.PermissionGetRawData)
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, didStr, nil)
		assert.Error(t, err)
		assert.Nil(t, opts)
	})

	t.Run("invalid DID returns unauthorized", func(t *testing.T) {
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetRawData)
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, "not-a-valid-did", nil)
		require.Error(t, err)
		assert.Nil(t, opts)
		assert.Contains(t, err.Error(), "does not have access to this subject")
	})

	t.Run("missing GetRawData permission returns required permission", func(t *testing.T) {
		ctx := contextWithToken(didStr)
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, didStr, nil)
		require.Error(t, err)
		assert.Nil(t, opts)
		assert.Contains(t, err.Error(), "required permission for this operation")
	})

	t.Run("developer license with both location and non-location history allowed", func(t *testing.T) {
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetLocationHistory, tokenclaims.PermissionGetNonLocationHistory)
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, didStr, nil)
		require.NoError(t, err)
		require.NotNil(t, opts)
		assert.Equal(t, didStr, opts.Subject.Value)
	})

	t.Run("only GetLocationHistory without GetNonLocationHistory denied", func(t *testing.T) {
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetLocationHistory)
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, didStr, nil)
		require.Error(t, err)
		assert.Nil(t, opts)
		assert.Contains(t, err.Error(), "required permission for this operation")
	})

	t.Run("no token claims returns unauthorized immediately", func(t *testing.T) {
		ctx := context.Background()
		deviceDID := cloudevent.ERC721DID{
			ChainID:         137,
			ContractAddress: common.HexToAddress("0x9c94C395cBcBDe662235E0A9d3bB87Ad708561BA"),
			TokenID:         big.NewInt(31694),
		}.String()
		r := &Resolver{
			IdentityClient: &stubIdentityClient{err: fmt.Errorf("device not found: invalid contract address")},
		}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Nil(t, opts)
		assert.Contains(t, err.Error(), "no token claims")
	})
}

func ptr(s string) *string { return &s }

func contextWithToken(asset string, permissions ...string) context.Context {
	return context.WithValue(context.Background(), ClaimsContextKey{}, &tokenclaims.Token{
		CustomClaims: tokenclaims.CustomClaims{
			Asset:       asset,
			Permissions: permissions,
		},
	})
}

// stubIdentityClient is a test double for identity.Client.
type stubIdentityClient struct {
	vehicleDID string
	err        error
}

var _ identity.Client = (*stubIdentityClient)(nil)

func (s *stubIdentityClient) GetLinkedDIDForDevice(_ context.Context, _ string) (string, error) {
	return s.vehicleDID, s.err
}

func TestRequireVehicleOptsByDID_DeviceDID(t *testing.T) {
	vehicleAddr := common.HexToAddress("0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	vehicleDID := cloudevent.ERC721DID{
		ChainID:         137,
		ContractAddress: vehicleAddr,
		TokenID:         big.NewInt(42),
	}.String()

	deviceAddr := common.HexToAddress("0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB")
	deviceDID := cloudevent.ERC721DID{
		ChainID:         137,
		ContractAddress: deviceAddr,
		TokenID:         big.NewInt(7),
	}.String()

	t.Run("device DID resolves to vehicle the token has access to", func(t *testing.T) {
		ctx := contextWithToken(vehicleDID, tokenclaims.PermissionGetRawData)
		r := &Resolver{
			IdentityClient: &stubIdentityClient{vehicleDID: vehicleDID},
		}
		q := &queryResolver{r}
		opts, err := q.requireSubjectOptsByDID(ctx, deviceDID, nil)
		require.NoError(t, err)
		require.NotNil(t, opts)
		// Subject must be the device DID, not the vehicle DID.
		assert.Equal(t, deviceDID, opts.Subject.Value)
	})

	t.Run("device DID with no identity client returns error", func(t *testing.T) {
		ctx := contextWithToken(vehicleDID, tokenclaims.PermissionGetRawData)
		r := &Resolver{
			IdentityClient: nil,
		}
		q := &queryResolver{r}
		_, err := q.requireSubjectOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not have access to this subject")
	})

	t.Run("identity-api returns error for device DID", func(t *testing.T) {
		ctx := contextWithToken(vehicleDID, tokenclaims.PermissionGetRawData)
		r := &Resolver{
			IdentityClient: &stubIdentityClient{err: fmt.Errorf("device not found")},
		}
		q := &queryResolver{r}
		_, err := q.requireSubjectOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not have access to this subject")
	})

	t.Run("token does not have access to the resolved vehicle", func(t *testing.T) {
		otherVehicleDID := cloudevent.ERC721DID{
			ChainID:         137,
			ContractAddress: vehicleAddr,
			TokenID:         big.NewInt(999),
		}.String()
		ctx := contextWithToken(otherVehicleDID, tokenclaims.PermissionGetRawData)
		r := &Resolver{
			IdentityClient: &stubIdentityClient{vehicleDID: vehicleDID},
		}
		q := &queryResolver{r}
		_, err := q.requireSubjectOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not have access to this subject")
	})

	t.Run("token missing GetRawData for resolved vehicle", func(t *testing.T) {
		ctx := contextWithToken(vehicleDID)
		r := &Resolver{
			IdentityClient: &stubIdentityClient{vehicleDID: vehicleDID},
		}
		q := &queryResolver{r}
		_, err := q.requireSubjectOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required permission for this operation")
	})
}
