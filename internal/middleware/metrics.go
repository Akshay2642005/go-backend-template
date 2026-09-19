package middleware

import (
	"backend/internal/server"

	"github.com/labstack/echo/v4"
)

// MetricsMiddleware is a no-op middleware (Sentry removed).
type MetricsMiddleware struct {
	server *server.Server
}

func NewMetricsMiddleware(s *server.Server) *MetricsMiddleware {
	return &MetricsMiddleware{
		server: s,
	}
}

// Instrument is a no-op middleware (Sentry removed).
func (m *MetricsMiddleware) Instrument() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	}
}
