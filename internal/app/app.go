package app

import (
	"context"
	"fmt"
	"net/http"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/DIMO-Network/fetch-api/internal/auth"
	"github.com/DIMO-Network/fetch-api/internal/config"
	"github.com/DIMO-Network/fetch-api/internal/fetch/rpc"
	"github.com/DIMO-Network/fetch-api/internal/graph"
	"github.com/DIMO-Network/fetch-api/internal/identity"
	"github.com/DIMO-Network/fetch-api/internal/limits"
	"github.com/DIMO-Network/fetch-api/pkg/eventrepo"
	fetchgrpc "github.com/DIMO-Network/fetch-api/pkg/grpc"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/DIMO-Network/server-garage/pkg/gql/errorhandler"
	gqlmetrics "github.com/DIMO-Network/server-garage/pkg/gql/metrics"
	"github.com/DIMO-Network/server-garage/pkg/mcpserver"
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
	Handler    http.Handler
	MCPHandler http.Handler
	cleanup    func()
}

// New creates a new application with GraphQL handler and middleware.
func New(settings config.Settings) (*App, error) {
	chConn, err := chClientFromSettings(&settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse connection: %w", err)
	}
	s3Client := s3ClientFromSettings(&settings)
	buckets := []string{settings.CloudEventBucket, settings.EphemeralBucket, settings.ParquetBucket}
	eventService := eventrepo.New(chConn, s3Client, s3.NewPresignClient(s3Client), settings.ParquetBucket)

	var identityClient identity.Client
	if settings.IdentityAPIURL != "" {
		identityClient = identity.New(settings.IdentityAPIURL)
	}

	es := newExecutableSchema(eventService, buckets, identityClient)
	gqlSrv := newGraphQLHandler(es)

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

	authChain := func(inner http.Handler) http.Handler {
		return PanicRecoveryMiddleware(
			LoggerMiddleware(
				limiter.AddRequestTimeout(
					jwtMiddleware.CheckJWT(
						authLoggerMiddleware(
							auth.AddClaimHandler(inner),
						),
					),
				),
			),
		)
	}

	serverHandler := authChain(gqlSrv)

	mcpHandler, err := mcpserver.New(context.Background(), mcpserver.NewGQLGenExecutor(es), "DIMO Fetch", "0.1.0", "fetch",
		mcpserver.WithTools(graph.MCPTools),
		mcpserver.WithCondensedSchema(graph.CondensedSchema),
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't create MCP handler: %w", err)
	}

	return &App{
		Handler:    serverHandler,
		MCPHandler: authChain(mcpHandler),
		cleanup:    func() {},
	}, nil
}

// Cleanup runs any cleanup logic (e.g. closing connections).
func (a *App) Cleanup() {
	if a.cleanup != nil {
		a.cleanup()
	}
}

// newExecutableSchema builds the gqlgen ExecutableSchema shared by the GraphQL and MCP handlers.
func newExecutableSchema(eventService *eventrepo.Service, buckets []string, identityClient identity.Client) graphql.ExecutableSchema {
	resolver := &graph.Resolver{
		EventService:   eventService,
		Buckets:        buckets,
		IdentityClient: identityClient,
	}
	return graph.NewExecutableSchema(graph.Config{Resolvers: resolver})
}

// newGraphQLHandler creates a configured gqlgen handler.Server.
func newGraphQLHandler(es graphql.ExecutableSchema) *handler.Server {
	srv := handler.New(es)
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
	eventService := eventrepo.New(chConn, s3Client, s3.NewPresignClient(s3Client), settings.ParquetBucket)

	rpcServer := rpc.NewServer([]string{settings.CloudEventBucket, settings.EphemeralBucket, settings.ParquetBucket}, eventService)

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
