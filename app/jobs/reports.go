/*
|--------------------------------------------------------------------------
| Report Jobs
|--------------------------------------------------------------------------
|
| Reports stay fresh within seconds: one refresh runs at a time, every
| three seconds, working out again the facts of the conversations that
| changed and moving backfills and rebuilds on; every minute the
| schedule restarts it if it stopped. Every hour the workspaces whose
| day ended get their backlog counted; the digests have their own job.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
refreshPause is how long a refresh sleeps between runs, and maxPasses
how many passes one run makes at most.
*/
const (
	refreshPause = 3 * time.Second
	maxPasses    = 20
)

/*
ReportWork is what the report jobs run.
*/
type ReportWork interface {
	/*
		Refresh works out changed facts and reports whether work is left.
	*/
	Refresh(ctx context.Context, dueBefore time.Time) (bool, error)
	/*
		Snapshot counts the backlog of workspaces whose day ended.
	*/
	Snapshot(ctx context.Context) error
	/*
		SendDigests sends the report digests that are due.
	*/
	SendDigests(ctx context.Context) error
}

/*
RefreshReportsArgs names the refresh; it takes no arguments.
*/
type RefreshReportsArgs struct{}

/*
Kind names the job.
*/
func (RefreshReportsArgs) Kind() string { return "refresh_reports" }

/*
Options keep one refresh at a time; the next minute's run tries again.
*/
func (RefreshReportsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1, Unique: queue.UniqueOpts{ByArgs: true, ByState: pendingStates()}}
}

/*
RefreshReports works out changed facts.
*/
type RefreshReports struct {
	Work ReportWork
	Now  func() time.Time
}

/*
Handle passes over the changes while work is left, a few passes at
most, then sleeps three seconds and runs again: the refresh keeps going
on its own, and the minute's schedule only restarts one that stopped.
*/
func (w *RefreshReports) Handle(ctx context.Context, _ *queue.Job[RefreshReportsArgs]) error {
	for range maxPasses {
		more, err := w.Work.Refresh(ctx, w.Now())
		if err != nil {
			return err
		}

		if !more {
			break
		}
	}

	return queue.Postpone(refreshPause)
}

/*
SnapshotBacklogsArgs names the backlog count; it takes no arguments.
*/
type SnapshotBacklogsArgs struct{}

/*
Kind names the job.
*/
func (SnapshotBacklogsArgs) Kind() string { return "snapshot_backlogs" }

/*
SnapshotBacklogs counts the backlog of workspaces whose day ended.
*/
type SnapshotBacklogs struct {
	Work ReportWork
}

/*
Handle counts the backlogs.
*/
func (w *SnapshotBacklogs) Handle(ctx context.Context, _ *queue.Job[SnapshotBacklogsArgs]) error {
	return w.Work.Snapshot(ctx)
}
