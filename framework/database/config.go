/*
|--------------------------------------------------------------------------
| Database Configuration
|--------------------------------------------------------------------------
|
| The application always talks to one primary. Operators may put a
| connection pooler in front of it; a transaction-mode pooler can hand
| each transaction a different server connection, so the pool must avoid
| anything that outlives a transaction, such as named prepared statements.
|
| Work that needs a server session to itself, such as a session lock or
| LISTEN, cannot go through such a pooler. It uses a direct connection to
| the primary instead, or is refused when there is none.
|
| Read replicas are optional. Reads that tolerate a few seconds of delay
| may go there; everything else stays on the primary.
|
*/

package database

import (
	"time"

	"github.com/jackc/pgx/v5"
)

/*
PoolMode says what sits between the application and PostgreSQL: a direct
connection or session-mode pooler, or a transaction-mode pooler such as
PgBouncer.
*/
type PoolMode string

/*
The supported pool modes, described on PoolMode.
*/
const (
	PoolModeSession     PoolMode = "session"
	PoolModeTransaction PoolMode = "transaction"
)

/*
Valid reports whether the pool mode is one we know.
*/
func (m PoolMode) Valid() bool {
	switch m {
	case PoolModeSession, PoolModeTransaction:
		return true
	}

	return false
}

/*
Config configures the pool.

URL must reach a primary; multi-host URLs with
target_session_attrs=read-write are accepted. TenantSetting names the
setting each transaction writes the tenant id to, which row-level
security policies read back with current_setting.

DirectURL reaches the primary without a transaction-mode pooler, for
session-scoped work; see SessionURL. ReadURLs lists read replicas for
Reader, and ReadMaxLag is how far behind the primary a replica may fall
before reads leave it. A zero ReadMaxLag accepts only replicas that are
fully caught up.

Tracer, set in code, observes every query on every pool, so query spans
join the trace of the request that ran them.
*/
type Config struct {
	URL           string          `koanf:"url"`
	DirectURL     string          `koanf:"direct_url"`
	ReadURLs      []string        `koanf:"read_urls"`
	ReadMaxLag    time.Duration   `default:"10s"           koanf:"read_max_lag"`
	MaxConns      int32           `default:"10"            koanf:"max_conns"`
	PoolMode      PoolMode        `default:"session"       koanf:"pool_mode"`
	TenantSetting string          `default:"app.tenant_id" koanf:"tenant_setting"`
	Tracer        pgx.QueryTracer `koanf:"-"`
}

/*
SessionURL returns the URL for work that needs a server session of its
own: session advisory locks, LISTEN, migrations and dumps.

It is DirectURL when set. Otherwise it is URL, unless URL goes through a
transaction-mode pooler, where a session can change server connection
between statements; then it returns ErrNoSessionURL.
*/
func (c Config) SessionURL() (string, error) {
	if c.DirectURL != "" {
		return c.DirectURL, nil
	}

	if c.PoolMode == PoolModeTransaction {
		return "", ErrNoSessionURL
	}

	return c.URL, nil
}
