package middleware

import (
	"backend/internal/server"
)

type Middlewares struct {
	Global          *GlobalMiddlewares
	Auth            *AuthMiddleware
	ContextEnhancer *ContextEnhancer
	Tracing         *TracingMiddleware
	RateLimit       *RateLimitMiddleware
	Metrics         *MetricsMiddleware
	CacheControl    *CacheControlMiddleware
	Idempotency     *idempotencyMiddleware
	RequestTimeout  *RequestTimeoutMiddleware
}

func NewMiddlewares(s *server.Server) *Middlewares {
	return &Middlewares{
		Global:          NewGlobalMiddlewares(s),
		Auth:            NewAuthMiddleware(s),
		ContextEnhancer: NewContextEnhancer(s),
		Tracing:         NewTracingMiddleware(s),
		RateLimit:       NewRateLimitMiddleware(s),
		Metrics:         NewMetricsMiddleware(s),
		CacheControl:    NewCacheControlMiddleware(s, CacheControlConfig{}),
		Idempotency:     NewIdempotencyMiddleware(s, IdempotencyConfig{}),
		RequestTimeout:  NewRequestTimeoutMiddleware(s),
	}
}
