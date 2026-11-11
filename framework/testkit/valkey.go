/*
|--------------------------------------------------------------------------
| Test Valkey
|--------------------------------------------------------------------------
|
| Tests share one Valkey from REDIS_URL. Each test gets a key prefix no
| other test uses, so parallel tests never see each other's cache entries,
| counters or locks, and nothing needs flushing afterwards: every key the
| framework writes expires on its own.
|
*/

package testkit

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/redis"
)

/*
ValkeyConfig returns a client config for the test: the shared server and
a key prefix of its own.
*/
func ValkeyConfig(tb testing.TB) redis.Config {
	tb.Helper()

	return redis.Config{
		URL:       Env(tb, "REDIS_URL"),
		Mode:      redis.ModeStandalone,
		KeyPrefix: UniqueName("t") + ":",
	}
}

/*
NewValkey returns a client with the test's own key prefix, closed when
the test ends.
*/
func NewValkey(tb testing.TB) *redis.Client {
	tb.Helper()

	client, err := redis.Open(tb.Context(), ValkeyConfig(tb))
	if err != nil {
		tb.Fatalf("open valkey: %v", err)
	}

	Cleanup(tb, client.Close)

	return client
}
