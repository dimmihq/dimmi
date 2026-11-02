/*
|--------------------------------------------------------------------------
| Pushing Inside A Transaction
|--------------------------------------------------------------------------
|
| River inserts a job in a caller's transaction when handed a pgx.Tx. The
| application's transaction is a database.Tx, which keeps its pgx.Tx to
| itself, so this adapter hands River the statements it needs to insert
| a job and notify the workers, run through the database.Tx.
|
| River also asks how the connection runs statements, so it can encode
| JSON arguments that a transaction pooler's mode would otherwise send
| as bytes. The adapter answers with the application pool's mode.
|
*/

package queue

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
riverTx is the pgx.Tx River sees. Inserting a job only runs statements,
so Exec, Query and QueryRow are all it provides; the embedded pgx.Tx is
nil and any other method River might call would fail loudly.
*/
type riverTx struct {
	pgx.Tx

	tx   *database.Tx
	mode pgx.QueryExecMode
}

/*
Exec runs a statement in the transaction.
*/
func (t riverTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, args...)
}

/*
Query runs a statement in the transaction.
*/
func (t riverTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

/*
QueryRow runs a single-row statement in the transaction.
*/
func (t riverTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

/*
Config tells River how the application's connections run statements.
*/
func (t riverTx) Config() *pgxpool.Config {
	return &pgxpool.Config{ConnConfig: &pgx.ConnConfig{DefaultQueryExecMode: t.mode}}
}
