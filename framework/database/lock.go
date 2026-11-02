/*
|--------------------------------------------------------------------------
| Advisory Locks
|--------------------------------------------------------------------------
|
| Some work must never run twice at once, even across servers: a nightly
| backup, a billing run, a migration. Advisory locks live in Postgres
| itself, so they hold as long as the database does. They are scoped to
| the transaction, released when it ends and never earlier, which keeps
| them safe behind a transaction-mode pooler.
|
*/

package database

import (
	"context"
	"fmt"
)

/*
AdvisoryLock waits for the advisory lock with the given name.
*/
func (tx *Tx) AdvisoryLock(ctx context.Context, key string) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", key); err != nil {
		return fmt.Errorf("advisory lock %q: %w", key, err)
	}

	return nil
}

/*
TryAdvisoryLock takes the advisory lock with the given name if it is free
and reports whether it did.
*/
func (tx *Tx) TryAdvisoryLock(ctx context.Context, key string) (bool, error) {
	var locked bool

	err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))", key).Scan(&locked)
	if err != nil {
		return false, fmt.Errorf("try advisory lock %q: %w", key, err)
	}

	return locked, nil
}
