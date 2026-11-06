/*
|--------------------------------------------------------------------------
| Erase Contact
|--------------------------------------------------------------------------
|
| Carries out a request to erase a customer once its grace period ends.
| A request a retention hold keeps is asked about again a day later, and
| one run early waits until it is due. Any failure, including one of the
| registered erasers, retries the whole run.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/erasures"
)

/*
erasureHoldRetry is how long a held request waits before the guard is
asked again.
*/
const erasureHoldRetry = 24 * time.Hour

/*
EraseContact runs due erasures.
*/
type EraseContact struct {
	Erase *erasures.EraseContact
}

/*
Handle runs the erasure, or puts it off while it is held or not yet due.
*/
func (w *EraseContact) Handle(ctx context.Context, job *queue.Job[erasures.EraseArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())

	outcome, err := w.Erase.Handle(ctx, job.Args)
	if err != nil {
		return err
	}

	switch outcome {
	case erasures.Held:
		return queue.Postpone(erasureHoldRetry)
	case erasures.Waiting:
		return queue.Postpone(time.Hour)
	case erasures.Done:
	}

	return nil
}
