package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/errs"
	"backend/internal/server"
)

// RequestTimeoutMiddleware enforces a per-request context timeout.
type RequestTimeoutMiddleware struct {
	server *server.Server
}

func NewRequestTimeoutMiddleware(s *server.Server) *RequestTimeoutMiddleware {
	return &RequestTimeoutMiddleware{server: s}
}

// Handle returns middleware that injects a deadline into each request's context.
// The timeout is configurable via BOILERPLATE_SERVER.REQUEST_TIMEOUT (seconds).
// Defaults to 0 (no timeout) if unset.
func (rt *RequestTimeoutMiddleware) Handle() echo.MiddlewareFunc {
	timeout := rt.getTimeout()
	if timeout <= 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			defer cancel()
			c.SetRequest(c.Request().WithContext(ctx))

			// Wrap next handler to check if context expired
			err := next(c)
			if ctx.Err() == context.DeadlineExceeded {
				return errs.NewProblem(
					http.StatusGatewayTimeout,
					"Gateway Timeout",
					"Request processing exceeded the time limit",
				)
			}
			return err
		}
	}
}

func (rt *RequestTimeoutMiddleware) getTimeout() time.Duration {
	if rt.server.Config.Server.RequestTimeout > 0 {
		return time.Duration(rt.server.Config.Server.RequestTimeout) * time.Second
	}
	return 0
}
