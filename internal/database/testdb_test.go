package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testDB struct {
	Pool      *pgxpool.Pool
	Container testcontainers.Container
}

func setupTestDB(t *testing.T) (*testDB, func()) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Skipping test: Docker not available or testcontainers error: %v", r)
		}
	}()

	ctx := context.Background()
	dbName := fmt.Sprintf("test_%s", uuid.New().String()[:8])

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       dbName,
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("Skipping test: Docker not available: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Skipf("Skipping test: failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Skipf("Skipping test: failed to get mapped port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://testuser:testpass@%s:%d/%s?sslmode=disable", host, port.Int(), dbName)

	var pool *pgxpool.Pool
	require.Eventually(t, func() bool {
		pool, err = pgxpool.New(ctx, dsn)
		return err == nil && pool.Ping(ctx) == nil
	}, 15*time.Second, 500*time.Millisecond, "failed to connect to database container")

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	_ = logger

	t.Cleanup(func() {
		pool.Close()
		container.Terminate(ctx)
	})

	return &testDB{Pool: pool, Container: container}, func() {
		pool.Close()
		container.Terminate(ctx)
	}
}
