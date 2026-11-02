package cache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
Test TTLs. Short ones leave room for two calls in a row even on a loaded
CI machine running with -race.
*/
const (
	long  = time.Minute
	short = 250 * time.Millisecond
)

func newStore(t *testing.T) *cache.Store {
	t.Helper()

	return openStore(t, testkit.NewValkey(t), "cache")
}

func openStore(t *testing.T, client *redis.Client, keyspace string) *cache.Store {
	t.Helper()

	store, err := cache.Open(client, cache.Options{Keyspace: keyspace})
	require.NoError(t, err)
	t.Cleanup(store.Close)

	return store
}

func waitPastShortTTL() {
	clock.System().Sleep(2 * short)
}
