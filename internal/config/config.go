package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	_ "github.com/joho/godotenv/autoload"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog"
)

type Config struct {
	Primary       Primary              `koanf:"primary" validate:"required"`
	Server        ServerConfig         `koanf:"server" validate:"required"`
	Database      DatabaseConfig       `koanf:"database" validate:"required"`
	Auth          AuthConfig           `koanf:"auth" validate:"required"`
	Redis         RedisConfig          `koanf:"redis" validate:"required"`
	Integration   IntegrationConfig    `koanf:"integration" validate:"required"`
	Observability *ObservabilityConfig `koanf:"observability"`
}

type Primary struct {
	Env string `koanf:"env" validate:"required"`
}

type ServerConfig struct {
	Port               string            `koanf:"port" validate:"required"`
	ReadTimeout        int               `koanf:"read_timeout" validate:"required"`
	WriteTimeout       int               `koanf:"write_timeout" validate:"required"`
	IdleTimeout        int               `koanf:"idle_timeout" validate:"required"`
	CORSAllowedOrigins []string          `koanf:"cors_allowed_origins" validate:"required"`
	RateLimit          RateLimitConfig   `koanf:"rate_limit"`
	Idempotency        IdempotencyConfig `koanf:"idempotency"`
	ShutdownTimeout    int               `koanf:"shutdown_timeout"`
	DrainTimeout       int               `koanf:"drain_timeout"`
	Compression        bool              `koanf:"compression"`
	RequestTimeout     int               `koanf:"request_timeout"`
	SecurityHSTS       int               `koanf:"security_hsts"`
}

type RateLimitConfig struct {
	Enabled   bool          `koanf:"enabled"`
	Window    time.Duration `koanf:"window"`
	Max       int           `koanf:"max"`
	KeyPrefix string        `koanf:"key_prefix"`
	ByUser    bool          `koanf:"by_user"`
}

type IdempotencyConfig struct {
	Enabled   bool          `koanf:"enabled"`
	TTL       time.Duration `koanf:"ttl"`
	KeyPrefix string        `koanf:"key_prefix"`
	SkipPaths []string      `koanf:"skip_paths"`
}

type DatabaseConfig struct {
	Host              string `koanf:"host" validate:"required"`
	Port              int    `koanf:"port" validate:"required"`
	User              string `koanf:"user" validate:"required"`
	Password          string `koanf:"password"`
	Name              string `koanf:"name" validate:"required"`
	SSLMode           string `koanf:"ssl_mode" validate:"required"`
	MaxOpenConns      int    `koanf:"max_open_conns" validate:"required"`
	MaxIdleConns      int    `koanf:"max_idle_conns" validate:"required"`
	ConnMaxLifetime   int    `koanf:"conn_max_lifetime" validate:"required"`
	ConnMaxIdleTime   int    `koanf:"conn_max_idle_time" validate:"required"`
	AutoMigrate       bool   `koanf:"auto_migrate"`
	ConnectRetries    int    `koanf:"connect_retries"`
	ConnectRetryDelay int    `koanf:"connect_retry_delay"`
}

