/*
|--------------------------------------------------------------------------
| Database Pool
|--------------------------------------------------------------------------
|
| Queries are written in SQL and turned into typed Go functions by sqlc;
| this pool is what those functions run on. A query joins the transaction
| carried by its context, so code written against the pool takes part in
| whatever transaction its caller opened, without passing it around.
|
| Data that belongs to a tenant is reachable only inside a transaction,
| because that is where the tenant is set for row-level security. Outside
| one, tenant tables simply look empty: the safe way to fail.
|
*/

package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
ErrInvalidPoolMode is returned by Open for an unknown pool mode.
*/
var ErrInvalidPoolMode = errors.New("database: invalid pool_mode")

/*
Querier runs statements. The pool and transactions implement it, and so
does the DBTX interface sqlc generates, so generated queries accept both.
*/
type Querier interface {
	/*
		Exec runs a statement that returns no rows.
	*/
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)

	/*
		Query runs a statement and returns its rows; the caller closes them.
	*/
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)

	/*
		QueryRow runs a statement that returns at most one row.
	*/
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var (
	_ Querier = (*DB)(nil)
	_ Querier = (*Tx)(nil)
)

/*
DB is a pool of connections to one primary, plus a pool for each read
replica when replicas are configured.
*/
type DB struct {
	pool          *pgxpool.Pool
	replicas      *replicaSet
	tenantSetting string
}

/*
Open connects to the primary at the configured URL.

It refuses a server in recovery, so a reader endpoint configured by
mistake is caught at boot instead of on the first write. Behind a
transaction-mode pooler, statements run without named prepared
statements, which would not survive a change of server connection.

Replica pools connect lazily and start unhealthy, so a replica that is
down never stops the boot; reads use the primary until a lag check
passes.
*/
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if !cfg.PoolMode.Valid() {
		return nil, fmt.Errorf("%w %q", ErrInvalidPoolMode, cfg.PoolMode)
	}

	pool, err := newPool(ctx, cfg.URL, cfg)
	if err != nil {
		return nil, err
	}

	db := &DB{pool: pool, tenantSetting: cfg.TenantSetting}
	if err := db.ensurePrimary(ctx); err != nil {
		pool.Close()

		return nil, err
	}

	db.replicas, err = openReplicas(ctx, cfg, clock.System())
	if err != nil {
		pool.Close()

		return nil, err
	}

	return db, nil
}

/*
Close stops the replica checks, waits for borrowed connections to return
and closes every pool.
*/
func (db *DB) Close() {
	db.replicas.close()
	db.pool.Close()
}

/*
Ping checks that a connection can be borrowed and used.
*/
func (db *DB) Ping(ctx context.Context) error {
	if err := db.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}

/*
Exec runs a statement in the context's transaction, or on its own. It
counts as a write for Reader.
*/
func (db *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := db.txFrom(ctx); ok {
		return tx.Exec(ctx, sql, args...)
	}

	markWrite(ctx)
	tag, err := db.pool.Exec(ctx, sql, args...)

	return tag, db.mapError(err)
}

/*
Query runs a statement in the context's transaction, or on its own.
*/
func (db *DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, ok := db.txFrom(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}

	rows, err := db.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, db.mapError(err)
	}

	return rows, nil
}

/*
QueryRow runs a statement in the context's transaction, or on its own.
Scan returns ErrNotFound when there is no row.
*/
func (db *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := db.txFrom(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}

	return mappedRow{row: db.pool.QueryRow(ctx, sql, args...), db: db}
}

func (db *DB) ensurePrimary(ctx context.Context) error {
	var inRecovery bool
	if err := db.pool.QueryRow(ctx, "SELECT pg_is_in_recovery()").Scan(&inRecovery); err != nil {
		return fmt.Errorf("check primary: %w", err)
	}

	if inRecovery {
		return ErrStandby
	}

	return nil
}

/*
newPool builds a lazily connecting pool for the URL with the pool size
and pool mode from the config.
*/
func newPool(ctx context.Context, url string, cfg Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.ConnConfig.Tracer = cfg.Tracer

	if cfg.PoolMode == PoolModeTransaction {
		poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	return pool, nil
}
