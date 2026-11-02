/*
|--------------------------------------------------------------------------
| Locks
|--------------------------------------------------------------------------
|
| Locks stop two processes from doing the same expensive work at once:
| rebuilding a report, refreshing a token, syncing one integration. They
| are best effort. A lock lives in Valkey, which may restart, so work that
| must never run twice takes a database advisory lock instead.
|
| A held lock is extended in the background for as long as its context
| lives, so slow work never loses it halfway. Releasing it is a function
| call, and a crashed process loses it within seconds.
|
*/

package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/rueidis/rueidislock"
)

/*
ErrLocked is returned by TryLock when another process holds the lock.
*/
var ErrLocked = errors.New("cache: lock is held")

/*
Lock waits until it holds the named lock, or until the context ends.

It returns a context that stays alive while the lock is held and a
function that releases it. Run the protected work with the returned
context, so the work stops if the lock is ever lost.
*/
func (s *Store) Lock(ctx context.Context, name string) (context.Context, context.CancelFunc, error) {
	held, release, err := s.locker.WithContext(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("lock %q: %w", name, err)
	}

	return held, release, nil
}

/*
TryLock takes the named lock if it is free, or returns ErrLocked at once.
*/
func (s *Store) TryLock(ctx context.Context, name string) (context.Context, context.CancelFunc, error) {
	held, release, err := s.locker.TryWithContext(ctx, name)
	if errors.Is(err, rueidislock.ErrNotLocked) {
		return nil, nil, fmt.Errorf("lock %q: %w", name, ErrLocked)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("lock %q: %w", name, err)
	}

	return held, release, nil
}
