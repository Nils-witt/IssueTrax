// Package postgres implements the storage ports on PostgreSQL using pgx and
// sqlc-generated queries (see queries/ and sqlc.yaml). Schema migrations are
// embedded and applied with goose.
package postgres

//go:generate go tool sqlc generate -f ../../../../sqlc.yaml

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres/sqlc"
	"nilswitt.dev/issuetrax/internal/core/port"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Compile-time check that DB satisfies the driven port.
var _ port.TxManager = (*DB)(nil)

// Migrate applies all pending schema migrations. A Postgres advisory lock
// keeps concurrently starting instances from migrating at the same time.
func Migrate(ctx context.Context, databaseURL string) error {
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}

	db := stdlib.OpenDB(*cfg)
	defer func() { _ = db.Close() }()

	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration lock: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

// DB is a pgx connection pool that also implements port.TxManager.
type DB struct {
	pool *pgxpool.Pool
}

// Open connects to databaseURL. The schema must already be migrated because
// every connection registers the custom enum types on connect. Pool sizing
// uses pgx defaults and can be tuned in the URL (pool_max_conns, ...).
func Open(ctx context.Context, databaseURL string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		types, err := conn.LoadTypes(ctx, []string{"issue_status", "_issue_status"})
		if err != nil {
			return fmt.Errorf("load custom types (is the schema migrated?): %w", err)
		}

		conn.TypeMap().RegisterTypes(types)

		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close releases all connections.
func (d *DB) Close() { d.pool.Close() }

// Ping checks the database is reachable.
func (d *DB) Ping(ctx context.Context) error { return d.pool.Ping(ctx) }

type txKey struct{}

// WithinTx runs fn in a read-committed transaction; see port.TxManager.
// Nested calls join the outer transaction.
func (d *DB) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.inTx(ctx, pgx.TxOptions{}, fn)
}

// readTx runs fn in a repeatable-read, read-only transaction so several
// queries (e.g. a page and its total) see one snapshot.
func (d *DB) readTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return d.inTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, fn)
}

func (d *DB) inTx(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := d.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		// No-op after a successful commit. Use a context that survives a
		// cancelled request so the connection is returned cleanly.
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// inTx reports whether ctx carries a transaction.
func inTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(pgx.Tx)

	return ok
}

// queries returns sqlc queries bound to the transaction in ctx, if any, or
// to the pool.
func (d *DB) queries(ctx context.Context) *sqlc.Queries {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return sqlc.New(tx)
	}

	return sqlc.New(d.pool)
}

// isUniqueViolation reports whether err violates the named unique constraint.
func isUniqueViolation(err error, constraint string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// pageLimit returns the query limit for a page: one extra row tells whether
// another page follows. Page sizes are bounded by domain.MaxPageSize.
// cursorField names the page cursor in validation errors.
const cursorField = "cursor"

func pageLimit(limit int) int32 {
	return int32(min(limit, 1<<20) + 1) //nolint:gosec // bounded above, cannot overflow
}
