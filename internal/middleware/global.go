package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"backend/internal/errs"
	"backend/internal/server"
	"backend/internal/sqlerr"
)

type GlobalMiddlewares struct {
	server *server.Server
}

func NewGlobalMiddlewares(s *server.Server) *GlobalMiddlewares {
	return &GlobalMiddlewares{
		server: s,
	}
}

func (global *GlobalMiddlewares) CORS() echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: global.server.Config.Server.CORSAllowedOrigins,
	})
}

func (global *GlobalMiddlewares) RequestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:     true,
		LogStatus:  true,
		LogError:   true,
		LogLatency: true,
		LogHost:    true,
		LogMethod:  true,
		LogURIPath: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			statusCode := v.Status

			// note that the status code is not set yet as it gets picked up by the global err handler
			// see here: https://github.com/labstack/echo/issues/2310#issuecomment-1288196898
			if v.Error != nil {
				var httpErr *errs.HTTPError
				var echoErr *echo.HTTPError
				if errors.As(v.Error, &httpErr) {
					statusCode = httpErr.Status
				} else if errors.As(v.Error, &echoErr) {
					statusCode = echoErr.Code
				}
			}

			// Get enhanced logger from context
			logger := GetLogger(c)

			var e *zerolog.Event

			switch {
			case statusCode >= 500:
				e = logger.Error().Err(v.Error)
			case statusCode >= 400:
				e = logger.Warn()
			default:
				e = logger.Info()
			}

			// Add request ID if available
			if requestID := GetRequestID(c); requestID != "" {
				e = e.Str("request_id", requestID)
			}

			// Add user context if available
			if userID := GetUserID(c); userID != "" {
				e = e.Str("user_id", userID)
			}

			e.
				Dur("latency", v.Latency).
				Int("status", statusCode).
				Str("method", v.Method).
				Str("uri", v.URI).
				Str("host", v.Host).
				Str("ip", c.RealIP()).
				Str("user_agent", c.Request().UserAgent()).
				Msg("API")

			return nil
		},
	})
}

func (global *GlobalMiddlewares) Recover() echo.MiddlewareFunc {
	return middleware.Recover()
}

func (global *GlobalMiddlewares) Secure() echo.MiddlewareFunc {
	return middleware.SecureWithConfig(middleware.SecureConfig{
		Skipper:            nil,
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "DENY",
		HSTSPreloadEnabled: true,
		HSTSMaxAge:         global.getHSTSMaxAge(),
	})
}

func (global *GlobalMiddlewares) getHSTSMaxAge() int {
	if global.server.Config.Server.SecurityHSTS > 0 {
		return global.server.Config.Server.SecurityHSTS
	}
	// Default: 1 year in production, 0 in dev
	if global.server.Config.Primary.Env == "production" {
		return 31536000
	}
	return 0
}

// Compression returns gzip compression middleware.
func (global *GlobalMiddlewares) Compression() echo.MiddlewareFunc {
	return middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: func(c echo.Context) bool {
			// Skip compression for small responses (Content-Length < 1KB)
			// and for already-compressed content types
			cl := c.Response().Header().Get("Content-Length")
			if cl == "0" || cl == "" {
				return false // let it through; size check happens after
			}
			return false
		},
		Level: 5, // balanced speed/ratio
	})
}

func (global *GlobalMiddlewares) GlobalErrorHandler(err error, c echo.Context) {
	// First try to handle database errors and convert them to appropriate HTTP errors
	originalErr := err

	// Try to handle known database errors
	// Only do this for errors that haven't already been converted to HTTPError
	var httpErr *errs.HTTPError
	if !errors.As(err, &httpErr) {
		var echoErr *echo.HTTPError
		if errors.As(err, &echoErr) {
			if echoErr.Code == http.StatusNotFound {
				err = errs.NewNotFoundError("Route not found", false, nil)
			}
		} else {
			// Here we call our sqlerr handler which will convert database errors
			// to appropriate application errors
			err = sqlerr.HandleError(err)
		}
	}

	// Now process the possibly converted error into an RFC 7807 problem
	var echoErr *echo.HTTPError
	var problem *errs.HTTPError

	switch {
	case errors.As(err, &httpErr):
		problem = httpErr

	case errors.As(err, &echoErr):
		status := echoErr.Code
		code := errs.MakeUpperCaseWithUnderscores(http.StatusText(status))
		msg := http.StatusText(status)
		if m, ok := echoErr.Message.(string); ok {
			msg = m
		}
		problem = errs.NewProblemWithCode(status, code, http.StatusText(status), msg)

	default:
		problem = errs.NewInternalServerError()
	}

	// Populate Instance with the request ID for traceability
	if requestID := GetRequestID(c); requestID != "" {
		problem.Instance = requestID
	}

	// Log the original error to help with debugging
	// 5xx are logged at error level; 4xx at warning level.
	logger := *GetLogger(c)

	var e *zerolog.Event
	if problem.Status >= http.StatusInternalServerError {
		e = logger.Error().Stack()
	} else {
		e = logger.Warn().Stack()
	}

	e.
		Err(originalErr).
		Int("status", problem.Status).
		Str("error_code", problem.Code).
		Str("error_type", problem.Type).
		Msg(problem.Detail)

	if !c.Response().Committed {
		c.Response().Header().Set(echo.HeaderContentType, "application/problem+json")
		_ = c.JSON(problem.Status, problem)
	}
}
