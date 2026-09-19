package config

import (
	"os"
	"strings"

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
	Port               string   `koanf:"port" validate:"required"`
	ReadTimeout        int      `koanf:"read_timeout" validate:"required"`
	WriteTimeout       int      `koanf:"write_timeout" validate:"required"`
	IdleTimeout        int      `koanf:"idle_timeout" validate:"required"`
	CORSAllowedOrigins []string `koanf:"cors_allowed_origins" validate:"required"`
}

type DatabaseConfig struct {
	Host            string `koanf:"host" validate:"required"`
	Port            int    `koanf:"port" validate:"required"`
	User            string `koanf:"user" validate:"required"`
	Password        string `koanf:"password"`
	Name            string `koanf:"name" validate:"required"`
	SSLMode         string `koanf:"ssl_mode" validate:"required"`
	MaxOpenConns    int    `koanf:"max_open_conns" validate:"required"`
	MaxIdleConns    int    `koanf:"max_idle_conns" validate:"required"`
	ConnMaxLifetime int    `koanf:"conn_max_lifetime" validate:"required"`
	ConnMaxIdleTime int    `koanf:"conn_max_idle_time" validate:"required"`
}

type RedisConfig struct {
	Address string `koanf:"address" validate:"required"`
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

	return cfg.Observability.Validate()
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
}
