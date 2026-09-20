package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"backend/internal/cache"
	"backend/internal/server"
)

// CacheControlConfig configures the HTTP response caching middleware.
type CacheControlConfig struct {
	TTL                time.Duration
	Private            bool
	KeyPrefix          string
	SkipPaths          []string
	MaxBodySize        int
	CacheControlHeader string
}

// CacheControlMiddleware handles HTTP-level caching with ETag and Cache-Control.
type CacheControlMiddleware struct {
	server *server.Server
	config CacheControlConfig
}

// NewCacheControlMiddleware creates a new CacheControl middleware.
func NewCacheControlMiddleware(s *server.Server, cfg CacheControlConfig) *CacheControlMiddleware {
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = 1 << 20
	}
	if cfg.TTL == 0 {
		cfg.TTL = 5 * time.Minute
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "http:"
	}
	return &CacheControlMiddleware{server: s, config: cfg}
}

// Handle returns an echo.MiddlewareFunc that intercepts 2xx GET responses,
// caches them, and serves 304 Not Modified for matching ETags.
func (m *CacheControlMiddleware) Handle() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method != http.MethodGet {
				return next(c)
			}
			for _, prefix := range m.config.SkipPaths {
				if strings.HasPrefix(c.Path(), prefix) {
					return next(c)
				}
			}

			key := m.buildKey(c)

			// Try to serve from cache
			if m.server.Cache != nil {
				var cached cachedResponse
				if err := cache.Get(m.server.Cache, c.Request().Context(), key, &cached); err == nil {
					if match := c.Request().Header.Get("If-None-Match"); match != "" && match == cached.ETag {
						return c.NoContent(http.StatusNotModified)
					}
					for k, v := range cached.Headers {
						c.Response().Header().Set(k, v)
					}
					return c.Blob(cached.Status, cached.ContentType, cached.Body)
				}
			}

			// Capture response by wrapping the writer
			rec := &responseRecorder{
				Writer: &bytes.Buffer{},
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
			if rec.status < 200 || rec.status >= 300 {
				return nil
			}
			if rec.Writer.Len() > m.config.MaxBodySize {
				return nil
			}
			if noStore(c.Response().Header().Get("Cache-Control")) {
				return nil
			}

			h := sha256.Sum256(rec.Writer.Bytes())
			etag := `"` + hex.EncodeToString(h[:16]) + `"`

			if match := c.Request().Header.Get("If-None-Match"); match == etag {
				return c.NoContent(http.StatusNotModified)
			}

			ccHeader := m.config.CacheControlHeader
			if ccHeader == "" {
				ccHeader = "public, max-age=" + strconv.Itoa(int(m.config.TTL.Seconds()))
				if m.config.Private {
					ccHeader = "private, max-age=" + strconv.Itoa(int(m.config.TTL.Seconds()))
				}
			}
			c.Response().Header().Set("Cache-Control", ccHeader)
			c.Response().Header().Set("ETag", etag)

			if m.server.Cache != nil {
				headers := make(map[string]string)
				for k := range rec.header {
					headers[k] = rec.header.Get(k)
				}
				stored := cachedResponse{
					Status:      rec.status,
					ContentType: rec.contentType(),
					Body:        rec.Writer.Bytes(),
					Headers:     headers,
					ETag:        etag,
					StoredAt:    time.Now(),
				}
				ctx, cancel := context.WithTimeout(c.Request().Context(), 500*time.Millisecond)
				defer cancel()
				_ = cache.Set(m.server.Cache, ctx, key, stored, m.config.TTL)
			}

			return nil
		}
	}
}

func (m *CacheControlMiddleware) buildKey(c echo.Context) string {
	parts := []interface{}{c.Path(), c.Request().URL.RawQuery}
	if m.config.Private {
		if userID := GetUserID(c); userID != "" {
			parts = append(parts, "user:"+userID)
		}
	}
	return cache.Key(m.config.KeyPrefix, parts...)
}

type cachedResponse struct {
	Status      int               `json:"status"`
	ContentType string            `json:"content_type"`
	Body        []byte            `json:"body"`
	Headers     map[string]string `json:"headers"`
	ETag        string            `json:"etag"`
	StoredAt    time.Time         `json:"stored_at"`
}

// responseRecorder captures writes for caching then replays them.
type responseRecorder struct {
	Writer *bytes.Buffer
	header http.Header
	status int
	once   sync.Once
}

func (r *responseRecorder) Write(p []byte) (n int, err error) {
	return r.Writer.Write(p)
}

func (r *responseRecorder) WriteHeader(code int) {
	r.once.Do(func() { r.status = code })
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Flush() {}

func (r *responseRecorder) Unwrap() http.ResponseWriter { return nil }

func (r *responseRecorder) contentType() string {
	if r.Writer.Len() == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(r.Writer.Bytes())
}

func noStore(header string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == "no-store" {
			return true
		}
	}
	return false
}
