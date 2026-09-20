package database

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTxCommit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := WithTx(context.Background(), db.Pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS tx_test (id serial PRIMARY KEY, val text)")
		return err
	})
	require.NoError(t, err)

	// Verify table was committed
	var exists bool
	err = db.Pool.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tx_test')",
	).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)

	// Clean up
	_, _ = db.Pool.Exec(context.Background(), "DROP TABLE IF EXISTS tx_test")
}

func TestWithTxRollback(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := WithTx(context.Background(), db.Pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), "CREATE TABLE tx_rollback_test (id serial PRIMARY KEY)")
		if err != nil {
			return err
		}
		return assert.AnError
	})
	assert.Error(t, err)

	// Verify table was NOT committed
	var exists bool
	err = db.Pool.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tx_rollback_test')",
	).Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestWithTxReadOnly(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a test table
	_, err := db.Pool.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS ro_test (id serial PRIMARY KEY, val text)")
	require.NoError(t, err)
	defer func() { _, _ = db.Pool.Exec(context.Background(), "DROP TABLE IF EXISTS ro_test") }()

	// Try to write in a read-only transaction — should fail
	err = WithTxOptions(context.Background(), db.Pool, TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadOnly,
	}, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), "INSERT INTO ro_test (val) VALUES ('test')")
		return err
	})
	assert.Error(t, err)
}

func TestDefaultTxOptions(t *testing.T) {
	opts := DefaultTxOptions()
	assert.Equal(t, pgx.ReadCommitted, opts.IsoLevel)
	assert.Equal(t, pgx.ReadWrite, opts.AccessMode)
}
