package middleware

import (
	"backend/internal/server"

	"github.com/labstack/echo/v4"
)

type TracingMiddleware struct {
	server *server.Server
}

func NewTracingMiddleware(s *server.Server) *TracingMiddleware {
	return &TracingMiddleware{
		server: s,
	}
}

// EnhanceTracing is a no-op middleware (Sentry removed).
func (tm *TracingMiddleware) EnhanceTracing() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	}
}
