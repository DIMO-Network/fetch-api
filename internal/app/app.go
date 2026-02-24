package app

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/DIMO-Network/fetch-api/internal/auth"
	"github.com/DIMO-Network/fetch-api/internal/config"
	"github.com/DIMO-Network/fetch-api/internal/fetch/rpc"
	"github.com/DIMO-Network/fetch-api/internal/graph"
	"github.com/DIMO-Network/fetch-api/internal/limits"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	fetchgrpc "github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/DIMO-Network/server-garage/pkg/gql/errorhandler"
	gqlmetrics "github.com/DIMO-Network/server-garage/pkg/gql/metrics"
	"github.com/DIMO-Network/shared/pkg/middleware/metrics"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

// AppName is the name of the application.
var AppName = "fetch-api"

// App is the main application (GraphQL over net/http). gRPC is created separately via CreateGRPCServer.
type App struct {
	Handler http.Handler
	cleanup  func()
}

// New creates a new application with GraphQL handler and middleware.
func New(settings config.Settings) (*App, error) {
	chainID, err := strconv.ParseUint(settings.ChainID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chain ID: %w", err)
	}

	chConn, err := chClientFromSettings(&settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse connection: %w", err)
	}
	s3Client := s3ClientFromSettings(&settings)
	buckets := []string{settings.CloudEventBucket, settings.EphemeralBucket}
	eventService := eventrepo.New(chConn, s3Client, settings.ParquetBucket)

	gqlSrv := newGraphQLHandler(&settings, eventService, buckets, chainID)

	jwtMiddleware, err := auth.NewJWTMiddleware(settings.TokenExchangeIssuer, settings.TokenExchangeJWTKeySetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT middleware: %w", err)
	}

	maxReqDur := settings.MaxRequestDuration
	if maxReqDur == "" {
		maxReqDur = "60s"
	}
	limiter, err := limits.New(maxReqDur)
	if err != nil {
		return nil, fmt.Errorf("couldn't create request time limit middleware: %w", err)
	}

	serverHandler := PanicRecoveryMiddleware(
		LoggerMiddleware(
			limiter.AddRequestTimeout(
				jwtMiddleware.CheckJWT(
					authLoggerMiddleware(
						auth.AddClaimHandler(gqlSrv),
					),
				),
			),
		),
	)

	return &App{
		Handler: serverHandler,
		cleanup: func() {},
	}, nil
}

// Cleanup runs any cleanup logic (e.g. closing connections).
func (a *App) Cleanup() {
	if a.cleanup != nil {
		a.cleanup()
	}
}

// newGraphQLHandler creates a configured gqlgen handler.Server.
func newGraphQLHandler(settings *config.Settings, eventService *eventrepo.Service, buckets []string, chainID uint64) *handler.Server {
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

// CreateGRPCServer creates a new gRPC server with the given logger and settings.
func CreateGRPCServer(logger *zerolog.Logger, settings *config.Settings) (*grpc.Server, error) {
	chConn, err := chClientFromSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse connection: %w", err)
	}

	s3Client := s3ClientFromSettings(settings)
	eventService := eventrepo.New(chConn, s3Client, settings.ParquetBucket)

	rpcServer := rpc.NewServer([]string{settings.CloudEventBucket, settings.EphemeralBucket}, eventService)

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
