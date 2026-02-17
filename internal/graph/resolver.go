package graph

import (
	"context"
	"fmt"
	"math/big"
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

// CheckVehicleRawData returns an error if the context does not have claims or the token
// does not have GetRawData permission for the given vehicle tokenID.
// requireVehicleOpts validates access and converts the filter into search options.
// Placed here (not in resolvers) so gqlgen regeneration doesn't discard it.
func (r *queryResolver) requireVehicleOpts(ctx context.Context, tokenID int, filter *model.CloudEventFilter) (*grpc.SearchOptions, error) {
	if err := CheckVehicleRawData(ctx, r.VehicleAddr, tokenID); err != nil {
		return nil, err
	}
	return filterToSearchOptions(filter, subjectFromTokenID(r.VehicleAddr, r.ChainID, tokenID)), nil
}

// CheckVehicleRawData returns an error if the context does not have claims or the token
// does not have GetRawData permission for the given vehicle tokenID.
func CheckVehicleRawData(ctx context.Context, vehicleAddr common.Address, tokenID int) error {
	tok, _ := ctx.Value(ClaimsContextKey{}).(*tokenclaims.Token)
	if tok == nil {
		return fmt.Errorf("unauthorized: no token claims")
	}
	assetDID, err := cloudevent.DecodeERC721DID(tok.Asset)
	if err != nil {
		return fmt.Errorf("unauthorized: invalid asset")
	}
	if assetDID.ContractAddress != vehicleAddr {
		return fmt.Errorf("unauthorized: wrong contract")
	}
	if assetDID.TokenID.Cmp(big.NewInt(int64(tokenID))) != 0 {
		return fmt.Errorf("unauthorized: token ID mismatch")
	}
	if !slices.Contains(tok.Permissions, tokenclaims.PermissionGetRawData) {
		return fmt.Errorf("unauthorized: missing GetRawData privilege")
	}
	return nil
}
