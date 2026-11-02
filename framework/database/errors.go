/*
|--------------------------------------------------------------------------
| Database Errors
|--------------------------------------------------------------------------
|
| Callers decide what to do about a failed query by meaning, not by driver
| internals: a missing row, a duplicate, a conflict worth retrying. pgx
| errors are translated here, so nothing above this package names pgx or
| knows a SQLSTATE code.
|
| A write that reaches a read-only server means a failover happened and a
| pooled connection still points at the old primary. The whole pool is
| reset so the next attempt finds the new one.
|
*/

package database

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

/*
Errors the database layer returns.

ErrNotFound means a single-row query found no row. ErrStandby stops the
boot when the configured URL reaches a server in recovery. ErrReadOnly
means a write reached a read-only server after a failover, or a write
was sent through Reader. ErrNoSessionURL means session-scoped work was
asked for behind a transaction-mode pooler without a direct URL.
ErrNotifyInTransaction refuses a notification sent from inside a
transaction, where listeners could act before the data is committed.
*/
var (
	ErrNotFound            = errors.New("database: no rows")
	ErrStandby             = errors.New("database: url points at a standby; use the primary or writer endpoint")
	ErrReadOnly            = errors.New("database: server is read-only")
	ErrNoSessionURL        = errors.New("database: direct_url is required behind a transaction pooler")
	ErrNotifyInTransaction = errors.New("database: notify inside a transaction; use AfterCommit")
)

/*
SQLSTATE codes the layer translates.
*/
const (
	codeUniqueViolation      = "23505"
	codeReadOnlyTransaction  = "25006"
	codeSerializationFailure = "40001"
)

/*
IsUniqueViolation reports whether the error is a unique violation of the
named constraint. An empty name matches any unique constraint.
*/
func IsUniqueViolation(err error, constraint string) bool {
	pgErr, ok := asPgError(err)
	if !ok || pgErr.Code != codeUniqueViolation {
		return false
	}

	return constraint == "" || pgErr.ConstraintName == constraint
}

/*
IsSerialization reports whether the error is a serialization failure,
which the caller may retry by running the whole transaction again.
*/
func IsSerialization(err error) bool {
	pgErr, ok := asPgError(err)

	return ok && pgErr.Code == codeSerializationFailure
}

func asPgError(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	ok := errors.As(err, &pgErr)

	return pgErr, ok
}

/*
translate maps a driver error to the package's errors without side
effects, for reads that are read-only on purpose.
*/
func translate(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	if pgErr, ok := asPgError(err); ok && pgErr.Code == codeReadOnlyTransaction {
		return fmt.Errorf("%w: %w", ErrReadOnly, err)
	}

	return err
}

/*
mapError translates the error and resets the pool when a write reached a
read-only server, so the next attempt finds the new primary.
*/
func (db *DB) mapError(err error) error {
	err = translate(err)
	if errors.Is(err, ErrReadOnly) {
		db.pool.Reset()
	}

	return err
}

/*
mappedRow translates the error a row's Scan returns.
*/
type mappedRow struct {
	row pgx.Row
	db  *DB
}

/*
Scan reads the row into the destinations.
*/
func (r mappedRow) Scan(dest ...any) error {
	return r.db.mapError(r.row.Scan(dest...))
}
