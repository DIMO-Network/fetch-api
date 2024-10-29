package httphandler

import (
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DIMO-Network/nameindexer/pkg/clickhouse/indexrepo"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v2"
)

// Handler contains the dependencies for the HTTP handlers.
type Handler struct {
	indexService     *indexrepo.Service
	cloudEventBucket string
	ephemeralBucket  string
}

// NewHandler creates a new Handler instance.
func NewHandler(chConn clickhouse.Conn, s3Client *s3.Client, cloudEventBucket, ephemeralBucket string) *Handler {
	indexService := indexrepo.New(chConn, s3Client)
	return &Handler{
		indexService:     indexService,
		cloudEventBucket: cloudEventBucket,
		ephemeralBucket:  ephemeralBucket,
	}
}

// GetLatestFileName handles requests for the latest filename
// @Summary Get the latest filename based on search criteria
// @Description Retrieves the most recent filename that matches the provided search options
// @Tags files
// @Accept json
// @Produce json
// @Param request body indexrepo.SearchOptions true "Search criteria for finding the latest file"
// @Success 200 {object} map[string]string "Returns the latest filename"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Server error"
// @Router /latest-filename [post]
func (h *Handler) GetLatestFileName(c *fiber.Ctx) error {
	var req indexrepo.SearchOptions
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to parse request body: %v", err),
		})
	}
	filename, err := h.indexService.GetLatestFileName(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get latest file name: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"filename": filename,
	})
}

type SearchOptionsWithLimit struct {
	indexrepo.SearchOptions
	Limit int `json:"limit" example:"10"`
}

// GetFileNames handles requests for multiple filenames
// @Summary Get multiple filenames based on search criteria
// @Description Retrieves a list of filenames that match the provided search options
// @Tags files
// @Accept json
// @Produce json
// @Param request body SearchOptionsWithLimit true "Search criteria and limit for finding files"
// @Success 200 {object} map[string][]string "Returns list of filenames"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Server error"
// @Router /filenames [post]
func (h *Handler) GetFileNames(c *fiber.Ctx) error {
	var req SearchOptionsWithLimit
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to parse request body: %v", err),
		})
	}
	filenames, err := h.indexService.GetFileNames(c.Context(), req.Limit, req.SearchOptions)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get file names: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"filenames": filenames,
	})
}

// GetFiles handles requests for multiple files
// @Summary Get multiple files based on search criteria
// @Description Retrieves the content of multiple files that match the provided search options
// @Tags files
// @Accept json
// @Produce json
// @Param request body SearchOptionsWithLimit true "Search criteria and limit for finding files"
// @Success 200 {object} map[string][]byte "Returns file data"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Server error"
// @Router /files [post]
func (h *Handler) GetFiles(c *fiber.Ctx) error {
	var req SearchOptionsWithLimit
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to parse request body: %v", err),
		})
	}
	data, err := h.indexService.GetData(c.Context(), h.cloudEventBucket, req.Limit, req.SearchOptions)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get files: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"data": data,
	})
}

// GetLatestFile handles requests for the latest file
// @Summary Get the latest file based on search criteria
// @Description Retrieves the content of the most recent file that matches the provided search options
// @Tags files
// @Accept json
// @Produce json
// @Param request body indexrepo.SearchOptions true "Search criteria for finding the latest file"
// @Success 200 {object} map[string][]byte "Returns latest file data"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Server error"
// @Router /latest-file [post]
func (h *Handler) GetLatestFile(c *fiber.Ctx) error {
	var req indexrepo.SearchOptions
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to parse request body: %v", err),
		})
	}
	data, err := h.indexService.GetLatestData(c.Context(), h.cloudEventBucket, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to get latest file: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"data": data,
	})
}
