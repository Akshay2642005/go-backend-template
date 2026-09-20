package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	maxBodyLogSize = 1024 // 1 KB
)

// BodyLoggerConfig holds configuration for the body logging middleware.
type BodyLoggerConfig struct {
	// Enabled toggles request/response body logging.
	Enabled bool
	// SkipPaths is a list of path prefixes to skip logging for.
	SkipPaths []string
}

// BodyLogger is middleware that logs request and response bodies at debug
// level, truncating payloads larger than 1 KB. Health, docs, and static
// paths are skipped by default.
type BodyLogger struct {
	config    BodyLoggerConfig
	skipPaths map[string]bool
}

// NewBodyLogger creates a new BodyLogger middleware.
func NewBodyLogger(config BodyLoggerConfig) *BodyLogger {
	skip := make(map[string]bool)
	for _, p := range config.SkipPaths {
		skip[p] = true
	}
	// Always skip health and docs endpoints
	skip["/healthz"] = true
	skip["/readyz"] = true
	skip["/status"] = true
	skip["/docs"] = true
	skip["/static"] = true

	return &BodyLogger{
		config:    config,
		skipPaths: skip,
	}
}

// Handle returns an Echo middleware that logs request and response bodies.
func (bl *BodyLogger) Handle() echo.MiddlewareFunc {
	if !bl.config.Enabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if bl.shouldSkip(path) {
				return next(c)
			}

			logger := GetLogger(c).With().
				Str("component", "body_logger").
				Logger()

			// Log request body (only for methods that have a body)
			method := c.Request().Method
			if method == "POST" || method == "PUT" || method == "PATCH" {
				if c.Request().Body != nil {
					body, err := io.ReadAll(c.Request().Body)
					if err != nil {
						logger.Error().Err(err).Msg("failed to read request body")
					} else {
						c.Request().Body = io.NopCloser(bytes.NewBuffer(body))
						logger.Debug().
							Str("body", truncateBody(body)).
							Int("size", len(body)).
							Msg("request body")
					}
				}
			}

			// Capture response body
			res := c.Response()
			resBody := &bytes.Buffer{}
			res.Writer = &bodyCapture{Writer: res.Writer, body: resBody}

			err := next(c)

			// Log response body
			if resBody.Len() > 0 {
				logger.Debug().
					Str("body", truncateBody(resBody.Bytes())).
					Int("size", resBody.Len()).
					Msg("response body")
			}

			return err
		}
	}
}

func (bl *BodyLogger) shouldSkip(path string) bool {
	// Exact match
	if bl.skipPaths[path] {
		return true
	}
	// Prefix match for paths like /static/...
	for prefix := range bl.skipPaths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// truncateBody truncates a byte slice to maxBodyLogSize and adds "..." if truncated.
func truncateBody(b []byte) string {
	if len(b) <= maxBodyLogSize {
		return string(b)
	}
	return string(b[:maxBodyLogSize]) + "...(truncated)"
}

// bodyCapture wraps a ResponseWriter to capture the response body.
type bodyCapture struct {
	io.Writer
	body *bytes.Buffer
}

func (bc *bodyCapture) Header() http.Header {
	if hw, ok := bc.Writer.(http.ResponseWriter); ok {
		return hw.Header()
	}
	return http.Header{}
}

func (bc *bodyCapture) WriteHeader(code int) {
	if hw, ok := bc.Writer.(http.ResponseWriter); ok {
		hw.WriteHeader(code)
	}
}

func (bc *bodyCapture) Write(b []byte) (int, error) {
	bc.body.Write(b)
	return bc.Writer.Write(b)
}

// Unwrap returns the underlying writer for http.ResponseController compatibility.
func (bc *bodyCapture) Unwrap() io.Writer {
	return bc.Writer
}
