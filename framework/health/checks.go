/*
|--------------------------------------------------------------------------
| Built-in Checks
|--------------------------------------------------------------------------
|
| Every installation depends on PostgreSQL and Valkey, so their checks
| ship with the framework. Each one makes the cheapest round trip the
| service offers: reaching it at all is what readiness needs to know.
|
*/

package health

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/redis"
)

/*
Database returns the check named "database" that pings PostgreSQL.
*/
func Database(db *database.DB) Check {
	return pingCheck{
		name: "database",
		ping: db.Ping,
		fix:  "check that PostgreSQL is running and the database url points at it",
	}
}

/*
Valkey returns the check named "valkey" that pings Valkey.
*/
func Valkey(client *redis.Client) Check {
	return pingCheck{
		name: "valkey",
		ping: client.Ping,
		fix:  "check that Valkey is running and the redis url points at it",
	}
}

type pingCheck struct {
	name string
	ping func(ctx context.Context) error
	fix  string
}

/*
Name returns the name of the service pinged.
*/
func (c pingCheck) Name() string {
	return c.name
}

/*
Run pings the service once and fails with the error and the fix when it
does not answer.
*/
func (c pingCheck) Run(ctx context.Context) Result {
	if err := c.ping(ctx); err != nil {
		return Result{Status: Fail, Detail: err.Error(), Fix: c.fix}
	}

	return Result{Status: Pass, Detail: "reachable"}
}
