/*
|--------------------------------------------------------------------------
| Cache Store
|--------------------------------------------------------------------------
|
| The cache only ever saves work. Every value expires on its own, so the
| store can be flushed or lost at any moment and the application simply
| computes again. A store lives in its own keyspace under the client's
| prefix, which lets several stores share one Valkey without ever reading
| or flushing each other's keys.
|
| Every command touches a single key, so the store behaves the same on a
| single server and on a cluster.
|
*/

package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidisaside"
	"github.com/redis/rueidis/rueidislock"

	"github.com/dimmihq/dimmi/framework/redis"
)

/*
Errors a store returns.

ErrMiss means the key is absent or expired. ErrTTLRequired guards the
rule that nothing in the cache lives forever. ErrNotInteger comes from
Increment on a key holding something other than a number.
*/
var (
	ErrMiss        = errors.New("cache: miss")
	ErrTTLRequired = errors.New("cache: ttl must be positive")
	ErrNotInteger  = errors.New("cache: value is not an integer")
)

/*
incrementScript adds to a counter and sets its expiry only when the
counter is new, in one round trip, so no crash can leave a counter that
never expires.
*/
const incrementScript = `
local value = redis.call('INCRBY', KEYS[1], ARGV[1])
if redis.call('PTTL', KEYS[1]) < 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return value`

/*
Options configure a store. The keyspace keeps the store's keys apart
from every other store and service on the same server.
*/
type Options struct {
	Keyspace string `default:"cache" koanf:"keyspace"`
}

/*
Store is a cache on Valkey.

Plain reads and writes go through the shared client. Remember and the
locks use dedicated clients built by rueidis, because both rely on
server-assisted client-side caching to wait without polling.
*/
type Store struct {
	client    *redis.Client
	keyspace  string
	aside     rueidisaside.CacheAsideClient
	locker    rueidislock.Locker
	increment *rueidis.Lua
}

/*
Open returns the store for a keyspace on the given client. Close it when
done to release its dedicated connections.
*/
func Open(client *redis.Client, opts Options) (*Store, error) {
	aside, err := rueidisaside.NewClient(rueidisaside.ClientOption{ClientOption: client.Option()})
	if err != nil {
		return nil, fmt.Errorf("open cache-aside client: %w", err)
	}

	locker, err := rueidislock.NewLocker(rueidislock.LockerOption{
		ClientOption: client.Option(),
		KeyPrefix:    client.Prefix(opts.Keyspace) + "lock",
	})
	if err != nil {
		aside.Close()

		return nil, fmt.Errorf("open lock client: %w", err)
	}

	return &Store{
		client:    client,
		keyspace:  opts.Keyspace,
		aside:     aside,
		locker:    locker,
		increment: rueidis.NewLuaScript(incrementScript),
	}, nil
}

/*
Close releases the store's dedicated connections. The shared client
stays open.
*/
func (s *Store) Close() {
	s.aside.Close()
	s.locker.Close()
}

/*
Get returns the value at the key, or ErrMiss.
*/
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	rdb := s.client.Rueidis()

	value, err := rdb.Do(ctx, rdb.B().Get().Key(s.key(key)).Build()).AsBytes()
	if rueidis.IsRedisNil(err) {
		return nil, ErrMiss
	}

	if err != nil {
		return nil, fmt.Errorf("get %q: %w", key, err)
	}

	return value, nil
}

/*
Many returns the values present at the keys; absent keys are left out.

The keys do not share a tag, so rueidis splits the read by cluster slot.
*/
func (s *Store) Many(ctx context.Context, keys []string) (map[string][]byte, error) {
	found := make(map[string][]byte, len(keys))
	if len(keys) == 0 {
		return found, nil
	}

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = s.key(key)
	}

	replies, err := rueidis.MGet(s.client.Rueidis(), ctx, fullKeys)
	if err != nil {
		return nil, fmt.Errorf("get many: %w", err)
	}

	for i, key := range keys {
		reply := replies[fullKeys[i]]
		if value, err := reply.AsBytes(); err == nil {
			found[key] = value
		}
	}

	return found, nil
}

/*
Set stores the value at the key for the TTL, replacing what was there.
*/
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return ErrTTLRequired
	}

	rdb := s.client.Rueidis()
	cmd := rdb.B().Set().Key(s.key(key)).Value(rueidis.BinaryString(value)).Px(ttl).Build()

	if err := rdb.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("set %q: %w", key, err)
	}

	return nil
}

/*
Add stores the value only if the key is absent and reports whether it
did. Replay and dedupe windows are built on it: the first caller wins and
every repeat inside the TTL is turned away.
*/
func (s *Store) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		return false, ErrTTLRequired
	}

	rdb := s.client.Rueidis()
	cmd := rdb.B().Set().Key(s.key(key)).Value(rueidis.BinaryString(value)).Nx().Px(ttl).Build()

	err := rdb.Do(ctx, cmd).Error()
	if rueidis.IsRedisNil(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("add %q: %w", key, err)
	}

	return true, nil
}

/*
Increment adds to the integer at the key and returns the new total.

A new counter starts at zero and expires after the TTL. Later increments
keep that first expiry, which is what fixed-window rate limits need.
*/
func (s *Store) Increment(ctx context.Context, key string, by int64, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, ErrTTLRequired
	}

	args := []string{strconv.FormatInt(by, 10), strconv.FormatInt(ttl.Milliseconds(), 10)}

	total, err := s.increment.Exec(ctx, s.client.Rueidis(), []string{s.key(key)}, args).AsInt64()
	if err != nil && strings.Contains(err.Error(), "not an integer") {
		return 0, ErrNotInteger
	}

	if err != nil {
		return 0, fmt.Errorf("increment %q: %w", key, err)
	}

	return total, nil
}

/*
Delete removes the key. Deleting an absent key is not an error.
*/
func (s *Store) Delete(ctx context.Context, key string) error {
	rdb := s.client.Rueidis()
	if err := rdb.Do(ctx, rdb.B().Unlink().Key(s.key(key)).Build()).Error(); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}

	return nil
}

func (s *Store) key(key string) string {
	return s.client.Key(s.keyspace, key)
}
