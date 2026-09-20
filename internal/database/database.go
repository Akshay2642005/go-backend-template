package database

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	pgxzero "github.com/jackc/pgx-zerolog"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"

	"backend/internal/config"
	loggerConfig "backend/internal/logger"

	"github.com/rs/zerolog"
)

type Database struct {
	Pool *pgxpool.Pool
	log  *zerolog.Logger
}

const DatabasePingTimeout = 10

func New(cfg *config.Config, logger *zerolog.Logger) (*Database, error) {
	hostPort := net.JoinHostPort(cfg.Database.Host, strconv.Itoa(cfg.Database.Port))

	// URL-encode the password
	encodedPassword := url.QueryEscape(cfg.Database.Password)
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
		cfg.Database.User,
		encodedPassword,
		hostPort,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	pgxPoolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pgx pool config: %w", err)
	}

	if cfg.Primary.Env == "local" {
		globalLevel := logger.GetLevel()
		pgxLogger := loggerConfig.NewPgxLogger(globalLevel)
		localTracer := &tracelog.TraceLog{
			Logger:   pgxzero.NewLogger(pgxLogger),
			LogLevel: tracelog.LogLevel(loggerConfig.GetPgxTraceLogLevel(globalLevel)),
		}
		pgxPoolConfig.ConnConfig.Tracer = localTracer
	}

	// Connect with optional retry
	retries := cfg.Database.ConnectRetries
	if retries < 0 {
		retries = 0
	}
	retryDelay := time.Duration(cfg.Database.ConnectRetryDelay) * time.Second
	if retryDelay <= 0 {
		retryDelay = 1 * time.Second
	}

	var pool *pgxpool.Pool
	for attempt := 0; attempt <= retries; attempt++ {
		pool, err = pgxpool.NewWithConfig(context.Background(), pgxPoolConfig)
		if err != nil {
			if attempt < retries {
				logger.Warn().Err(err).Int("attempt", attempt+1).Int("max", retries).Dur("delay", retryDelay).Msg("database connection failed, retrying")
				time.Sleep(retryDelay)
				continue
			}
			return nil, fmt.Errorf("failed to create pgx pool after %d attempts: %w", attempt+1, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), DatabasePingTimeout*time.Second)
		err = pool.Ping(ctx)
		cancel()

		if err == nil {
			break
		}

		pool.Close()
		if attempt < retries {
			logger.Warn().Err(err).Int("attempt", attempt+1).Int("max", retries).Dur("delay", retryDelay).Msg("database ping failed, retrying")
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to ping database after %d attempts: %w", retries+1, err)
	}

	database := &Database{
		Pool: pool,
		log:  logger,
	}

	logger.Info().Msg("connected to the database")
	return database, nil
}

func (db *Database) Close() error {
	db.log.Info().Msg("closing database connection pool")
	db.Pool.Close()
	return nil
}
