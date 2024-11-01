// Packagge httphandler provides the HTTP handler for the fetch service.
package httphandler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DIMO-Network/model-garage/pkg/cloudevent"
	"github.com/DIMO-Network/nameindexer"
	"github.com/DIMO-Network/nameindexer/pkg/clickhouse/indexrepo"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

var (
	errInternal = errors.New("internal error")
	errTimeout  = errors.New("request exceeded or is estimated to exceed the maximum execution time")
)

// Handler is the HTTP handler for the fetch service.
type Handler struct {
	indexService     *indexrepo.Service
	cloudEventBucket string
	ephemeralBucket  string
	vehicleAddr      common.Address
	chainID          uint64
	logger           *zerolog.Logger
}

type indexKeysResponse struct {
	IndexKeys []string `json:"indexKeys"`
}
type indexKeyResponse struct {
	IndexKey string `json:"indexKey"`
}

type searchParams struct {
	Type     *string   `query:"type"`
	Source   *string   `query:"source"`
	Producer *string   `query:"producer"`
	Before   time.Time `query:"before"`
	After    time.Time `query:"after"`
	Limit    int       `query:"limit"`
}

func (s *searchParams) toSearchOptions(subject cloudevent.NFTDID) indexrepo.SearchOptions {
	var primaryFiller *string
	if s.Type != nil {
		filler := nameindexer.CloudTypeToFiller(*s.Type)
		primaryFiller = &filler
	}
	encodedSubject := nameindexer.EncodeNFTDID(subject)
	return indexrepo.SearchOptions{
		Subject:       &encodedSubject,
		PrimaryFiller: primaryFiller,
		Source:        s.Source,
		Producer:      s.Producer,
		Before:        s.Before,
		After:         s.After,
	}
}

// NewHandler creates a new Handler instance.
func NewHandler(logger *zerolog.Logger, chConn clickhouse.Conn, s3Client *s3.Client,
	cloudEventBucket, ephemeralBucket string,
	vehicleAddr common.Address, chainID uint64,
) *Handler {
	indexService := indexrepo.New(chConn, s3Client)
	return &Handler{
		indexService:     indexService,
		cloudEventBucket: cloudEventBucket,
		ephemeralBucket:  ephemeralBucket,
		vehicleAddr:      vehicleAddr,
		chainID:          chainID,
		logger:           logger,
	}
}

// GetLatestIndexKey handles requests for the latest index key
// @Summary Get the latest index key based on search criteria
// @Description Retrieves the most recent index key that matches the provided search options
// @Tags objects
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Param tokenId path string true "Token ID"
// @Success 200 {object} indexKeyResponse "Returns the latest index key"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/latest-index-key/{tokenId} [get]
func (h *Handler) GetLatestIndexKey(fCtx *fiber.Ctx) error {
	tokenID := fCtx.Params("tokenId")
	uTokenID, err := strconv.ParseUint(tokenID, 0, 32)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse token ID: %v", err))
	}

	var params searchParams
	err = fCtx.QueryParser(&params)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse request query: %v", err))
	}

	opts := params.toSearchOptions(cloudevent.NFTDID{ChainID: h.chainID, ContractAddress: h.vehicleAddr, TokenID: uint32(uTokenID)})

	indexKey, err := h.indexService.GetLatestIndexKey(fCtx.Context(), opts)
	if err != nil {
		return handleDBError(err, h.logger)
	}

	return fCtx.JSON(indexKeyResponse{IndexKey: indexKey})
}

// GetIndexKeys handles requests for multiple index keys
// @Summary Get multiple index keys based on search criteria
// @Description Retrieves a list of index keys that match the provided search options
// @Tags objects
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Param tokenId path string true "Token ID"
// @Success 200 {object} indexKeysResponse "Returns list of index keys"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/index-keys/{tokenId} [get]
func (h *Handler) GetIndexKeys(fCtx *fiber.Ctx) error {
	tokenID := fCtx.Params("tokenId")
	uTokenID, err := strconv.ParseUint(tokenID, 0, 32)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse token ID: %v", err))
	}

	var params searchParams
	err = fCtx.QueryParser(&params)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse request query: %v", err))
	}

	opts := params.toSearchOptions(cloudevent.NFTDID{ChainID: h.chainID, ContractAddress: h.vehicleAddr, TokenID: uint32(uTokenID)})

	indexKeys, err := h.indexService.GetIndexKeys(fCtx.Context(), params.Limit, opts)
	if err != nil {
		return handleDBError(err, h.logger)
	}

	return fCtx.JSON(indexKeysResponse{IndexKeys: indexKeys})
}

// GetObjects handles requests for multiple objects
// @Summary Get multiple objects based on search criteria
// @Description Retrieves the content of multiple objects that match the provided search options
// @Tags objects
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Param tokenId path string true "Token ID"
// @Success 200 {object} []indexrepo.DataObject "Returns latest object data"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/objects/{tokenId} [get]
func (h *Handler) GetObjects(fCtx *fiber.Ctx) error {
	tokenID := fCtx.Params("tokenId")
	uTokenID, err := strconv.ParseUint(tokenID, 0, 32)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse token ID: %v", err))
	}

	var params searchParams
	err = fCtx.QueryParser(&params)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse request query: %v", err))
	}

	opts := params.toSearchOptions(cloudevent.NFTDID{ChainID: h.chainID, ContractAddress: h.vehicleAddr, TokenID: uint32(uTokenID)})

	data, err := h.indexService.GetObject(fCtx.Context(), h.cloudEventBucket, params.Limit, opts)
	if err != nil {
		return handleDBError(err, h.logger)
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
// @Success 200 {object} indexrepo.DataObject "Returns latest object data"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/latest-object/{tokenId} [get]
func (h *Handler) GetLatestObject(fCtx *fiber.Ctx) error {
	tokenID := fCtx.Params("tokenId")
	uTokenID, err := strconv.ParseUint(tokenID, 0, 32)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse token ID: %v", err))
	}

	var params searchParams
	err = fCtx.QueryParser(&params)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("failed to parse request query: %v", err))
	}

	opts := params.toSearchOptions(cloudevent.NFTDID{ChainID: h.chainID, ContractAddress: h.vehicleAddr, TokenID: uint32(uTokenID)})

	data, err := h.indexService.GetLatestObject(fCtx.Context(), h.cloudEventBucket, opts)
	if err != nil {
		return handleDBError(err, h.logger)
	}

	return fCtx.JSON(data)
}

// handleDBError logs the error and returns a generic error message.
func handleDBError(err error, log *zerolog.Logger) error {
	if errors.Is(err, context.DeadlineExceeded) {
		log.Error().Err(err).Msg("failed to query db")
		return errTimeout
	}
	log.Error().Err(err).Msg("failed to query db")
	return errInternal
}
