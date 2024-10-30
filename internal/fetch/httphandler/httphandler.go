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

// GetLatestFileName handles requests for the latest filename
// @Summary Get the latest filename based on search criteria
// @Description Retrieves the most recent filename that matches the provided search options
// @Tags files
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Success 200 {object} map[string]string "Returns the latest filename"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/{tokenId}/latest-filename [get]
func (h *Handler) GetLatestFileName(fCtx *fiber.Ctx) error {
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

	filename, err := h.indexService.GetLatestFileName(fCtx.Context(), opts)
	if err != nil {
		return handleDBError(err, h.logger)
	}

	return fCtx.JSON(fiber.Map{
		"filename": filename,
	})
}

// GetFileNames handles requests for multiple filenames
// @Summary Get multiple filenames based on search criteria
// @Description Retrieves a list of filenames that match the provided search options
// @Tags files
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Success 200 {object} map[string][]string "Returns list of filenames"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/{tokenId}/filenames [get]
func (h *Handler) GetFileNames(fCtx *fiber.Ctx) error {
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

	filenames, err := h.indexService.GetFileNames(fCtx.Context(), params.Limit, opts)
	if err != nil {
		return handleDBError(err, h.logger)
	}

	return fCtx.JSON(fiber.Map{
		"filenames": filenames,
	})
}

// GetFiles handles requests for multiple files
// @Summary Get multiple files based on search criteria
// @Description Retrieves the content of multiple files that match the provided search options
// @Tags files
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Success 200 {object} map[string][]byte "Returns file data"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/{tokenId}/files [get]
func (h *Handler) GetFiles(fCtx *fiber.Ctx) error {
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

	data, err := h.indexService.GetData(fCtx.Context(), h.cloudEventBucket, params.Limit, opts)
	if err != nil {
		return handleDBError(err, h.logger)
	}

	return fCtx.JSON(fiber.Map{
		"data": data,
	})
}

// GetLatestFile handles requests for the latest file
// @Summary Get the latest file based on search criteria
// @Description Retrieves the content of the most recent file that matches the provided search options
// @Tags files
// @Accept json
// @Produce json
// @Param params query searchParams false "Search parameters"
// @Success 200 {object} map[string][]byte "Returns latest file data"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /v1/vehicle/{tokenId}/latest-file [get]
func (h *Handler) GetLatestFile(fCtx *fiber.Ctx) error {
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

	data, err := h.indexService.GetLatestData(fCtx.Context(), h.cloudEventBucket, opts)
	if err != nil {
		return handleDBError(err, h.logger)
	}

	return fCtx.JSON(fiber.Map{
		"data": data,
	})
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
