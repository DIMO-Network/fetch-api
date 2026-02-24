package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/DIMO-Network/fetch-api/internal/config"
	"github.com/DIMO-Network/fetch-api/internal/fetch/httphandler"
	"github.com/DIMO-Network/fetch-api/internal/fetch/rpc"
	"github.com/DIMO-Network/fetch-api/internal/graph"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	fetchgrpc "github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/server-garage/pkg/fibercommon"
	"github.com/DIMO-Network/server-garage/pkg/fibercommon/jwtmiddleware"
	"github.com/DIMO-Network/server-garage/pkg/gql/errorhandler"
	gqlmetrics "github.com/DIMO-Network/server-garage/pkg/gql/metrics"
	"github.com/DIMO-Network/shared/pkg/middleware/metrics"
	"github.com/DIMO-Network/token-exchange-api/pkg/tokenclaims"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/redirect"
	"github.com/gofiber/swagger"
	"github.com/golang-jwt/jwt/v5"
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

	jwtAuth := jwtmiddleware.NewJWTMiddleware(settings.TokenExchangeJWTKeySetURL)

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

	chConn, err := chClientFromSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse connection: %w", err)
	}
	s3Client := s3ClientFromSettings(settings)
	buckets := []string{settings.CloudEventBucket, settings.EphemeralBucket}

	// API v1 routes (register first, same order as main)
	v1 := app.Group("/v1")
	vehicleGroup := v1.Group("/vehicle")

	vehHandler := httphandler.NewHandler(chConn, s3Client, buckets, settings.VehicleNFTAddress, chainId)

	vehicleMiddleware := jwtmiddleware.AllOfPermissions(settings.VehicleNFTAddress, httphandler.TokenIDParam, []string{tokenclaims.PermissionGetRawData})

	vehicleGroup.Post("/latest-index-key/:"+httphandler.TokenIDParam, jwtAuth, vehicleMiddleware, vehHandler.GetLatestIndexKey)
	vehicleGroup.Post("/index-keys/:"+httphandler.TokenIDParam, jwtAuth, vehicleMiddleware, vehHandler.GetIndexKeys)
	vehicleGroup.Post("/objects/:"+httphandler.TokenIDParam, jwtAuth, vehicleMiddleware, vehHandler.GetObjects)
	vehicleGroup.Post("/latest-object/:"+httphandler.TokenIDParam, jwtAuth, vehicleMiddleware, vehHandler.GetLatestObject)

	// GraphQL endpoint (added after vehicle routes)
	gqlSrv := newGraphQLHandler(settings, chConn, s3Client, buckets, chainId)
	app.Post("/query", jwtAuth, graphQLHandler(gqlSrv))
	app.Get("/query", jwtAuth, graphQLHandler(gqlSrv))

	return app, nil
}

// CreateGRPCServer creates a new gRPC server with the given logger and settings.
func CreateGRPCServer(logger *zerolog.Logger, settings *config.Settings) (*grpc.Server, error) {
	chConn, err := chClientFromSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse connection: %w", err)
	}

	s3Client := s3ClientFromSettings(settings)

	rpcServer := rpc.NewServer(chConn, s3Client, []string{settings.CloudEventBucket, settings.EphemeralBucket})

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

// newGraphQLHandler creates a configured gqlgen handler.Server.
func newGraphQLHandler(settings *config.Settings, chConn clickhouse.Conn, s3Client *s3.Client, buckets []string, chainID uint64) *handler.Server {
	eventService := eventrepo.New(chConn, s3Client)

	resolver := &graph.Resolver{
		EventService: eventService,
		Buckets:      buckets,
		VehicleAddr:  settings.VehicleNFTAddress,
		ChainID:      chainID,
	}

	cfg := graph.Config{Resolvers: resolver}
	srv := handler.New(graph.NewExecutableSchema(cfg))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})
	srv.Use(extension.FixedComplexityLimit(100))
	srv.Use(gqlmetrics.Tracer{})
	srv.SetErrorPresenter(errorhandler.ErrorPresenter)
	return srv
}

// graphQLHandler bridges Fiber to the gqlgen http.Handler and injects token claims into the request context.
func graphQLHandler(gqlHandler *handler.Server) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var claims *tokenclaims.Token
		if jwtToken, ok := c.Locals(jwtmiddleware.TokenClaimsKey).(*jwt.Token); ok {
			claims, _ = jwtToken.Claims.(*tokenclaims.Token)
		}
		ctx := context.WithValue(c.Context(), graph.ClaimsContextKey{}, claims)
		body := c.Body()
		var req *http.Request
		var err error
		if len(body) > 0 {
			req, err = http.NewRequestWithContext(ctx, c.Method(), c.OriginalURL(), bytes.NewReader(body))
		} else {
			req, err = http.NewRequestWithContext(ctx, c.Method(), c.OriginalURL(), nil)
		}
		if err != nil {
			return err
		}
		for k, v := range c.GetReqHeaders() {
			if len(v) > 0 {
				req.Header.Set(k, v[0])
			}
		}
		w := &fiberResponseWriter{c: c}
		gqlHandler.ServeHTTP(w, req)
		return nil
	}
}

type fiberResponseWriter struct {
	c         *fiber.Ctx
	header    http.Header
	status    int
	committed bool
}

func (w *fiberResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *fiberResponseWriter) commit() {
	if w.committed {
		return
	}
	w.committed = true
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.c.Status(w.status)
	for k, v := range w.header {
		for _, vv := range v {
			w.c.Set(k, vv)
		}
	}
}

func (w *fiberResponseWriter) Write(b []byte) (int, error) {
	w.commit()
	return w.c.Write(b)
}

func (w *fiberResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.commit()
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
