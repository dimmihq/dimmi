/*
|--------------------------------------------------------------------------
| Background Series Workers
|--------------------------------------------------------------------------
|
| The workers of jobs that run apart from any one change: looking
| through a series' audience, making the calls steps wait on, and the
| periodic sweep.
|
*/

package series

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
)

/*
EnrollWorker looks for contacts who match a series.
*/
type EnrollWorker struct {
	Series *Series
}

/*
Handle looks through one series' audience.
*/
func (w *EnrollWorker) Handle(ctx context.Context, job *queue.Job[jobs.EnrollSeriesArgs]) error {
	return w.Series.EnrollAll(ctx, job.Args)
}

/*
CallWorker makes the calls steps wait on.
*/
type CallWorker struct {
	Series *Series
}

/*
Handle makes one call.
*/
func (w *CallWorker) Handle(ctx context.Context, job *queue.Job[jobs.CallSeriesStepArgs]) error {
	return w.Series.Call(ctx, job.Args)
}

/*
SweepWorker runs the periodic sweep.
*/
type SweepWorker struct {
	Series *Series
}

/*
Handle sweeps every workspace.
*/
func (w *SweepWorker) Handle(ctx context.Context, _ *queue.Job[jobs.SweepSeriesArgs]) error {
	return w.Series.Sweep(ctx)
}
