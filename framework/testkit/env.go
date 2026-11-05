/*
|--------------------------------------------------------------------------
| Test Environment
|--------------------------------------------------------------------------
|
| Tests talk to the same kinds of services production does: PostgreSQL,
| Valkey and S3. They find them through the same environment variables
| the application reads, set per environment: a .env.testing file locally,
| service containers in CI. A missing variable fails the test loudly
| instead of skipping it, so a green run always means the code ran
| against real services.
|
*/

package testkit

import (
	"crypto/rand"
	"os"
	"strings"
	"testing"
)

/*
Env returns the environment variable's value, failing the test when it
is not set.
*/
func Env(tb testing.TB, name string) string {
	tb.Helper()

	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		tb.Fatalf("%s is not set: start the services with `docker compose up -d` and run `make test`", name)
	}

	return value
}

/*
Switch reports whether the environment variable is set to 1, for runs
that change how every test behaves, such as connecting as another role.
*/
func Switch(name string) bool {
	return os.Getenv(name) == "1"
}

/*
UniqueName returns the prefix followed by ten random lowercase letters
and digits, safe as a database, role, bucket or key-prefix name.
*/
func UniqueName(prefix string) string {
	return prefix + strings.ToLower(rand.Text()[:10])
}
