package graph

import (
	"context"
	"fmt"
	"slices"

	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/graph/model"
	"github.com/DIMO-Network/fetch-api/internal/identity"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/token-exchange-api/pkg/tokenclaims"
	"github.com/ethereum/go-ethereum/common"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// ClaimsContextKey is the context key for token claims (set by JWT middleware).
type ClaimsContextKey struct{}

type Resolver struct {
	EventService   *eventrepo.Service
	Buckets        []string
	VehicleAddr    common.Address
	ChainID        uint64
	IdentityClient identity.Client
}

// CheckVehicleRawDataByDID returns an error if the context does not have claims or the token
// does not have GetRawData permission for the given vehicle DID, or if the DID is invalid.
func CheckVehicleRawDataByDID(ctx context.Context, did string) error {
	if _, err := cloudevent.DecodeERC721DID(did); err != nil {
		return fmt.Errorf("invalid DID: %w", err)
	}
	return checkRawDataPermissionsForVehicle(ctx, did)
}

// checkRawDataPermissionsForVehicle verifies the context token has access for vehicleDID.
// vehicleDID must already be validated.
// Allowed: GetRawData, or both GetLocationHistory (all-time location) and GetNonLocationHistory.
func checkRawDataPermissionsForVehicle(ctx context.Context, vehicleDID string) error {
	tok, _ := ctx.Value(ClaimsContextKey{}).(*tokenclaims.Token)
	if tok == nil {
		return fmt.Errorf("unauthorized: no token claims")
	}
	if tok.Asset != vehicleDID {
		return fmt.Errorf("unauthorized: token does not have access to this vehicle")
	}
	hasGetRawData := slices.Contains(tok.Permissions, tokenclaims.PermissionGetRawData)
	hasLocationHistory := slices.Contains(tok.Permissions, tokenclaims.PermissionGetLocationHistory)
	hasNonLocationHistory := slices.Contains(tok.Permissions, tokenclaims.PermissionGetNonLocationHistory)
	hasAllTimeData := hasLocationHistory && hasNonLocationHistory
	if !hasGetRawData && !hasAllTimeData {
		return fmt.Errorf("unauthorized: token does not have required permission for this vehicle operation")
	}
	return nil
}

// requireVehicleOptsByDID validates raw-data access and returns search options for the DID.
//
// If did belongs to the vehicle NFT contract (r.VehicleAddr), the token asset must match that
// vehicle DID directly. Otherwise the DID is treated as a device DID: identity-api is called
// to resolve the paired vehicle, and the token must have access to that vehicle. Either way,
// the search subject is always the originally requested DID.
func (r *queryResolver) requireVehicleOptsByDID(ctx context.Context, did string, filter *model.CloudEventFilter) (*grpc.SearchOptions, error) {
	decoded, err := cloudevent.DecodeERC721DID(did)
	if err != nil {
		return nil, fmt.Errorf("invalid DID: %w", err)
	}

	var vehicleDID string
	if decoded.ContractAddress == r.VehicleAddr {
		vehicleDID = did
	} else {
		if r.IdentityClient == nil {
			return nil, fmt.Errorf("device DID lookup not configured: no identity client")
		}
		vehicleDID, err = r.IdentityClient.GetVehicleDIDForDevice(ctx, did)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve vehicle for device DID: %w", err)
		}
	}

	if err := checkRawDataPermissionsForVehicle(ctx, vehicleDID); err != nil {
		return nil, err
	}

	return filterToSearchOptions(filter, decoded), nil
}
