package app

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/DIMO-Network/fetch-api/internal/config"
	"github.com/DIMO-Network/fetch-api/internal/fetch/httphandler"
	"github.com/DIMO-Network/fetch-api/internal/fetch/rpc"
	"github.com/DIMO-Network/fetch-api/pkg/auth"
	fetchgrpc "github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/server-garage/pkg/fibercommon"
	"github.com/DIMO-Network/shared/pkg/middleware/metrics"
	"github.com/DIMO-Network/shared/pkg/middleware/privilegetoken"
	"github.com/DIMO-Network/shared/pkg/privileges"
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
func CreateWebServer(settings *config.Settings) (*fiber.App, error) {
	chainId, err := strconv.ParseUint(settings.ChainID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chain ID: %w", err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler:          fibercommon.ErrorHandler,
		DisableStartupMessage: true,
	})
	app.Use(fibercommon.ContextLoggerMiddleware)

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

	vehiclePriv := auth.AllOf(settings.VehicleNFTAddress, "tokenId", []privileges.Privilege{privileges.Privilege(7)})

	chConn, err := chClientFromSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse connection: %w", err)
	}

	s3Client := s3ClientFromSettings(settings)
	vehHandler := httphandler.NewHandler(chConn, s3Client,
		[]string{settings.CloudEventBucket, settings.EphemeralBucket, settings.VCBucket}, settings.VehicleNFTAddress, chainId)
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

	rpcServer := rpc.NewServer(chConn, s3Client, []string{settings.CloudEventBucket, settings.EphemeralBucket, settings.VCBucket})

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
