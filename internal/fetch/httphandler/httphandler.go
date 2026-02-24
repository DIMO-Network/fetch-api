// Package httphandler provides the HTTP handler for the fetch service.
package httphandler

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DIMO-Network/cloudevent"
	"github.com/DIMO-Network/fetch-api/internal/fetch"
	"github.com/DIMO-Network/fetch-api/internal/graph"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	"github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/server-garage/pkg/fibercommon/jwtmiddleware"
	"github.com/DIMO-Network/server-garage/pkg/richerrors"
	"github.com/DIMO-Network/token-exchange-api/pkg/tokenclaims"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	TokenIDParam = "tokenId"
	errTimeout   = "request exceeded or is estimated to exceed the maximum execution time"
)

// cloudReturn is used by swag for OpenAPI response types (see @Success annotations).
type cloudReturn cloudevent.CloudEvent[eventrepo.ObjectInfo]

var _ = (*cloudReturn)(nil) // ensure type is used for staticcheck

// Handler is the HTTP handler for the fetch service.
type Handler struct {
	eventService *eventrepo.Service
	buckets      []string
	vehicleAddr  common.Address
	chainID      uint64
}

type searchParams struct {
	ID       *string   `query:"id"`
	Type     *string   `query:"type"`
	Source   *string   `query:"source"`
	Producer *string   `query:"producer"`
	Before   time.Time `query:"before"`
	After    time.Time `query:"after"`
	Limit    int       `query:"limit"`
}

func (s *searchParams) toSearchOptions(subject cloudevent.ERC721DID) *grpc.SearchOptions {
	opts := &grpc.SearchOptions{}
	if s.ID != nil {
		opts.Id = &wrapperspb.StringValue{Value: *s.ID}
	}
	opts.Subject = &wrapperspb.StringValue{Value: subject.String()}

	if s.Type != nil {
		opts.Type = &wrapperspb.StringValue{Value: *s.Type}
	}
	if s.Source != nil {
		opts.Source = &wrapperspb.StringValue{Value: *s.Source}
	}
	if s.Producer != nil {
		opts.Producer = &wrapperspb.StringValue{Value: *s.Producer}
	}
	if !s.Before.IsZero() {
		opts.Before = &timestamppb.Timestamp{Seconds: s.Before.Unix()}
	}
	if !s.After.IsZero() {
		opts.After = &timestamppb.Timestamp{Seconds: s.After.Unix()}
	}
	return opts
}

// NewHandler creates a new Handler instance.
func NewHandler(chConn clickhouse.Conn, s3Client *s3.Client, buckets []string, eventService *eventrepo.Service,
	vehicleAddr common.Address, chainID uint64,
) *Handler {
	return &Handler{
		eventService: eventService,
		buckets:      buckets,
		vehicleAddr:  vehicleAddr,
		chainID:      chainID,
	}
}

// parseTokenAndParams parses tokenId path param and query params, validates access via
// the same logic as GraphQL (DID built from path), and returns opts and any error.
func (h *Handler) parseTokenAndParams(fCtx *fiber.Ctx) (*grpc.SearchOptions, *searchParams, error) {
	tokenID := fCtx.Params(TokenIDParam)
	uTokenID, err := strconv.ParseUint(tokenID, 0, 32)
	if err != nil {
		return nil, nil, fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse token ID: %v", err))
	}
	var params searchParams
	if err = fCtx.QueryParser(&params); err != nil {
		return nil, nil, fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse request query: %v", err))
	}
	subject := cloudevent.ERC721DID{ChainID: h.chainID, ContractAddress: h.vehicleAddr, TokenID: big.NewInt(int64(uTokenID))}
	did := subject.String()

	var claims *tokenclaims.Token
	if jwtToken, ok := fCtx.Locals(jwtmiddleware.TokenClaimsKey).(*jwt.Token); ok {
		claims, _ = jwtToken.Claims.(*tokenclaims.Token)
	}
	ctx := context.WithValue(fCtx.Context(), graph.ClaimsContextKey{}, claims)
	if err := graph.CheckVehicleRawDataByDID(ctx, did); err != nil {
		return nil, nil, fiber.NewError(fiber.StatusForbidden, err.Error())
	}
	return params.toSearchOptions(subject), &params, nil
}

