package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"

	"backend/internal/config"
	"backend/internal/database"
	"backend/internal/handler"
	"backend/internal/logger"
	"backend/internal/otel"
	"backend/internal/repository"
	"backend/internal/router"
	"backend/internal/server"
	"backend/internal/service"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	log := logger.NewLogger(cfg.Observability)

	// Initialize OpenTelemetry (no-op when disabled)
	otelShutdown, err := otel.Init(context.Background(), cfg.Observability, cfg.Observability.ServiceName)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize OpenTelemetry")
	}

	// Auto-migrate if configured (runs in any environment when enabled)
	if cfg.Database.AutoMigrate {
		log.Info().Msg("auto-migrate enabled, running database migrations")
		if err := database.Migrate(context.Background(), &log, cfg); err != nil {
			log.Fatal().Err(err).Msg("auto-migration failed")
		}
	} else if cfg.Primary.Env != "local" {
		if err := database.Migrate(context.Background(), &log, cfg); err != nil {
			log.Fatal().Err(err).Msg("failed to migrate database")
		}
	}

	// Initialize server
	srv, err := server.New(cfg, &log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize server")
	}

	// Initialize repositories, services, and handlers
	repos := repository.NewRepositories(srv)
	services, serviceErr := service.NewServices(srv, repos)
	if serviceErr != nil {
		log.Fatal().Err(serviceErr).Msg("could not create services")
	}
	handlers := handler.NewHandlers(srv, services)

	// Initialize router
	r := router.NewRouter(srv, handlers)

	// Setup HTTP server
	srv.SetupHTTPServer(r)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	// Start server
	go func() {
		if err = srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	log.Info().Msg("interrupt received, starting graceful shutdown")

	// Use configured shutdown timeout instead of hardcoded value
	shutdownTimeout := srv.GetShutdownTimeout()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)

	// Shutdown OpenTelemetry first (flush pending traces/metrics)
	if err := otelShutdown(shutdownCtx); err != nil {
		log.Warn().Err(err).Msg("failed to shutdown OpenTelemetry")
	}

	// Shutdown server (ordered: HTTP drain → DB → cache → jobs)
	if err = srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}

	stop()
	cancel()
	log.Info().Msg("server exited properly")
}
