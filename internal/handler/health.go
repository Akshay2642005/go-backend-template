package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"backend/internal/middleware"
	"backend/internal/server"

	"github.com/labstack/echo/v4"
)

type HealthHandler struct {
	Handler
}

func NewHealthHandler(s *server.Server) *HealthHandler {
	return &HealthHandler{
		Handler: NewHandler(s),
	}
}

// CheckLiveness is the Kubernetes liveness probe.
// It always returns 200 OK if the process is running — no dependency checks.
func (h *HealthHandler) CheckLiveness(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().UTC(),
	})
}

// CheckReadiness is the Kubernetes readiness probe.
// It checks database and Redis connectivity; returns 203 if all pass,
// 503 if any dependency is unhealthy.
func (h *HealthHandler) CheckReadiness(c echo.Context) error {
	start := time.Now()
	logger := middleware.GetLogger(c).With().
		Str("operation", "readiness_check").
		Logger()

	checks := make(map[string]interface{})
	isReady := true

	// Check database connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbStart := time.Now()
	if err := h.server.DB.Pool.Ping(ctx); err != nil {
		checks["database"] = map[string]interface{}{
			"status":        "unavailable",
			"response_time": time.Since(dbStart).String(),
			"error":         err.Error(),
		}
		isReady = false
		logger.Error().Err(err).Dur("response_time", time.Since(dbStart)).Msg("database readiness check failed")
	} else {
		checks["database"] = map[string]interface{}{
			"status":        "available",
			"response_time": time.Since(dbStart).String(),
		}
	}

	// Check Redis connectivity (if configured)
	if h.server.Cache != nil {
		if h.server.Cache.Breaker().State() != 0 { // not StateClosed
			checks["redis"] = map[string]interface{}{
				"status":        "unavailable",
				"response_time": "0s",
				"error":         "circuit breaker is " + h.server.Cache.Breaker().State().String(),
			}
			isReady = false
			logger.Warn().Str("breaker_state", h.server.Cache.Breaker().State().String()).Msg("redis circuit breaker is open")
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			redisStart := time.Now()
			if err := h.server.Cache.Ping(ctx); err != nil {
				h.server.Cache.Breaker().Failure()
				checks["redis"] = map[string]interface{}{
					"status":        "unavailable",
					"response_time": time.Since(redisStart).String(),
					"error":         err.Error(),
				}
				isReady = false
				logger.Error().Err(err).Dur("response_time", time.Since(redisStart)).Msg("redis readiness check failed")
			} else {
				h.server.Cache.Breaker().Success()
				checks["redis"] = map[string]interface{}{
					"status":        "available",
					"response_time": time.Since(redisStart).String(),
				}
			}
		}
	}

	status := "ready"
	httpStatus := http.StatusOK
	if !isReady {
		status = "not_ready"
		httpStatus = http.StatusServiceUnavailable
		logger.Warn().
			Dur("total_duration", time.Since(start)).
			Msg("readiness check failed")
	} else {
		logger.Info().
			Dur("total_duration", time.Since(start)).
			Msg("readiness check passed")
	}

	return c.JSON(httpStatus, map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC(),
		"checks":    checks,
	})
}

// CheckHealth is the legacy /status endpoint with full health details.
// Kept for backward compatibility and debugging.
func (h *HealthHandler) CheckHealth(c echo.Context) error {
	start := time.Now()
	logger := middleware.GetLogger(c).With().
		Str("operation", "health_check").
		Logger()

	response := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().UTC(),
		"environment": h.server.Config.Primary.Env,
		"checks":      make(map[string]interface{}),
	}

	checks := response["checks"].(map[string]interface{})
	isHealthy := true

	// Check database connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbStart := time.Now()
	if err := h.server.DB.Pool.Ping(ctx); err != nil {
		checks["database"] = map[string]interface{}{
			"status":        "unhealthy",
			"response_time": time.Since(dbStart).String(),
			"error":         err.Error(),
		}
		isHealthy = false
		logger.Error().Err(err).Dur("response_time", time.Since(dbStart)).Msg("database health check failed")
	} else {
		checks["database"] = map[string]interface{}{
			"status":        "healthy",
			"response_time": time.Since(dbStart).String(),
		}
		logger.Info().Dur("response_time", time.Since(dbStart)).Msg("database health check passed")
	}

	// Check Redis connectivity
	if h.server.Cache != nil {
		// Check circuit breaker state first
		if h.server.Cache.Breaker().State() != 0 { // not StateClosed
			checks["redis"] = map[string]interface{}{
				"status":        "unhealthy",
				"response_time": "0s",
				"error":         "circuit breaker is " + h.server.Cache.Breaker().State().String(),
			}
			logger.Warn().Str("breaker_state", h.server.Cache.Breaker().State().String()).Msg("redis circuit breaker is open")
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			redisStart := time.Now()
			if err := h.server.Cache.Ping(ctx); err != nil {
				h.server.Cache.Breaker().Failure()
				checks["redis"] = map[string]interface{}{
					"status":        "unhealthy",
					"response_time": time.Since(redisStart).String(),
					"error":         err.Error(),
				}
				logger.Error().Err(err).Dur("response_time", time.Since(redisStart)).Msg("redis health check failed")
			} else {
				h.server.Cache.Breaker().Success()
				checks["redis"] = map[string]interface{}{
					"status":        "healthy",
					"response_time": time.Since(redisStart).String(),
				}
				logger.Info().Dur("response_time", time.Since(redisStart)).Msg("redis health check passed")
			}
		}
	}

	// Set overall status
	if !isHealthy {
		response["status"] = "unhealthy"
		logger.Warn().
			Dur("total_duration", time.Since(start)).
			Msg("health check failed")
		return c.JSON(http.StatusServiceUnavailable, response)
	}

	logger.Info().
		Dur("total_duration", time.Since(start)).
		Msg("health check passed")

	err := c.JSON(http.StatusOK, response)
	if err != nil {
		logger.Error().Err(err).Msg("failed to write JSON response")
		return fmt.Errorf("failed to write JSON response: %w", err)
	}

	return nil
}
