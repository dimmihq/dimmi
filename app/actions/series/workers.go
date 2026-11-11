/*
|--------------------------------------------------------------------------
| Series Workers
|--------------------------------------------------------------------------
|
| The queue's way into the engine: one worker per job, each a thin call
| to the action that does the work.
|
*/

package series

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
)

/*
AdvanceWorker moves journeys on.
*/
type AdvanceWorker struct {
	Series *Series
}

/*
Handle moves one journey on.
*/
func (w *AdvanceWorker) Handle(ctx context.Context, job *queue.Job[jobs.AdvanceSeriesEnrollmentArgs]) error {
	return w.Series.Advance(ctx, job.Args)
}

/*
CheckWorker checks changed contacts against live series.
*/
type CheckWorker struct {
	Series *Series
}

/*
Handle checks one contact.
*/
func (w *CheckWorker) Handle(ctx context.Context, job *queue.Job[jobs.CheckSeriesSubjectArgs]) error {
	return w.Series.CheckSubject(ctx, job.Args)
}

/*
EventWorker offers tracked events to the series they start.
*/
type EventWorker struct {
	Series *Series
}

/*
Handle offers one event.
*/
func (w *EventWorker) Handle(ctx context.Context, job *queue.Job[jobs.SeriesEventArgs]) error {
	return w.Series.OnEvent(ctx, job.Args)
}
