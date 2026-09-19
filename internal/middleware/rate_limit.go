package middleware

import (
	"backend/internal/server"
)

type RateLimitMiddleware struct {
	server *server.Server
}

func NewRateLimitMiddleware(s *server.Server) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		server: s,
	}
}

// RecordRateLimitHit is a no-op (Sentry removed).
func (r *RateLimitMiddleware) RecordRateLimitHit(endpoint string) {
	// No-op: Sentry removed
}
