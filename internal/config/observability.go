package config

import (
	"fmt"
	"time"
)

const (
	// DefaultServiceName is used when no service name is configured.
	DefaultServiceName = "boilerplate"
)

type ObservabilityConfig struct {
	ServiceName  string             `koanf:"service_name" validate:"required"`
	Environment  string             `koanf:"environment" validate:"required"`
	Logging      LoggingConfig      `koanf:"logging"`
	HealthChecks HealthChecksConfig `koanf:"health_checks"`
	Tracing      TracingConfig      `koanf:"tracing"`
	Metrics      MetricsConfig      `koanf:"metrics"`
}

type LoggingConfig struct {
	Level              string        `koanf:"level" validate:"required"`
	Format             string        `koanf:"format" validate:"required"`
	SlowQueryThreshold time.Duration `koanf:"slow_query_threshold"`
}

type HealthChecksConfig struct {
	Enabled  bool          `koanf:"enabled"`
	Interval time.Duration `koanf:"interval" validate:"min=1s"`
	Timeout  time.Duration `koanf:"timeout" validate:"min=1s"`
	Checks   []string      `koanf:"checks"`
}

type TracingConfig struct {
	Enabled    bool    `koanf:"enabled"`
	Endpoint   string  `koanf:"endpoint"`
	Insecure   bool    `koanf:"insecure"`
	SampleRate float64 `koanf:"sample_rate"`
}

type MetricsConfig struct {
	Enabled  bool          `koanf:"enabled"`
	Endpoint string        `koanf:"endpoint"`
	Insecure bool          `koanf:"insecure"`
	Interval time.Duration `koanf:"interval"`
}

func DefaultObservabilityConfig() *ObservabilityConfig {
	return &ObservabilityConfig{
		ServiceName: DefaultServiceName,
		Environment: "development",
		Logging: LoggingConfig{
			Level:              "info",
			Format:             "json",
			SlowQueryThreshold: 100 * time.Millisecond,
		},
		HealthChecks: HealthChecksConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
			Checks:   []string{"database", "redis"},
		},
		Tracing: TracingConfig{
			Enabled:    false,
			Endpoint:   "localhost:4317",
			Insecure:   true,
			SampleRate: 1.0,
		},
		Metrics: MetricsConfig{
			Enabled:  false,
			Endpoint: "localhost:4317",
			Insecure: true,
			Interval: 15 * time.Second,
		},
	}
}

func (c *ObservabilityConfig) Validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("service_name is required")
	}

	// Validate log level
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging level: %s (must be one of: debug, info, warn, error)", c.Logging.Level)
	}

	// Validate slow query threshold
	if c.Logging.SlowQueryThreshold < 0 {
		return fmt.Errorf("logging slow_query_threshold must be non-negative")
	}

	// Validate tracing
	if c.Tracing.Enabled && c.Tracing.Endpoint == "" {
		return fmt.Errorf("tracing.endpoint is required when tracing is enabled")
	}
	if c.Tracing.SampleRate < 0 || c.Tracing.SampleRate > 1 {
		return fmt.Errorf("tracing.sample_rate must be between 0 and 1")
	}

	// Validate metrics
	if c.Metrics.Enabled && c.Metrics.Endpoint == "" {
		return fmt.Errorf("metrics.endpoint is required when metrics is enabled")
	}
	if c.Metrics.Interval <= 0 {
		c.Metrics.Interval = 15 * time.Second
	}

	return nil
}

func (c *ObservabilityConfig) GetLogLevel() string {
	switch c.Environment {
	case "production":
		if c.Logging.Level == "" {
			return "info"
		}
	case "development":
		if c.Logging.Level == "" {
			return "debug"
		}
	}
	return c.Logging.Level
}

func (c *ObservabilityConfig) IsProduction() bool {
	return c.Environment == "production"
}
