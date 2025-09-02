package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"strconv"

	_ "github.com/DIMO-Network/fetch-api/docs"
	"github.com/DIMO-Network/fetch-api/internal/app"
	"github.com/DIMO-Network/fetch-api/internal/config"
	"github.com/DIMO-Network/server-garage/pkg/env"
	"github.com/DIMO-Network/server-garage/pkg/logging"
	"github.com/DIMO-Network/server-garage/pkg/monserver"
	"github.com/DIMO-Network/server-garage/pkg/runner"
	"golang.org/x/sync/errgroup"
)

// @title                       DIMO Fetch API
// @version                     1.0
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
func main() {
	settingsFile := flag.String("env", ".env", "env file")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	logger := logging.GetAndSetDefaultLogger("fetch-api")

	settings, err := env.LoadSettings[config.Settings](*settingsFile)
	if err != nil {
		logger.Fatal().Err(err).Msg("Couldn't load settings.")
	}

	webServer, err := app.CreateWebServer(&settings)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create web server.")
	}
	rpcServer, err := app.CreateGRPCServer(&logger, &settings)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create RPC server.")
	}

	group, gCtx := errgroup.WithContext(ctx)

	monApp := monserver.NewMonitoringServer(&logger, settings.EnablePprof)
	logger.Info().Str("port", strconv.Itoa(settings.MonPort)).Msgf("Starting monitoring server")
	runner.RunHandler(gCtx, group, monApp, ":"+strconv.Itoa(settings.MonPort))

	logger.Info().Str("port", strconv.Itoa(settings.Port)).Msgf("Starting web server")
	runner.RunFiber(gCtx, group, webServer, ":"+strconv.Itoa(settings.Port))

	logger.Info().Str("port", strconv.Itoa(settings.GRPCPort)).Msgf("Starting gRPC server")
	runner.RunGRPC(gCtx, group, rpcServer, ":"+strconv.Itoa(settings.GRPCPort))

	err = group.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal().Err(err).Msg("Server shut down due to an error.")
	}
	logger.Info().Msg("Server shut down.")
}
