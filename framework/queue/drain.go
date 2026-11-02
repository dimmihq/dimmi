/*
|--------------------------------------------------------------------------
| Draining The Queue
|--------------------------------------------------------------------------
|
| Tests want queued work done before they look at its results, without
| guessing how long to wait. Draining runs the real workers on the real
| job rows until nothing is ready or running, including jobs that the
| drained jobs pushed themselves, and a job the database sees as due a
| moment from now because its clock lags the application's.
|
*/

package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
drainCheckInterval is how often Drain looks whether work is left.
*/
const drainCheckInterval = 10 * time.Millisecond

/*
busySQL counts the jobs in the configured queues that are running or
ready to run now. A job is stamped with the clock of the process that
pushed it and fetched by the database's, and the two drift apart, so a
job pushed just now may look due a moment from now to the database. A
job never tried that is due within the skew window is counted as
ready, and Drain waits for the workers to pick it up instead of
returning early. A retry is left out of the window: it is due a backoff
from now on purpose, as close as a second, and waits for later like a
scheduled job.
*/
const busySQL = `SELECT count(*) FROM river_job
WHERE queue = ANY($1) AND (state = 'running' OR state = 'available' AND (scheduled_at <= now()
  OR attempt = 0 AND scheduled_at <= now() + interval '1 second'))`

/*
Drain works every job that is ready in the configured queues until none
is ready or running. Jobs that wait for a later time, such as scheduled,
postponed and failed ones, are left alone. When the workers are not
running, Drain starts them for its duration and stops them before it
returns.
*/
func (q *Queue) Drain(ctx context.Context) (err error) {
	if !q.running.Load() {
		if err := q.Start(ctx); err != nil {
			return err
		}

		defer func() { err = errors.Join(err, q.Stop(ctx)) }()
	}

	for {
		var busy int
		if err := q.pool.QueryRow(ctx, busySQL, q.queues).Scan(&busy); err != nil {
			return fmt.Errorf("drain queue: %w", err)
		}

		if busy == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("drain queue: %w", ctx.Err())
		case <-clock.System().After(drainCheckInterval):
		}
	}
}
