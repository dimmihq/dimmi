/*
|--------------------------------------------------------------------------
| Wake Expired Snoozes
|--------------------------------------------------------------------------
|
| Every minute the conversations whose snooze ran out open again.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
WakeExpiredSnoozesArgs names the job; it takes no arguments.
*/
type WakeExpiredSnoozesArgs struct{}

/*
Kind names the job.
*/
func (WakeExpiredSnoozesArgs) Kind() string {
	return "wake_expired_snoozes"
}

/*
Options keep one round at a time: a round still waiting or running
makes the next minute's push a no-op.
*/
func (WakeExpiredSnoozesArgs) Options() queue.Options {
	return queue.Options{Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
WakeExpiredSnoozes opens the conversations whose snooze ran out.
*/
type WakeExpiredSnoozes struct {
	Snoozes Sweeper
}

/*
Handle runs the sweep.
*/
func (w *WakeExpiredSnoozes) Handle(ctx context.Context, _ *queue.Job[WakeExpiredSnoozesArgs]) error {
	return w.Snoozes.Sweep(ctx)
}
