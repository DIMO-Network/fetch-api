package graph

import (
	"context"
	"fmt"
	"slices"

	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/graph/model"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/token-exchange-api/pkg/tokenclaims"
	"github.com/ethereum/go-ethereum/common"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

// ClaimsContextKey is the context key for token claims (set by JWT middleware).
type ClaimsContextKey struct{}

type Resolver struct {
	EventService *eventrepo.Service
	Buckets      []string
	VehicleAddr  common.Address
	ChainID      uint64
}

// CheckVehicleRawDataByDID returns an error if the context does not have claims or the token
// does not have GetRawData permission for the given vehicle DID, or if the DID is invalid.
func CheckVehicleRawDataByDID(ctx context.Context, did string) error {
	if _, err := cloudevent.DecodeERC721DID(did); err != nil {
		return fmt.Errorf("invalid DID: %w", err)
	}
	tok, _ := ctx.Value(ClaimsContextKey{}).(*tokenclaims.Token)
	if tok == nil {
		return fmt.Errorf("unauthorized: no token claims")
	}
	if tok.Asset != did {
		return fmt.Errorf("unauthorized: token does not have access to this vehicle")
	}
	if !slices.Contains(tok.Permissions, tokenclaims.PermissionGetRawData) {
		return fmt.Errorf("unauthorized: missing GetRawData privilege")
	}
	return nil
}

// requireVehicleOptsByDID validates access by DID and returns advanced search options for the vehicle.
func (r *queryResolver) requireVehicleOptsByDID(ctx context.Context, did string, filter *model.CloudEventFilter) (*grpc.AdvancedSearchOptions, error) {
	if err := CheckVehicleRawDataByDID(ctx, did); err != nil {
		return nil, err
	}
	subject, err := cloudevent.DecodeERC721DID(did)
	if err != nil {
		return nil, fmt.Errorf("invalid DID: %w", err)
	}
	return filterToAdvancedSearchOptions(filter, subject), nil
}
