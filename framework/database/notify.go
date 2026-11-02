/*
|--------------------------------------------------------------------------
| Notifications
|--------------------------------------------------------------------------
|
| A notification tells listeners on other connections that something
| changed. It is sent on its own, never inside a transaction: a listener
| that reacts to it must find the data already committed. Code inside a
| transaction sends it from an after-commit hook.
|
*/

package database

import (
	"context"
	"fmt"
)

/*
Notify sends the payload to listeners on the channel. It returns
ErrNotifyInTransaction when the context carries a transaction.
*/
func (db *DB) Notify(ctx context.Context, channel, payload string) error {
	if _, ok := db.txFrom(ctx); ok {
		return ErrNotifyInTransaction
	}

	if _, err := db.pool.Exec(ctx, "SELECT pg_notify($1, $2)", channel, payload); err != nil {
		return fmt.Errorf("notify %s: %w", channel, db.mapError(err))
	}

	return nil
}
