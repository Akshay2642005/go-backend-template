package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestRedis holds a Redis container and client for integration tests.
type TestRedis struct {
	Client    redis.UniversalClient
	Container testcontainers.Container
	Config    TestRedisConfig
}

type TestRedisConfig struct {
	Address string
	DB      int
}

// SetupTestRedis starts a Redis container for tests.
// Skips the test if Docker is not available.
func SetupTestRedis(t *testing.T) (*TestRedis, func()) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Skipping test: Docker not available or testcontainers error: %v", r)
		}
	}()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:8-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("Skipping test: Docker not available: %v", err)
	}

	host, err := container.Host(ctx)
	require.NoError(t, err, "failed to get container host")

	mappedPort, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err, "failed to get mapped port")

	addr := fmt.Sprintf("%s:%d", host, mappedPort.Int())

	client := redis.NewClient(&redis.Options{Addr: addr})

	// Verify connectivity
	require.Eventually(t, func() bool {
		return client.Ping(ctx).Err() == nil
	}, 10*time.Second, 100*time.Millisecond, "failed to connect to redis container")

	t.Cleanup(func() {
		_ = client.Close()
		_ = container.Terminate(ctx)
	})

	return &TestRedis{
		Client:    client,
		Container: container,
		Config:    TestRedisConfig{Address: addr, DB: 0},
	}, func() {
		_ = client.Close()
		_ = container.Terminate(ctx)
	}
}

// NewTestCache creates a Cache backed by the test Redis container.
func NewTestCache(t *testing.T, tr *TestRedis) *Cache {
	t.Helper()

	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	return &Cache{
		client:             tr.Client,
		logger:             logger,
		metrics:            &metrics{},
		sf:                 newGroup(),
		prefix:             "test:",
		defaultTTL:         5 * time.Minute,
		slowQueryThreshold: 100 * time.Millisecond,
	}
}
