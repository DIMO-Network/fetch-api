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
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// ClaimsContextKey is the context key for token claims (set by JWT middleware).
type ClaimsContextKey struct{}

type Resolver struct {
	EventService   *eventrepo.Service
	Buckets        []string
	IdentityClient identity.Client
}

const (
	errNoTokenClaims     = "unauthorized: no token claims"
	errNoPermission      = "unauthorized: token does not have required permission for this operation"
	errNoAccessToSubject = "unauthorized: token does not have access to this subject"
)

// requireSubjectOptsByDID validates raw-data access and returns search options for the DID.
// requestedDID: the DID from the client (e.g. cloudEvents(did: "...")).
// tokenSubjectDID: the DID the JWT grants access to (tok.Asset).
func (r *queryResolver) requireSubjectOptsByDID(ctx context.Context, requestedDID string, filter *model.CloudEventFilter) (*grpc.SearchOptions, error) {
	token, err := requireRawDataToken(ctx)
	if err != nil {
		return nil, err
	}
	tokenSubjectDID := token.Asset // DID the JWT permits access to
	searchSubject, err := r.ensureRequestedDIDLinkedToPermissionedSubject(ctx, requestedDID, tokenSubjectDID)
	if err != nil {
		return nil, err
	}
	return filterToSearchOptions(filter, searchSubject), nil
}

// requireRawDataToken returns the token if the context has claims and the token has raw-data permission.
// Call this first so unauthenticated or insufficient-permission requests get a clear error before any DID resolution.
func requireRawDataToken(ctx context.Context) (*tokenclaims.Token, error) {
	tok, _ := ctx.Value(ClaimsContextKey{}).(*tokenclaims.Token)
	if tok == nil {
		return nil, fmt.Errorf("%s", errNoTokenClaims)
	}
	hasGetRawData := slices.Contains(tok.Permissions, tokenclaims.PermissionGetRawData)
	hasLocationHistory := slices.Contains(tok.Permissions, tokenclaims.PermissionGetLocationHistory)
	hasNonLocationHistory := slices.Contains(tok.Permissions, tokenclaims.PermissionGetNonLocationHistory)
	hasAllTimeData := hasLocationHistory && hasNonLocationHistory
	if !hasGetRawData && !hasAllTimeData {
		return nil, fmt.Errorf("%s", errNoPermission)
	}
	return tok, nil
}

// ensureRequestedDIDLinkedToPermissionedSubject verifies the client-requested DID is allowed by the token.
// requestedDID: the DID from the query (e.g. cloudEvents(did: "...")).
// tokenSubjectDID: the DID the JWT grants access to (tok.Asset).
func (r *queryResolver) ensureRequestedDIDLinkedToPermissionedSubject(ctx context.Context, requestedDID string, tokenSubjectDID string) (cloudevent.ERC721DID, error) {
	requestedDIDParsed, err := cloudevent.DecodeERC721DID(requestedDID)
	if err != nil {
		return cloudevent.ERC721DID{}, fmt.Errorf("%s", errNoAccessToSubject)
	}
	if requestedDID == tokenSubjectDID {
		return requestedDIDParsed, nil
	}
	if r.IdentityClient == nil {
		return cloudevent.ERC721DID{}, fmt.Errorf("%s", errNoAccessToSubject)
	}
	linkedDID, err := r.IdentityClient.GetLinkedDIDForDevice(ctx, requestedDIDParsed.String())
	if err != nil || linkedDID != tokenSubjectDID {
		return cloudevent.ERC721DID{}, fmt.Errorf("%s", errNoAccessToSubject)
	}
	return requestedDIDParsed, nil
}
