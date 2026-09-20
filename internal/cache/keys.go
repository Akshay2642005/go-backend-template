package cache

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Key builds a namespaced cache key from the prefix and parts joined with ":".
// Long segments are SHA-256 hashed to keep Redis keys compact.
func Key(prefix string, parts ...interface{}) string {
	var b strings.Builder

	if prefix != "" {
		b.WriteString(prefix)
	}

	for _, p := range parts {
		b.WriteString(":")
		s := fmt.Sprintf("%v", p)
		if len(s) > 128 {
			h := sha256.Sum256([]byte(s))
			s = fmt.Sprintf("%x", h[:8])
		}
		b.WriteString(s)
	}

	return b.String()
}

// KeyWithEnv builds a key prefixed with "boilerplate:{env}:".
func KeyWithEnv(env string, parts ...interface{}) string {
	prefix := "boilerplate"
	if env != "" {
		prefix += ":" + env
	}
	return Key(prefix, parts...)
}

// Pattern builds a SCAN/DELETE pattern with a wildcard suffix.
func Pattern(prefix string, parts ...interface{}) string {
	return Key(prefix, parts...) + ":*"
}

// TTLWithJitter returns the TTL with a random jitter of +/-jitterPct (0.0-1.0).
// The jitter prevents cache stampedes when many keys expire simultaneously.
func TTLWithJitter(ttl time.Duration, jitterPct float64) time.Duration {
	if jitterPct <= 0 || ttl <= 0 {
		return ttl
	}

	half := jitterPct * float64(ttl)
	jitter := time.Duration(rand.Int63n(int64(2*half+1))) - time.Duration(half)
	return ttl + jitter
}
