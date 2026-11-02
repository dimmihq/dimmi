/*
|--------------------------------------------------------------------------
| Failing Closed
|--------------------------------------------------------------------------
|
| Sign-in and sign-up count attempts in Valkey. When Valkey cannot be
| reached nothing can be counted, so nothing is let through either: the
| client is told the server is busy and to come back shortly, and the
| cause is logged here, the one place that handles it, since the client
| never sees it.
|
*/

package auth

import (
	"context"

	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
unavailableRetry is how many seconds a client waits before trying again
when the counters are out of reach.
*/
const unavailableRetry = 5

/*
Unavailable logs why attempts could not be counted and returns the busy
answer.
*/
func Unavailable(ctx context.Context, cause error) error {
	log.From(ctx).WarnContext(ctx, "sign-in counters unavailable", log.Err(cause))

	return exceptions.ErrAuthBusy.With("retry_after", unavailableRetry)
}
