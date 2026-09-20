package cache

import "errors"

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = errors.New("cache miss")

// IsCacheMiss reports whether the error is a cache miss.
func IsCacheMiss(err error) bool {
	return errors.Is(err, ErrCacheMiss)
}
