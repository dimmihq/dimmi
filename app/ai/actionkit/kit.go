/*
|--------------------------------------------------------------------------
| Action Kit
|--------------------------------------------------------------------------
|
| The services every part of an action's life shares: the database the
| call rows live in, the clock deadlines are measured on, the guarded
| HTTP client every request leaves through, the keys secrets are sealed
| with, the events teammates hear and the queue that runs approved
| calls and wakes the agent afterwards.
|
*/

package actionkit

import (
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
Call limits: how long a customer has to confirm, and the default
timeout of one request.
*/
const (
	ConfirmationTTL = 15 * time.Minute
	defaultTimeout  = 8 * time.Second
)

/*
Kit runs actions. Queue is nil-safe only in code paths that never
need to schedule work, such as tests of definitions.
*/
type Kit struct {
	DB     *database.DB
	Clock  clock.Clock
	HTTP   *httpclient.Client
	Keys   *encryption.Keys
	Events *events.Dispatcher
	Queue  *queue.Queue
}

/*
timeoutFor is how long one request of the action may take: the
action's own limit, else the connection's.
*/
func timeoutFor(t Target) time.Duration {
	ms := t.Spec.TimeoutMS
	if ms == 0 {
		ms = t.Conn.TimeoutMs
	}
	if ms <= 0 {
		return defaultTimeout
	}
	return msDuration(ms)
}

/*
msDuration reads a count of milliseconds as a duration.
*/
func msDuration(ms int32) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