// GetLatestIndexKey handles requests for the latest index key
// @Summary Get the latest index key based on search criteria
// @Description Retrieves the most recent index key that matches the provided search options
// @Tags objects
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Param tokenId path string true "Token ID"
// @Success 200 {object} cloudReturn "Returns the latest index key"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/latest-index-key/{tokenId} [get]
func (h *Handler) GetLatestIndexKey(fCtx *fiber.Ctx) error {
	opts, _, err := h.parseTokenAndParams(fCtx)
	if err != nil {
		return err
	}
	metadata, err := h.eventService.GetLatestIndex(fCtx.Context(), opts)
	if err != nil {
		return handleDBError(err)
	}
	return fCtx.JSON(metadata)
}

// GetIndexKeys handles requests for multiple index keys
// @Summary Get multiple index keys based on search criteria
// @Description Retrieves a list of index keys that match the provided search options
// @Tags objects
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Param tokenId path string true "Token ID"
// @Success 200 {object} []cloudReturn "Returns list of index keys"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/index-keys/{tokenId} [get]
func (h *Handler) GetIndexKeys(fCtx *fiber.Ctx) error {
	opts, params, err := h.parseTokenAndParams(fCtx)
	if err != nil {
		return err
	}
	metaList, err := h.eventService.ListIndexes(fCtx.Context(), params.Limit, opts)
	if err != nil {
		return handleDBError(err)
	}
	return fCtx.JSON(metaList)
}

// GetObjects handles requests for multiple objects
// @Summary Get multiple objects based on search criteria
// @Description Retrieves the content of multiple objects that match the provided search options
// @Tags objects
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Param tokenId path string true "Token ID"
// @Success 200 {object} []cloudevent.RawEvent "Returns latest object data"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/objects/{tokenId} [get]
func (h *Handler) GetObjects(fCtx *fiber.Ctx) error {
	opts, params, err := h.parseTokenAndParams(fCtx)
	if err != nil {
		return err
	}
	metaList, err := h.eventService.ListIndexes(fCtx.Context(), params.Limit, opts)
	if err != nil {
		return handleDBError(err)
	}
	data, err := fetch.ListCloudEventsFromIndexes(fCtx.Context(), h.eventService, metaList, h.buckets)
	if err != nil {
		return handleDBError(err)
	}
	return fCtx.JSON(data)
}

// GetLatestObject handles requests for the latest object
// @Summary Get the latest object based on search criteria
// @Description Retrieves the content of the most recent object that matches the provided search options
// @Tags objects
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Param tokenId path string true "Token ID"
// @Success 200 {object} cloudevent.RawEvent "Returns latest object data"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/latest-object/{tokenId} [get]
func (h *Handler) GetLatestObject(fCtx *fiber.Ctx) error {
	opts, _, err := h.parseTokenAndParams(fCtx)
	if err != nil {
		return err
	}
	metadata, err := h.eventService.GetLatestIndex(fCtx.Context(), opts)
	if err != nil {
		return handleDBError(err)
	}
	data, err := fetch.GetCloudEventFromIndex(fCtx.Context(), h.eventService, metadata, h.buckets)
	if err != nil {
		return handleDBError(err)
	}
	return fCtx.JSON(data)
}

// handleDBError logs the error and returns a generic error message.
func handleDBError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return richerrors.Error{
			Code:        http.StatusRequestTimeout,
			ExternalMsg: errTimeout,
			Err:         err,
		}
	}
	return richerrors.Error{
		Code:        http.StatusInternalServerError,
		ExternalMsg: "Failed to query db",
		Err:         err,
	}
}
