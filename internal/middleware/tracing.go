package middleware

import (
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"

	"backend/internal/server"
)

type TracingMiddleware struct {
	server *server.Server
}

func NewTracingMiddleware(s *server.Server) *TracingMiddleware {
	return &TracingMiddleware{
		server: s,
	}
}

// EnhanceTracing instruments incoming requests with OpenTelemetry tracing.
func (tm *TracingMiddleware) EnhanceTracing() echo.MiddlewareFunc {
	// Check if tracing is enabled
	if tm.server.Config == nil || !tm.server.Config.Observability.Tracing.Enabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	return otelecho.Middleware(tm.server.Config.Observability.ServiceName)
}
