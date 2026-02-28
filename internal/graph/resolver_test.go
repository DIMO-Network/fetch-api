package graph

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/identity"
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
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetRawData)
		err := CheckVehicleRawDataByDID(ctx, didStr)
		assert.NoError(t, err)
	})

	t.Run("invalid DID", func(t *testing.T) {
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetRawData)
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
		ctx := contextWithToken(otherDID, tokenclaims.PermissionGetRawData)
		err := CheckVehicleRawDataByDID(ctx, didStr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not have access to this vehicle")
	})

	t.Run("missing GetRawData permission", func(t *testing.T) {
		ctx := contextWithToken(didStr)
		err := CheckVehicleRawDataByDID(ctx, didStr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token does not have access to this vehicle")
	})

	t.Run("developer license with both location and non-location history allowed", func(t *testing.T) {
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetLocationHistory, tokenclaims.PermissionGetNonLocationHistory)
		err := CheckVehicleRawDataByDID(ctx, didStr)
		require.NoError(t, err)
	})

	t.Run("only GetLocationHistory without GetNonLocationHistory denied", func(t *testing.T) {
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetLocationHistory)
		err := CheckVehicleRawDataByDID(ctx, didStr)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token does not have access to this vehicle")
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
		ctx := contextWithToken(didStr, tokenclaims.PermissionGetRawData)
		r := &Resolver{VehicleAddr: common.HexToAddress("0x1234567890123456789012345678901234567890")}
		q := &queryResolver{r}
		opts, err := q.requireVehicleOptsByDID(ctx, didStr, nil)
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
		r := &Resolver{VehicleAddr: common.HexToAddress("0x1234567890123456789012345678901234567890")}
		q := &queryResolver{r}
		opts, err := q.requireVehicleOptsByDID(ctx, didStr, filter)
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
		r := &Resolver{VehicleAddr: common.HexToAddress("0x1234567890123456789012345678901234567890")}
		q := &queryResolver{r}
		opts, err := q.requireVehicleOptsByDID(ctx, didStr, nil)
		assert.Error(t, err)
		assert.Nil(t, opts)
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

func (s *stubIdentityClient) GetVehicleDIDForDevice(_ context.Context, _ string) (string, error) {
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
			VehicleAddr:    vehicleAddr,
			IdentityClient: &stubIdentityClient{vehicleDID: vehicleDID},
		}
		q := &queryResolver{r}
		opts, err := q.requireVehicleOptsByDID(ctx, deviceDID, nil)
		require.NoError(t, err)
		require.NotNil(t, opts)
		// Subject must be the device DID, not the vehicle DID.
		assert.Equal(t, deviceDID, opts.Subject.Value)
	})

	t.Run("device DID with no identity client returns error", func(t *testing.T) {
		ctx := contextWithToken(vehicleDID, tokenclaims.PermissionGetRawData)
		r := &Resolver{
			VehicleAddr:    vehicleAddr,
			IdentityClient: nil,
		}
		q := &queryResolver{r}
		_, err := q.requireVehicleOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("identity-api returns error for device DID", func(t *testing.T) {
		ctx := contextWithToken(vehicleDID, tokenclaims.PermissionGetRawData)
		r := &Resolver{
			VehicleAddr:    vehicleAddr,
			IdentityClient: &stubIdentityClient{err: fmt.Errorf("device not found")},
		}
		q := &queryResolver{r}
		_, err := q.requireVehicleOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device not found")
	})

	t.Run("token does not have access to the resolved vehicle", func(t *testing.T) {
		otherVehicleDID := cloudevent.ERC721DID{
			ChainID:         137,
			ContractAddress: vehicleAddr,
			TokenID:         big.NewInt(999),
		}.String()
		ctx := contextWithToken(otherVehicleDID, tokenclaims.PermissionGetRawData)
		r := &Resolver{
			VehicleAddr:    vehicleAddr,
			IdentityClient: &stubIdentityClient{vehicleDID: vehicleDID},
		}
		q := &queryResolver{r}
		_, err := q.requireVehicleOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not have access to this vehicle")
	})

	t.Run("token missing GetRawData for resolved vehicle", func(t *testing.T) {
		ctx := contextWithToken(vehicleDID)
		r := &Resolver{
			VehicleAddr:    vehicleAddr,
			IdentityClient: &stubIdentityClient{vehicleDID: vehicleDID},
		}
		q := &queryResolver{r}
		_, err := q.requireVehicleOptsByDID(ctx, deviceDID, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token does not have access to this vehicle")
	})
}
