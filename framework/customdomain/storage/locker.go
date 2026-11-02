/*
|--------------------------------------------------------------------------
| Certificate Locks
|--------------------------------------------------------------------------
|
| Two servers seeing the first handshake for a hostname at the same time
| must not both order its certificate. Each order runs under a named lock
| held as a PostgreSQL advisory lock, so it covers every server and is
| released by the database itself if the holder dies.
|
| Advisory locks here live as long as a transaction, so a held lock keeps
| one transaction, and one pooled connection, open until it is unlocked.
| Locks are held only while a certificate is ordered, which is rare and
| brief, so this costs little.
|
*/

package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
ErrNotLocked is returned by Unlock for a name this storage does not
hold.
*/
var ErrNotLocked = errors.New("storage: lock not held")

/*
heldLock is a lock this process holds. Closing release ends the
transaction holding it; finished then reports how it ended.
*/
type heldLock struct {
	release  chan struct{}
	finished chan error
}

/*
Lock waits until it holds the named lock, or the context ends.
*/
func (s *Storage) Lock(ctx context.Context, name string) error {
	lock := &heldLock{release: make(chan struct{}), finished: make(chan error, 1)}
	acquired := make(chan error, 1)

	go s.hold(ctx, name, lock, acquired)

	if err := <-acquired; err != nil {
		return fmt.Errorf("lock %s: %w", name, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.held[name] = lock

	return nil
}

/*
Unlock releases the named lock and waits until the database has let it
go.
*/
func (s *Storage) Unlock(_ context.Context, name string) error {
	s.mu.Lock()
	lock, ok := s.held[name]
	delete(s.held, name)
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("unlock %s: %w", name, ErrNotLocked)
	}

	close(lock.release)

	if err := <-lock.finished; err != nil {
		return fmt.Errorf("unlock %s: %w", name, err)
	}

	return nil
}

/*
hold takes the lock in a transaction and keeps the transaction open
until the lock is released. Waiting for the lock ends with the caller's
context; once held, the lock outlives it until Unlock.
*/
func (s *Storage) hold(ctx context.Context, name string, lock *heldLock, acquired chan<- error) {
	held := false

	err := s.db.Transaction(context.WithoutCancel(ctx), func(_ context.Context, tx *database.Tx) error {
		if err := tx.AdvisoryLock(ctx, "certmagic:"+name); err != nil {
			return err
		}

		held = true
		acquired <- nil

		<-lock.release

		return nil
	})

	if !held {
		acquired <- err

		return
	}

	lock.finished <- err
}
