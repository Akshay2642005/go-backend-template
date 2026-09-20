package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"backend/internal/config"
)

// Cache provides an enterprise-grade Redis cache layer with
// single, cluster, and sentinel support, plus observability and fail-open.
type Cache struct {
	client             redis.UniversalClient
	logger             zerolog.Logger
	metrics            *metrics
	sf                 *group
	prefix             string
	defaultTTL         time.Duration
	slowQueryThreshold time.Duration
}

// New creates a new Cache from the Redis configuration.
// On connection failure it logs a warning but does not panic (fail-open).
func New(cfg *config.Config, logger *zerolog.Logger) *Cache {
	redisCfg := cfg.Redis
	log := logger.With().Str("component", "cache").Logger()

	var client redis.UniversalClient

	switch redisCfg.Mode {
	case "cluster":
		client = newClusterClient(redisCfg)
	case "sentinel":
		client = newSentinelClient(redisCfg)
	default:
		client = newSingleClient(redisCfg)
	}

	// Test connection (non-blocking; warn and continue on failure)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Warn().Err(err).Msg("failed to connect to Redis — cache will operate in fail-open mode")
	} else {
		log.Info().Str("mode", redisCfg.Mode).Str("address", redisCfg.Address).Msg("Redis cache connected")
	}

	var threshold time.Duration
	if cfg.Observability != nil {
		threshold = cfg.Observability.Logging.SlowQueryThreshold
	}
	if threshold == 0 {
		threshold = 100 * time.Millisecond
	}

	return &Cache{
		client:             client,
		logger:             log,
		metrics:            &metrics{},
		sf:                 newGroup(),
		prefix:             redisCfg.KeyPrefix,
		defaultTTL:         redisCfg.DefaultTTL,
		slowQueryThreshold: threshold,
	}
}

// newSingleClient creates a standalone Redis client.
func newSingleClient(cfg config.RedisConfig) *redis.Client {
	opts := &redis.Options{
		Addr:            cfg.Address,
		Username:        cfg.Username,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	}

	if cfg.TLSEnabled {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}
	}

	return redis.NewClient(opts)
}

// newClusterClient creates a Redis Cluster client.
func newClusterClient(cfg config.RedisConfig) *redis.ClusterClient {
	addrs := strings.Split(cfg.Address, ",")

	opts := &redis.ClusterOptions{
		Addrs:           addrs,
		Username:        cfg.Username,
		Password:        cfg.Password,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	}

	if cfg.TLSEnabled {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}
	}

	return redis.NewClusterClient(opts)
}

// newSentinelClient creates a Redis Sentinel (failover) client.
func newSentinelClient(cfg config.RedisConfig) *redis.Client {
	sentinels := strings.Split(cfg.Address, ",")

	opts := &redis.FailoverOptions{
		MasterName:      cfg.Username, // reuse Username field for master name in sentinel mode
		SentinelAddrs:   sentinels,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	}

	if cfg.TLSEnabled {
		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}
	}

	return redis.NewFailoverClient(opts)
}

// Client returns the underlying redis.UniversalClient for advanced use cases.
func (c *Cache) Client() redis.UniversalClient {
	return c.client
}

// Ping checks Redis connectivity.
func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis connection pool.
func (c *Cache) Close() error {
	return c.client.Close()
}

// Prefix returns the configured key prefix.
func (c *Cache) Prefix() string {
	return c.prefix
}

// DefaultTTL returns the configured default TTL.
func (c *Cache) DefaultTTL() time.Duration {
	return c.defaultTTL
}

// Metrics returns a snapshot of cache performance counters.
func (c *Cache) Metrics() MetricsSnapshot {
	return c.metrics.Snapshot()
}

// MakeKey builds a namespaced key using the configured prefix.
func (c *Cache) MakeKey(parts ...interface{}) string {
	return Key(c.prefix, parts...)
}

// MakePattern builds a SCAN pattern using the configured prefix.
func (c *Cache) MakePattern(parts ...interface{}) string {
	return Pattern(c.prefix, parts...)
}

// String returns a human-readable summary of the cache configuration.
func (c *Cache) String() string {
	return fmt.Sprintf("Cache{prefix=%q, defaultTTL=%s}", c.prefix, c.defaultTTL)
}
