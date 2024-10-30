package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"

	_ "github.com/DIMO-Network/fetch-api/docs"
	"github.com/DIMO-Network/fetch-api/internal/app"
	"github.com/DIMO-Network/fetch-api/internal/config"
	"github.com/DIMO-Network/shared"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

// @title                       DIMO Fetch API
// @version                     1.0
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("app", "fetch-api").Logger()
	// create a flag for the settings file
	settingsFile := flag.String("settings", "settings.yaml", "settings file")
	flag.Parse()
	settings, err := shared.LoadConfig[config.Settings](*settingsFile)
	if err != nil {
		logger.Fatal().Err(err).Msg("Couldn't load settings.")
	}
	webServer, err := app.CreateWebServer(&logger, &settings)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create web server.")
	}
	rpcServer, err := app.CreateGRPCServer(&logger, &settings)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create RPC server.")
	}

	monApp := CreateMonitoringServer(strconv.Itoa(settings.MonPort), &logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	group, gCtx := errgroup.WithContext(ctx)

	logger.Info().Str("port", strconv.Itoa(settings.GRPCPort)).Msgf("Starting gRPC server")
	runGRPC(gCtx, rpcServer, ":"+strconv.Itoa(settings.GRPCPort), group)
	logger.Info().Str("port", strconv.Itoa(settings.MonPort)).Msgf("Starting monitoring server")
	runFiber(gCtx, monApp, ":"+strconv.Itoa(settings.MonPort), group)
	logger.Info().Str("port", strconv.Itoa(settings.Port)).Msgf("Starting web server")
	runFiber(gCtx, webServer, ":"+strconv.Itoa(settings.Port), group)

	err = group.Wait()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to run servers.")
	}
}

func runFiber(ctx context.Context, fiberApp *fiber.App, addr string, group *errgroup.Group) {
	group.Go(func() error {
		if err := fiberApp.Listen(addr); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}
		return nil
	})
	group.Go(func() error {
		<-ctx.Done()
		if err := fiberApp.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
		return nil
	})
}

func runGRPC(ctx context.Context, grpcServer *grpc.Server, addr string, group *errgroup.Group) {
	group.Go(func() error {
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to listen on gRPC port %s: %w", addr, err)
		}
		if err := grpcServer.Serve(lis); err != nil {
			return fmt.Errorf("gRPC server failed to serve: %w", err)
		}
		return nil
	})
	group.Go(func() error {
		<-ctx.Done()
		grpcServer.GracefulStop()
		return nil
	})
}

func CreateMonitoringServer(port string, logger *zerolog.Logger) *fiber.App {
	monApp := fiber.New(fiber.Config{DisableStartupMessage: true})
	monApp.Get("/", func(c *fiber.Ctx) error { return nil })
	monApp.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	return monApp
}
