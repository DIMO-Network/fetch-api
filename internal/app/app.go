package app

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/DIMO-Network/fetch-api/internal/config"
	"github.com/DIMO-Network/fetch-api/internal/fetch/httphandler"
	"github.com/DIMO-Network/fetch-api/internal/fetch/rpc"
	"github.com/DIMO-Network/fetch-api/pkg/auth"
	fetchgrpc "github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/shared/middleware/metrics"
	"github.com/DIMO-Network/shared/middleware/privilegetoken"
	"github.com/DIMO-Network/shared/privileges"
	"github.com/ethereum/go-ethereum/common"
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/redirect"
	"github.com/gofiber/swagger"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

// CreateWebServer creates a new web server with the given logger and settings.
func CreateWebServer(logger *zerolog.Logger, settings *config.Settings) (*fiber.App, error) {
	if !common.IsHexAddress(settings.VehicleNFTAddress) {
		return nil, errors.New("invalid vehicle NFT address")
	}
	vehicleNFTAddress := common.HexToAddress(settings.VehicleNFTAddress)
	chainId, err := strconv.ParseUint(settings.ChainID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chain ID: %w", err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return ErrorHandler(c, err, logger)
		},
		DisableStartupMessage: true,
	})

	jwtAuth := jwtware.New(jwtware.Config{
		JWKSetURLs: []string{settings.TokenExchangeJWTKeySetURL},
		Claims:     &privilegetoken.Token{},
	})

	app.Use(recover.New(recover.Config{
		Next:              nil,
		EnableStackTrace:  true,
		StackTraceHandler: nil,
	}))
	app.Use(cors.New())
	app.Get("/", HealthCheck)
	app.Use(redirect.New(redirect.Config{
		Rules: map[string]string{
			"/v1/swagger":   "/swagger",
			"/v1/swagger/*": "/swagger/$1",
		},
		StatusCode: http.StatusMovedPermanently,
	}))
	app.Get("/swagger/*", swagger.HandlerDefault)

	// API v1 routes
	v1 := app.Group("/v1")
	vehicleGroup := v1.Group("/vehicle")

	vehiclePriv := auth.AllOf(vehicleNFTAddress, "tokenId", []privileges.Privilege{privileges.Privilege(7)})

	chConn, err := chClientFromSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse connection: %w", err)
	}

	s3Client := s3ClientFromSettings(settings)
	vehHandler := httphandler.NewHandler(logger, chConn, s3Client,
		settings.CloudEventBucket, settings.EphemeralBucket, vehicleNFTAddress, chainId)
	// File endpoints
	vehicleGroup.Post("/latest-index-key/:tokenId", vehiclePriv, jwtAuth, vehHandler.GetLatestIndexKey)
	vehicleGroup.Post("/index-keys/:tokenId", vehiclePriv, jwtAuth, vehHandler.GetIndexKeys)
	vehicleGroup.Post("/objects/:tokenId", jwtAuth, vehiclePriv, vehHandler.GetObjects)
	vehicleGroup.Post("/latest-object/:tokenId", jwtAuth, vehiclePriv, vehHandler.GetLatestObject)

	return app, nil
}

// CreateGRPCServer creates a new gRPC server with the given logger and settings.
func CreateGRPCServer(logger *zerolog.Logger, settings *config.Settings) (*grpc.Server, error) {
	chConn, err := chClientFromSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse connection: %w", err)
	}

	s3Client := s3ClientFromSettings(settings)

	rpcServer := rpc.NewServer(chConn, s3Client, settings.CloudEventBucket, settings.EphemeralBucket)

	grpcPanic := metrics.GRPCPanicker{Logger: logger}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			metrics.GRPCMetricsAndLogMiddleware(logger),
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_prometheus.UnaryServerInterceptor,
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanic.GRPCPanicRecoveryHandler)),
		)),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	)
	fetchgrpc.RegisterFetchServiceServer(server, rpcServer)

	return server, nil
}

// ErrorHandler custom handler to log recovered errors using our logger and return json instead of string
func ErrorHandler(ctx *fiber.Ctx, err error, logger *zerolog.Logger) error {
	code := fiber.StatusInternalServerError // Default 500 statuscode
	message := "Internal error."

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
	}

	// don't log not found errors
	if code != fiber.StatusNotFound {
		logger.Err(err).Int("httpStatusCode", code).
			Str("httpPath", strings.TrimPrefix(ctx.Path(), "/")).
			Str("httpMethod", ctx.Method()).
			Msg("caught an error from http request")
	}

	return ctx.Status(code).JSON(codeResp{Code: code, Message: message})
}

type codeResp struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// HealthCheck godoc
// @Summary Show the status of server.
// @Description get the status of server.
// @Tags root
// @Accept */*
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router / [get]
func HealthCheck(ctx *fiber.Ctx) error {
	res := map[string]any{
		"data": "Server is up and running",
	}

	return ctx.JSON(res)
}
