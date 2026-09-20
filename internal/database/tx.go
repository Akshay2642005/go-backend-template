package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier abstracts database query operations over both pool and transaction
// connections, enabling repository methods to accept either for testability.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxFunc is a function that executes within a transaction.
type TxFunc func(tx pgx.Tx) error

// TxOptions configures the transaction behavior.
type TxOptions struct {
	// IsoLevel sets the transaction isolation level.
	// Default: ReadCommitted.
	IsoLevel pgx.TxIsoLevel
	// AccessMode sets the access mode (ReadWrite or ReadOnly).
	// Default: ReadWrite.
	AccessMode pgx.TxAccessMode
}

// DefaultTxOptions returns sensible default transaction options.
func DefaultTxOptions() TxOptions {
	return TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	}
}

// WithTx executes fn within a database transaction. On success the transaction
// is committed; on error it is rolled back. The transaction is always cleaned up.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) error {
	return WithTxOptions(ctx, pool, DefaultTxOptions(), fn)
}

// WithTxOptions executes fn within a database transaction with custom options.
func WithTxOptions(ctx context.Context, pool *pgxpool.Pool, opts TxOptions, fn TxFunc) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   opts.IsoLevel,
		AccessMode: opts.AccessMode,
	})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// WithTxContext wraps WithTx and propagates the transaction into the context.
// Use this when repository methods need access to the transaction via context.
func WithTxContext(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	return WithTx(ctx, pool, func(tx pgx.Tx) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}

type txKey struct{}

// TxFromContext retrieves the transaction from context, if present.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
