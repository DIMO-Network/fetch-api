package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/DIMO-Network/fetch-api/internal/app"
	"github.com/DIMO-Network/fetch-api/internal/config"
	"github.com/DIMO-Network/server-garage/pkg/env"
	"github.com/DIMO-Network/server-garage/pkg/logging"
	"github.com/DIMO-Network/server-garage/pkg/monserver"
	"github.com/DIMO-Network/server-garage/pkg/runner"
	"golang.org/x/sync/errgroup"
)

func main() {
	settingsFile := flag.String("env", ".env", "env file")
	flag.Parse()

	mainCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := logging.GetAndSetDefaultLogger("fetch-api")

	settings, err := env.LoadSettings[config.Settings](*settingsFile)
	if err != nil {
		logger.Fatal().Err(err).Msg("Couldn't load settings.")
	}

	application, err := app.New(&settings)
	if err != nil {
		logger.Fatal().Err(err).Msg("Couldn't create application.")
	}
	defer application.Cleanup()

	rpcServer, err := app.CreateGRPCServer(&logger, &settings)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create gRPC server.")
	}

	group, gCtx := errgroup.WithContext(mainCtx)

	// Monitoring server (health, pprof) - same pattern as telemetry-api
	monSrv := monserver.NewMonitoringServer(&logger, settings.EnablePprof)
	runner.RunHandler(gCtx, group, monSrv, ":"+strconv.Itoa(settings.MonPort))

	mux := http.NewServeMux()
	mux.Handle("/", app.PanicRecoveryMiddleware(app.LoggerMiddleware(playground.Handler("GraphQL playground", "/query"))))
	mux.Handle("/query", application.Handler)

	logger.Info().Str("port", strconv.Itoa(settings.Port)).Msg("Starting web server")
	runner.RunHandler(gCtx, group, mux, ":"+strconv.Itoa(settings.Port))

	logger.Info().Str("port", strconv.Itoa(settings.GRPCPort)).Msg("Starting gRPC server")
	runner.RunGRPC(gCtx, group, rpcServer, ":"+strconv.Itoa(settings.GRPCPort))

	err = group.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal().Err(err).Msg("Server shut down due to an error.")
	}
	logger.Info().Msg("Server shut down.")
}
