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

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	if err := s.DB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	if s.Cache != nil {
		if err := s.Cache.Close(); err != nil {
			s.Logger.Warn().Err(err).Msg("failed to close Redis cache")
		}
	}

	if s.Job != nil {
		s.Job.Stop()
	}

	return nil
}