type RedisConfig struct {
	Address         string        `koanf:"address" validate:"required"`
	Mode            string        `koanf:"mode"` // single | cluster | sentinel
	Username        string        `koanf:"username"`
	Password        string        `koanf:"password"`
	DB              int           `koanf:"db"`
	TLSEnabled      bool          `koanf:"tls_enabled"`
	TLSSkipVerify   bool          `koanf:"tls_skip_verify"`
	PoolSize        int           `koanf:"pool_size"`
	MinIdleConns    int           `koanf:"min_idle_conns"`
	DialTimeout     time.Duration `koanf:"dial_timeout"`
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	MaxRetries      int           `koanf:"max_retries"`
	MinRetryBackoff time.Duration `koanf:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `koanf:"max_retry_backoff"`
	KeyPrefix       string        `koanf:"key_prefix"`
	DefaultTTL      time.Duration `koanf:"default_ttl"`
}

func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Mode:            "single",
		PoolSize:        10,
		MinIdleConns:    5,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		MaxRetries:      3,
		MinRetryBackoff: 100 * time.Millisecond,
		MaxRetryBackoff: 3 * time.Second,
		DefaultTTL:      5 * time.Minute,
	}
}

func mergeRedisWithDefaults(cfg *RedisConfig, def RedisConfig) {
	if cfg.Mode == "" {
		cfg.Mode = def.Mode
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = def.PoolSize
	}
	if cfg.MinIdleConns == 0 {
		cfg.MinIdleConns = def.MinIdleConns
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = def.DialTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = def.ReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = def.WriteTimeout
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = def.MaxRetries
	}
	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = def.MinRetryBackoff
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = def.MaxRetryBackoff
	}
	if cfg.DefaultTTL == 0 {
		cfg.DefaultTTL = def.DefaultTTL
	}
}

type IntegrationConfig struct {
	ResendAPIKey string `koanf:"resend_api_key" validate:"required"`
}

type AuthConfig struct {
	SecretKey string `koanf:"secret_key" validate:"required"`
}

func LoadConfig() (*Config, error) {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	k := koanf.New(".")

	err := k.Load(env.Provider("BOILERPLATE_", ".", func(s string) string {
		return strings.ToLower(strings.TrimPrefix(s, "BOILERPLATE_"))
	}), nil)
	if err != nil {
		return nil, err
	}

	mainConfig := &Config{}

	err = k.Unmarshal("", mainConfig)
	if err != nil {
		return nil, err
	}

	// Set default observability config if not provided, otherwise merge the
	// partially provided values over the defaults.
	if mainConfig.Observability == nil {
		mainConfig.Observability = DefaultObservabilityConfig()
	} else {
		mergeObservabilityWithDefaults(mainConfig.Observability, DefaultObservabilityConfig())
	}

	// Override service name and environment from primary config
	mainConfig.Observability.ServiceName = DefaultServiceName
	mainConfig.Observability.Environment = mainConfig.Primary.Env

	// Merge Redis defaults for any unset fields.
	mergeRedisWithDefaults(&mainConfig.Redis, DefaultRedisConfig())

	if err := validateConfig(mainConfig); err != nil {
		logger.Error().Err(err).Msg("config validation failed")
		return nil, err
	}

	return mainConfig, nil
}

func validateConfig(cfg *Config) error {
	validate := validator.New()

	if err := validate.Struct(cfg); err != nil {
		return err
	}

	if err := cfg.Observability.Validate(); err != nil {
		return err
	}

	validRedisModes := map[string]bool{"single": true, "cluster": true, "sentinel": true}
	if !validRedisModes[cfg.Redis.Mode] {
		return fmt.Errorf("redis.mode must be one of: single, cluster, sentinel; got %q", cfg.Redis.Mode)
	}

	return nil
}

// mergeObservabilityWithDefaults fills any unset fields of the loaded
// observability config with the default values.
func mergeObservabilityWithDefaults(cfg, def *ObservabilityConfig) {
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = def.Logging.Level
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = def.Logging.Format
	}
	if cfg.Logging.SlowQueryThreshold == 0 {
		cfg.Logging.SlowQueryThreshold = def.Logging.SlowQueryThreshold
	}
	if cfg.HealthChecks.Interval == 0 {
		cfg.HealthChecks.Interval = def.HealthChecks.Interval
	}
	if cfg.HealthChecks.Timeout == 0 {
		cfg.HealthChecks.Timeout = def.HealthChecks.Timeout
	}
	if len(cfg.HealthChecks.Checks) == 0 {
		cfg.HealthChecks.Checks = def.HealthChecks.Checks
	}

	// Tracing defaults
	if cfg.Tracing.Endpoint == "" {
		cfg.Tracing.Endpoint = def.Tracing.Endpoint
	}
	if cfg.Tracing.SampleRate == 0 {
		cfg.Tracing.SampleRate = def.Tracing.SampleRate
	}

	// Metrics defaults
	if cfg.Metrics.Endpoint == "" {
		cfg.Metrics.Endpoint = def.Metrics.Endpoint
	}
	if cfg.Metrics.Interval == 0 {
		cfg.Metrics.Interval = def.Metrics.Interval
	}
}
