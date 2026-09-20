package router

import (
	"github.com/labstack/echo/v4"

	"backend/internal/handler"
)

func registerSystemRoutes(r *echo.Echo, h *handler.Handlers) {
	// Liveness probe: always 200 if the process is running (no dependency checks)
	r.GET("/healthz", h.Health.CheckLiveness)

	// Readiness probe: checks DB + Redis, returns 503 if any dependency is down
	r.GET("/readyz", h.Health.CheckReadiness)

	// Legacy full health check with detailed dependency info
	r.GET("/status", h.Health.CheckHealth)

	r.Static("/static", "static")

	r.GET("/docs", h.OpenAPI.ServeOpenAPIUI)
}
