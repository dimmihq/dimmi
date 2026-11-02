/*
|--------------------------------------------------------------------------
| Reader
|--------------------------------------------------------------------------
|
| Call sites that can live with slightly old data ask for a reader instead
| of the pool. Everything else reads the primary, so no one sees stale
| data by accident.
|
| A request that wrote must read its own writes, so once anything in a
| tracked context wrote, its reads stay on the primary. Inside a
| transaction the reader is the transaction itself.
|
| Every reader statement runs in a read-only transaction that sets the
| tenant first, so row-level security applies on replicas exactly as it
| does on the primary.
|
*/

package database

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type writeKey struct{}

/*
TrackWrites returns a context that remembers whether anything run with
it wrote. Requests start from one, so a request that wrote reads its own
writes from the primary. Exec outside a transaction and Transaction
count as writes; Query and QueryRow outside a transaction do not.
*/
func TrackWrites(ctx context.Context) context.Context {
	return context.WithValue(ctx, writeKey{}, new(atomic.Bool))
}

func markWrite(ctx context.Context) {
	if wrote, ok := ctx.Value(writeKey{}).(*atomic.Bool); ok {
		wrote.Store(true)
	}
}

func wroteIn(ctx context.Context) bool {
	wrote, ok := ctx.Value(writeKey{}).(*atomic.Bool)

	return ok && wrote.Load()
}

/*
Reader returns a Querier for reads that may lag the primary by up to the
configured maximum.

It reads a healthy replica when one is configured. It reads the primary
when no replica is configured or healthy, or when the context already
wrote. Inside a transaction it returns that transaction. If a replica
cannot start a transaction, the read moves to the primary at once.
*/
func (db *DB) Reader(ctx context.Context) Querier {
	if tx, ok := db.txFrom(ctx); ok {
		return tx
	}

	if wroteIn(ctx) {
		return &reader{db: db, pool: db.pool}
	}

	r := db.replicas.pick()
	if r == nil {
		return &reader{db: db, pool: db.pool}
	}

	return &reader{db: db, pool: r.pool, replica: r}
}

/*
reader runs each statement in its own read-only transaction on the pool.
The replica field is set when the pool belongs to a replica.
*/
type reader struct {
	db      *DB
	pool    *pgxpool.Pool
	replica *replica
}

/*
Exec runs a statement in a read-only transaction.
*/
func (r *reader) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	defer endRead(ctx, tx)

	tag, err := tx.Exec(ctx, sql, args...)

	return tag, translate(err)
}

/*
Query runs a statement in a read-only transaction that ends when the
rows are closed.
*/
func (r *reader) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		endRead(ctx, tx)

		return nil, translate(err)
	}

	return &readRows{Rows: rows, end: func() { endRead(ctx, tx) }}, nil
}

/*
QueryRow runs a statement in a read-only transaction when the row is
scanned. Scan returns ErrNotFound when there is no row.
*/
func (r *reader) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return scanFunc(func(dest ...any) error {
		tx, err := r.begin(ctx)
		if err != nil {
			return err
		}
		defer endRead(ctx, tx)

		return translate(tx.QueryRow(ctx, sql, args...).Scan(dest...))
	})
}

/*
begin opens the read-only transaction and sets the tenant. A replica
that fails to start one is marked unhealthy until its next check, and
the read moves to the primary.
*/
func (r *reader) begin(ctx context.Context) (pgx.Tx, error) {
	options := pgx.TxOptions{AccessMode: pgx.ReadOnly}

	tx, err := r.pool.BeginTx(ctx, options)
	if err != nil && r.replica != nil && ctx.Err() == nil {
		r.replica.healthy.Store(false)
		tx, err = r.db.pool.BeginTx(ctx, options)
	}

	if err != nil {
		return nil, fmt.Errorf("begin read: %w", translate(err))
	}

	if err := r.db.applyTenant(ctx, tx); err != nil {
		endRead(ctx, tx)

		return nil, err
	}

	return tx, nil
}

/*
endRead ends a read-only transaction. Nothing was written, so a failure
to end it loses nothing, and the pool discards the connection either
way.
*/
func endRead(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(context.WithoutCancel(ctx))
}

/*
readRows ends the read transaction when the rows are closed.
*/
type readRows struct {
	pgx.Rows

	end func()
}

/*
Close closes the rows and ends their transaction.
*/
func (r *readRows) Close() {
	r.Rows.Close()
	r.end()
}

/*
scanFunc is a pgx.Row whose Scan runs the function.
*/
type scanFunc func(dest ...any) error

/*
Scan reads the row into the destinations.
*/
func (f scanFunc) Scan(dest ...any) error {
	return f(dest...)
}
