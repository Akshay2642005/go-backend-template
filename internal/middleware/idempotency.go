package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/cache"
	"backend/internal/server"
)

type IdempotencyConfig struct {
	Enabled   bool          `koanf:"enabled"`
	TTL       time.Duration `koanf:"ttl"`
	KeyPrefix string        `koanf:"key_prefix"`
	SkipPaths []string      `koanf:"skip_paths"`
}

type idempotencyMiddleware struct {
	server *server.Server
	config IdempotencyConfig
}

type idempotencyEntry struct {
	StatusCode  int       `json:"status_code"`
	Body        []byte    `json:"body"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewIdempotencyMiddleware(s *server.Server, cfg IdempotencyConfig) *idempotencyMiddleware {
	if cfg.TTL == 0 {
		cfg.TTL = 24 * time.Hour
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "idemp:"
	}

	return &idempotencyMiddleware{
		server: s,
		config: cfg,
	}
}

func (im *idempotencyMiddleware) Handle() echo.MiddlewareFunc {
	if !im.config.Enabled || im.server.Cache == nil {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Only enforce for methods with side effects
			method := c.Request().Method
			if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
				return next(c)
			}

			// Skip configured paths
			for _, prefix := range im.config.SkipPaths {
				if strings.HasPrefix(c.Path(), prefix) {
					return next(c)
				}
			}

			// Check for idempotency key header
			idempotencyKey := c.Request().Header.Get("Idempotency-Key")
			if idempotencyKey == "" {
				return next(c)
			}

			// Build cache key
			cacheKey := im.buildKey(c, idempotencyKey)

			// Check for existing entry
			ctx, cancel := context.WithTimeout(c.Request().Context(), 500*time.Millisecond)
			defer cancel()

			var entry idempotencyEntry
			if err := cache.Get(im.server.Cache, ctx, cacheKey, &entry); err == nil {
				// Return cached response
				for k, v := range im.getResponseHeaders(c) {
					c.Response().Header().Set(k, v)
				}
				c.Response().Header().Set("Idempotency-Key", idempotencyKey)
				c.Response().Header().Set("X-Idempotent-Replay", "true")
				return c.Blob(entry.StatusCode, entry.ContentType, entry.Body)
			}

			// Capture response
			rec := &idempotencyRecorder{
				body:   &bytes.Buffer{},
				header: make(http.Header),
			}
			origWriter := c.Response().Writer
			c.Response().Writer = rec

			err := next(c)

			// Restore original writer
			c.Response().Writer = origWriter

			if err != nil {
				return err
			}

			// Only cache successful responses
			if rec.status >= 200 && rec.status < 300 && rec.body.Len() > 0 {
				entry := idempotencyEntry{
					StatusCode:  rec.status,
					Body:        rec.body.Bytes(),
					ContentType: rec.contentType(),
					CreatedAt:   time.Now(),
				}

				// Store in cache (best-effort)
				storeCtx, storeCancel := context.WithTimeout(c.Request().Context(), 500*time.Millisecond)
				defer storeCancel()
				_ = cache.Set(im.server.Cache, storeCtx, cacheKey, entry, im.config.TTL)
			}

			return nil
		}
	}
}

func (im *idempotencyMiddleware) buildKey(c echo.Context, idempotencyKey string) string {
	// Include method, path, and user ID for scoped idempotency
	parts := []interface{}{
		c.Request().Method,
		c.Path(),
		idempotencyKey,
	}

	if userID := GetUserID(c); userID != "" {
		parts = append(parts, "user:"+userID)
	}

	return cache.Key(im.config.KeyPrefix, parts...)
}

func (im *idempotencyMiddleware) getResponseHeaders(c echo.Context) map[string]string {
	headers := make(map[string]string)
	for _, k := range []string{"Content-Type"} {
		if v := c.Response().Header().Get(k); v != "" {
			headers[k] = v
		}
	}
	return headers
}

type idempotencyRecorder struct {
	body   *bytes.Buffer
	header http.Header
	status int
}

func (r *idempotencyRecorder) Write(p []byte) (n int, err error) {
	return r.body.Write(p)
}

func (r *idempotencyRecorder) WriteHeader(code int) {
	r.status = code
}

func (r *idempotencyRecorder) Header() http.Header {
	return r.header
}

func (r *idempotencyRecorder) Flush() {}

func (r *idempotencyRecorder) contentType() string {
	if r.body.Len() == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(r.body.Bytes())
}

// BuildIdempotencyKeyHash creates a short hash from request body for key generation.
func BuildIdempotencyKeyHash(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:8])
}

// FormatRetryAfter formats a duration as a Retry-After header value.
func FormatRetryAfter(d time.Duration) string {
	return strconv.Itoa(int(d.Seconds()))
}

// FormatRateLimitReset formats a time as a Rate-Limit-Reset header value.
func FormatRateLimitReset(t time.Time) string {
	return fmt.Sprintf("%d", t.Unix())
}
