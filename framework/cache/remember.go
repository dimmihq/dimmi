/*
|--------------------------------------------------------------------------
| Remember
|--------------------------------------------------------------------------
|
| Most cache reads follow one pattern: return the cached value, or compute
| it, store it and return it. When a popular key expires, every process
| would otherwise recompute it at once. Remember lets one caller compute
| while the others wait for its result, and keeps hot values in process
| memory until Valkey says they changed.
|
| Keys filled by Remember belong to Remember: while a value is being
| computed the key holds a placeholder, so read them only through it.
|
*/

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

/*
Loader computes a value to cache.
*/
type Loader func(ctx context.Context) ([]byte, error)

/*
Remember returns the value cached at the key, or runs load, caches what
it returns for the TTL and returns that.

Only one caller across all processes runs load for a key at a time; the
others wait for its result.
*/
func (s *Store) Remember(ctx context.Context, key string, ttl time.Duration, load Loader) ([]byte, error) {
	if ttl <= 0 {
		return nil, ErrTTLRequired
	}

	value, err := s.aside.Get(ctx, ttl, s.key(key), func(ctx context.Context, _ string) (string, error) {
		loaded, err := load(ctx)

		return string(loaded), err
	})
	if err != nil {
		return nil, fmt.Errorf("remember %q: %w", key, err)
	}

	return []byte(value), nil
}

/*
Forget removes a value cached by Remember, so the next read loads it
again.
*/
func (s *Store) Forget(ctx context.Context, key string) error {
	if err := s.aside.Del(ctx, s.key(key)); err != nil {
		return fmt.Errorf("forget %q: %w", key, err)
	}

	return nil
}

/*
Typed reads and writes values of one Go type, stored as JSON, each for
the same TTL.

	users := cache.For[User](store, 10*time.Minute)
	user, err := users.Remember(ctx, "user:"+id, loadUser)
*/
type Typed[T any] struct {
	store *Store
	ttl   time.Duration
}

/*
For returns a typed view of the store whose values live for the TTL.
*/
func For[T any](store *Store, ttl time.Duration) Typed[T] {
	return Typed[T]{store: store, ttl: ttl}
}

/*
Remember returns the value cached at the key, or runs load, caches its
result and returns it.
*/
func (t Typed[T]) Remember(ctx context.Context, key string, load func(context.Context) (T, error)) (T, error) {
	var value T

	encoded, err := t.store.Remember(ctx, key, t.ttl, func(ctx context.Context) ([]byte, error) {
		loaded, err := load(ctx)
		if err != nil {
			return nil, err
		}

		return json.Marshal(loaded)
	})
	if err != nil {
		return value, err
	}

	if err := json.Unmarshal(encoded, &value); err != nil {
		return value, fmt.Errorf("decode cached %q: %w", key, err)
	}

	return value, nil
}
