package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"backend/internal/config"
	"backend/internal/server"
)

// slidingWindowScript is a Lua script for atomic sliding-window rate limiting.
// KEYS[1] = rate limit key
// ARGV[1] = window size in seconds
// ARGV[2] = max requests
// ARGV[3] = current timestamp in seconds
// ARGV[4] = unique request ID (prevents collisions)
// Returns: {allowed (0/1), current_count, retry_after_seconds}
var slidingWindowScript = `
local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local request_id = ARGV[4]

local window_start = now - window

-- Remove expired entries
redis.call('ZREMRANGEBYSCORE', key, 0, window_start)

-- Count current requests in window
local count = redis.call('ZCARD', key)

if count < limit then
    -- Allow: add this request
    redis.call('ZADD', key, now, request_id)
    redis.call('EXPIRE', key, window)
    return {1, count + 1, 0}
else
    -- Deny: calculate retry-after from oldest entry
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_after = 0
    if #oldest > 0 then
        retry_after = math.ceil(window - (now - tonumber(oldest[2])))
        if retry_after < 0 then
            retry_after = 0
        end
    end
    return {0, count, retry_after}
end
`

type RateLimitConfig struct {
	Enabled   bool          `koanf:"enabled"`
	Window    time.Duration `koanf:"window"`
	Max       int           `koanf:"max"`
	KeyPrefix string        `koanf:"key_prefix"`
	ByUser    bool          `koanf:"by_user"`
}

type RateLimitMiddleware struct {
	server *server.Server
	script *rateLimitScript
}

type rateLimitScript struct {
	sha1 string
}

func NewRateLimitMiddleware(s *server.Server) *RateLimitMiddleware {
	rl := &RateLimitMiddleware{
		server: s,
		script: &rateLimitScript{},
	}

	if s.Cache != nil && s.Config != nil && getRateLimitConfig(s.Config).Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sha, err := s.Cache.Client().ScriptLoad(ctx, slidingWindowScript).Result()
		if err != nil {
			s.Logger.Warn().Err(err).Msg("failed to load rate limit Lua script — falling back to in-memory")
		} else {
			rl.script.sha1 = sha
		}
	}

	return rl
}

func getRateLimitConfig(cfg *config.Config) config.RateLimitConfig {
	return cfg.Server.RateLimit
}

// Handle returns the rate limiting middleware.
func (r *RateLimitMiddleware) Handle() echo.MiddlewareFunc {
	cfg := getRateLimitConfig(r.server.Config)

	if !cfg.Enabled || r.script.sha1 == "" {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	if cfg.Window == 0 {
		cfg.Window = 60 * time.Second
	}
	if cfg.Max == 0 {
		cfg.Max = 100
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "ratelimit:"
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Determine client key
			key := r.buildKey(c, cfg)

			// Execute Lua script
			ctx, cancel := context.WithTimeout(c.Request().Context(), 500*time.Millisecond)
			defer cancel()

			now := time.Now().UnixMilli()
			windowSec := int64(cfg.Window.Seconds())
			requestID := uuid.New().String()

			result, err := r.server.Cache.Client().EvalSha(
				ctx,
				r.script.sha1,
				[]string{key},
				windowSec,
				cfg.Max,
				now,
				requestID,
			).Int64Slice()

			if err != nil {
				// Redis error — fail open, let request through
				r.server.Logger.Warn().Err(err).Str("key", key).Msg("rate limit check failed, allowing request")
				return next(c)
			}

			if len(result) < 3 {
				return next(c)
			}

			allowed := result[0] == 1
			count := result[1]
			retryAfter := result[2]

			if !allowed {
				c.Response().Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.Max))
				c.Response().Header().Set("X-RateLimit-Remaining", "0")
				c.Response().Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Duration(retryAfter)*time.Second).Unix()))
				if retryAfter > 0 {
					c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				}

				r.server.Logger.Warn().
					Str("request_id", GetRequestID(c)).
					Str("key", key).
					Int64("count", count).
					Dur("window", cfg.Window).
					Msg("rate limit exceeded")

				return echo.NewHTTPError(http.StatusTooManyRequests, "Rate limit exceeded")
			}

			// Set rate limit headers on successful requests
			c.Response().Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.Max))
			c.Response().Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", int64(cfg.Max)-count))
			c.Response().Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(cfg.Window).Unix()))

			return next(c)
		}
	}
}

func (r *RateLimitMiddleware) buildKey(c echo.Context, cfg config.RateLimitConfig) string {
	var identifier string

	if cfg.ByUser {
		if userID := GetUserID(c); userID != "" {
			identifier = "user:" + userID
		}
	}

	if identifier == "" {
		identifier = "ip:" + c.RealIP()
	}

	return cfg.KeyPrefix + identifier
}

// RecordRateLimitHit logs a rate limit violation.
func (r *RateLimitMiddleware) RecordRateLimitHit(endpoint string) {
	r.server.Logger.Warn().
		Str("endpoint", endpoint).
		Msg("rate limit hit recorded")
}
