/*
|--------------------------------------------------------------------------
| Transactions
|--------------------------------------------------------------------------
|
| A business action runs in one transaction: everything it writes commits
| together or not at all. Actions call other actions, so a transaction
| opened inside another becomes a savepoint, and a failing inner step can
| be undone without losing the outer work.
|
| Side effects that must only happen for committed data, such as events,
| mail and jobs, are registered as after-commit hooks. They run once the
| outermost transaction commits, and are dropped if it rolls back, so no
| listener ever reacts to a row that does not exist.
|
| The tenant in the context is set at the start of every transaction with
| SET LOCAL semantics. It ends with the transaction, so a pooler can never
| leak it to another client.
|
*/

package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
Hook runs after a transaction ends. Its context carries no transaction,
so any query it makes runs on its own.
*/
type Hook func(ctx context.Context)

/*
Tx is one transaction level: the outermost transaction or a savepoint.
*/
type Tx struct {
	db *DB
	tx pgx.Tx

	afterCommit   []Hook
	afterRollback []Hook
}

type txKey struct{}

/*
Transaction runs the function in a transaction. It commits when the
function returns nil and rolls back when it returns an error or panics.
Inside another transaction on the same pool it runs in a savepoint.

The context the function receives carries the transaction, so every
query made with it joins the transaction. A transaction counts as a
write for Reader.
*/
func (db *DB) Transaction(ctx context.Context, fn func(ctx context.Context, tx *Tx) error) error {
	if parent, ok := db.txFrom(ctx); ok {
		return parent.savepoint(ctx, fn)
	}

	markWrite(ctx)

	pgxTx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", db.mapError(err))
	}

	tx := &Tx{db: db, tx: pgxTx}
	if err := db.applyTenant(ctx, pgxTx); err != nil {
		tx.rollback(ctx)

		return err
	}

	if err := tx.run(ctx, fn); err != nil {
		return err
	}

	if err := pgxTx.Commit(ctx); err != nil {
		tx.runHooks(ctx, tx.afterRollback)

		return fmt.Errorf("commit transaction: %w", db.mapError(err))
	}

	tx.runHooks(ctx, tx.afterCommit)

	return nil
}

/*
Exec runs a statement in this transaction.
*/
func (tx *Tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.tx.Exec(ctx, sql, args...)

	return tag, tx.db.mapError(err)
}

/*
Query runs a statement in this transaction.
*/
func (tx *Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, tx.db.mapError(err)
	}

	return rows, nil
}

/*
QueryRow runs a statement in this transaction. Scan returns ErrNotFound
when there is no row.
*/
func (tx *Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return mappedRow{row: tx.tx.QueryRow(ctx, sql, args...), db: tx.db}
}

/*
AfterCommit registers a hook to run once the outermost transaction
commits. If this level is a savepoint that rolls back, the hook is
dropped.
*/
func (tx *Tx) AfterCommit(fn Hook) {
	tx.afterCommit = append(tx.afterCommit, fn)
}

/*
AfterRollback registers a hook to run if this level rolls back, or if
the outermost transaction does.
*/
func (tx *Tx) AfterRollback(fn Hook) {
	tx.afterRollback = append(tx.afterRollback, fn)
}

/*
run calls the function and rolls back when it fails or panics, then lets
the panic continue.
*/
func (tx *Tx) run(ctx context.Context, fn func(ctx context.Context, tx *Tx) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			tx.rollback(ctx)
			panic(r)
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, tx), tx); err != nil {
		tx.rollback(ctx)

		return err
	}

	return nil
}

/*
Transaction runs the function in a savepoint of this transaction, for
code that holds the transaction but not the database, such as a hook
that runs inside another's transaction. A failure undoes only the
savepoint's work and leaves this transaction usable, so a caller can
try an insert that may hit a unique constraint and carry on.
*/
func (tx *Tx) Transaction(ctx context.Context, fn func(ctx context.Context, tx *Tx) error) error {
	return tx.savepoint(ctx, fn)
}

/*
savepoint runs the function in a savepoint. Once released, the
savepoint's hooks belong to the parent: its work now stands or falls
with the parent's.
*/
func (tx *Tx) savepoint(ctx context.Context, fn func(ctx context.Context, tx *Tx) error) error {
	pgxTx, err := tx.tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create savepoint: %w", tx.db.mapError(err))
	}

	child := &Tx{db: tx.db, tx: pgxTx}
	if err := child.run(ctx, fn); err != nil {
		return err
	}

	if err := pgxTx.Commit(ctx); err != nil {
		return fmt.Errorf("release savepoint: %w", tx.db.mapError(err))
	}

	tx.afterCommit = append(tx.afterCommit, child.afterCommit...)
	tx.afterRollback = append(tx.afterRollback, child.afterRollback...)

	return nil
}

/*
rollback undoes this level and runs its rollback hooks. A failed rollback
is ignored: the transaction is already failing, and Postgres discards it
when the connection returns to the pool either way.
*/
func (tx *Tx) rollback(ctx context.Context) {
	ctx = context.WithoutCancel(ctx)
	_ = tx.tx.Rollback(ctx)
	tx.runHooks(ctx, tx.afterRollback)
}

func (tx *Tx) runHooks(ctx context.Context, hooks []Hook) {
	ctx = context.WithoutCancel(ctx)
	for _, hook := range hooks {
		hook(ctx)
	}
}

func (db *DB) txFrom(ctx context.Context) (*Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*Tx)

	return tx, ok && tx.db == db
}

/*
applyTenant scopes the transaction to the tenant in the context. Calling
set_config with is_local true is SET LOCAL with a bind parameter.
*/
func (db *DB) applyTenant(ctx context.Context, tx pgx.Tx) error {
	tenant, ok := tenancy.From(ctx)
	if !ok || db.tenantSetting == "" {
		return nil
	}

	if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", db.tenantSetting, tenant); err != nil {
		return fmt.Errorf("set tenant: %w", db.mapError(err))
	}

	return nil
}
