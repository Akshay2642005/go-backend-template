package otel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/config"
)

func TestInitDisabled(t *testing.T) {
	cfg := config.DefaultObservabilityConfig()
	cfg.Tracing.Enabled = false

	shutdown, err := Init(context.Background(), cfg, "test")
	require.NoError(t, err)
	assert.NotNil(t, shutdown)

	// Shutdown should succeed (no-op)
	err = shutdown(context.Background())
	assert.NoError(t, err)
}

func TestInitEnabledWithNoEndpointFails(t *testing.T) {
	cfg := config.DefaultObservabilityConfig()
	cfg.Tracing.Enabled = true
	cfg.Tracing.Endpoint = ""

	_, err := Init(context.Background(), cfg, "test")
	assert.Error(t, err)
}
