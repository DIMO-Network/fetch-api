package graph

import (
	"context"
	"math/big"
	"testing"

	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/graph/model"
	"github.com/DIMO-Network/token-exchange-api/pkg/tokenclaims"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckVehicleRawDataByDID(t *testing.T) {
	did := cloudevent.ERC721DID{
		ChainID:         137,
		ContractAddress: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		TokenID:         big.NewInt(1),
	}
	didStr := did.String()
	require.NotEmpty(t, didStr, "DID string must not be empty")

	t.Run("valid DID and matching token with GetRawData", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, &tokenclaims.Token{
			CustomClaims: tokenclaims.CustomClaims{
				Asset:       didStr,
				Permissions: []string{tokenclaims.PermissionGetRawData},
			},
		})
		err := CheckVehicleRawDataByDID(ctx, didStr)
		assert.NoError(t, err)
	})

	t.Run("invalid DID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, &tokenclaims.Token{
			CustomClaims: tokenclaims.CustomClaims{
				Asset:       didStr,
				Permissions: []string{tokenclaims.PermissionGetRawData},
			},
		})
		err := CheckVehicleRawDataByDID(ctx, "not-a-valid-did")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid DID")
	})

	t.Run("no token claims", func(t *testing.T) {
		ctx := context.Background()
		err := CheckVehicleRawDataByDID(ctx, didStr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no token claims")
	})

	t.Run("token asset does not match requested DID", func(t *testing.T) {
		otherDID := cloudevent.ERC721DID{
			ChainID:         137,
			ContractAddress: common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			TokenID:         big.NewInt(999),
		}.String()
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, &tokenclaims.Token{
			CustomClaims: tokenclaims.CustomClaims{
				Asset:       otherDID,
				Permissions: []string{tokenclaims.PermissionGetRawData},
			},
		})
		err := CheckVehicleRawDataByDID(ctx, didStr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not have access to this vehicle")
	})

	t.Run("missing GetRawData permission", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, &tokenclaims.Token{
			CustomClaims: tokenclaims.CustomClaims{
				Asset:       didStr,
				Permissions: []string{},
			},
		})
		err := CheckVehicleRawDataByDID(ctx, didStr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing GetRawData privilege")
	})
}

func TestRequireVehicleOptsByDID(t *testing.T) {
	did := cloudevent.ERC721DID{
		ChainID:         137,
		ContractAddress: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		TokenID:         big.NewInt(1),
	}
	didStr := did.String()

	t.Run("returns search options with subject DID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, &tokenclaims.Token{
			CustomClaims: tokenclaims.CustomClaims{
				Asset:       didStr,
				Permissions: []string{tokenclaims.PermissionGetRawData},
			},
		})
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireVehicleOptsByDID(ctx, didStr, nil)
		require.NoError(t, err)
		require.NotNil(t, opts)
		require.NotNil(t, opts.Subject)
		assert.Equal(t, []string{didStr}, opts.Subject.In)
	})

	t.Run("applies filter to search options", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, &tokenclaims.Token{
			CustomClaims: tokenclaims.CustomClaims{
				Asset:       didStr,
				Permissions: []string{tokenclaims.PermissionGetRawData},
			},
		})
		filter := &model.CloudEventFilter{
			Type: ptr("dimo.status"),
		}
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireVehicleOptsByDID(ctx, didStr, filter)
		require.NoError(t, err)
		require.NotNil(t, opts)
		require.NotNil(t, opts.Type)
		assert.Equal(t, []string{"dimo.status"}, opts.Type.In)
		assert.Equal(t, []string{didStr}, opts.Subject.In)
	})

	t.Run("unauthorized when token does not match DID", func(t *testing.T) {
		otherDID := cloudevent.ERC721DID{
			ChainID:         137,
			ContractAddress: common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
			TokenID:         big.NewInt(999),
		}.String()
		ctx := context.WithValue(context.Background(), ClaimsContextKey{}, &tokenclaims.Token{
			CustomClaims: tokenclaims.CustomClaims{
				Asset:       otherDID,
				Permissions: []string{tokenclaims.PermissionGetRawData},
			},
		})
		r := &Resolver{}
		q := &queryResolver{r}
		opts, err := q.requireVehicleOptsByDID(ctx, didStr, nil)
		assert.Error(t, err)
		assert.Nil(t, opts)
	})
}

func ptr(s string) *string { return &s }
