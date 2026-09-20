package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"backend/internal/cache"
	"backend/internal/config"
	"backend/internal/database"
	"backend/internal/lib/job"

	"github.com/rs/zerolog"
)

type Server struct {
	Config     *config.Config
	Logger     *zerolog.Logger
	DB         *database.Database
	Cache      *cache.Cache
	httpServer *http.Server
	Job        *job.JobService
}

func New(cfg *config.Config, logger *zerolog.Logger) (*Server, error) {
	db, err := database.New(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis cache layer (fail-open: logs warning if Redis is unavailable)
	redisCache := cache.New(cfg, logger)

	// job service
	jobService := job.NewJobService(logger, cfg)
	jobService.InitHandlers(cfg, logger)

	// Start job server in the background. asynq Server.Start is blocking, so it
	// must not run on the startup path or the HTTP server would never start.
	// (Source started it synchronously, which hangs `task run`.)
	go func() {
		if err := jobService.Start(); err != nil {
			logger.Error().Err(err).Msg("job server failed to start")
		}
	}()

	server := &Server{
		Config: cfg,
		Logger: logger,
		DB:     db,
		Cache:  redisCache,
		Job:    jobService,
	}

	return server, nil
}

func (s *Server) SetupHTTPServer(handler http.Handler) {
	s.httpServer = &http.Server{
		Addr:         ":" + s.Config.Server.Port,
		Handler:      handler,
		ReadTimeout:  time.Duration(s.Config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.Config.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(s.Config.Server.IdleTimeout) * time.Second,
	}
}

func (s *Server) Start() error {
	if s.httpServer == nil {
		return errors.New("HTTP server not initialized")
	}

	s.Logger.Info().
		Str("port", s.Config.Server.Port).
		Str("env", s.Config.Primary.Env).
		Msg("starting server")

	return s.httpServer.ListenAndServe()
}

// Shutdown performs an ordered graceful shutdown:
// 1. Stop accepting new HTTP connections and drain in-flight requests
// 2. Close database connection pool
// 3. Close Redis cache
// 4. Stop background jobs
func (s *Server) Shutdown(ctx context.Context) error {
	s.Logger.Info().Msg("shutdown initiated")

	drainTimeout := s.getDrainTimeout()
	drainCtx, drainCancel := context.WithTimeout(ctx, drainTimeout)
	defer drainCancel()

	// Step 1: Drain HTTP connections
	s.Logger.Info().Dur("timeout", drainTimeout).Msg("draining HTTP connections")
	if err := s.httpServer.Shutdown(drainCtx); err != nil {
		s.Logger.Error().Err(err).Msg("failed to drain HTTP connections")
		return fmt.Errorf("failed to drain HTTP server: %w", err)
	}
	s.Logger.Info().Msg("HTTP connections drained")

	// Step 2: Close database
	s.Logger.Info().Msg("closing database connections")
	if err := s.DB.Close(); err != nil {
		s.Logger.Error().Err(err).Msg("failed to close database")
		return fmt.Errorf("failed to close database: %w", err)
	}
	s.Logger.Info().Msg("database connections closed")

	// Step 3: Close Redis cache
	if s.Cache != nil {
		s.Logger.Info().Msg("closing Redis cache")
		if err := s.Cache.Close(); err != nil {
			s.Logger.Warn().Err(err).Msg("failed to close Redis cache")
		} else {
			s.Logger.Info().Msg("Redis cache closed")
		}
	}

	// Step 4: Stop background jobs
	if s.Job != nil {
		s.Logger.Info().Msg("stopping background jobs")
		s.Job.Stop()
		s.Logger.Info().Msg("background jobs stopped")
	}

	s.Logger.Info().Msg("shutdown complete")
	return nil
}

// getDrainTimeout returns the configured drain timeout or a sensible default.
func (s *Server) getDrainTimeout() time.Duration {
	if s.Config.Server.DrainTimeout > 0 {
		return time.Duration(s.Config.Server.DrainTimeout) * time.Second
	}
	return 15 * time.Second
}

// GetShutdownTimeout returns the configured shutdown timeout or a sensible default.
func (s *Server) GetShutdownTimeout() time.Duration {
	if s.Config.Server.ShutdownTimeout > 0 {
		return time.Duration(s.Config.Server.ShutdownTimeout) * time.Second
	}
	return 30 * time.Second
}
