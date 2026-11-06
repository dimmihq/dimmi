/*
|--------------------------------------------------------------------------
| Data Export Jobs
|--------------------------------------------------------------------------
|
| Exports are built in the background, since a large one reads many
| conversations, and every hour the files whose export period ran out
| are removed.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
BuildDataExportArgs names the export a job builds.
*/
type BuildDataExportArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ExportID    ids.UUID `json:"export_id"`
}

/*
Kind names the job.
*/
func (BuildDataExportArgs) Kind() string { return "build_data_export" }

/*
Options build one export at a time, retried a few times.
*/
func (BuildDataExportArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: 5,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRetryable, queue.StateRunning, queue.StateScheduled,
		}},
	}
}

/*
DataExportBuilder builds one export.
*/
type DataExportBuilder interface {
	/*
		Handle builds the export the arguments name.
	*/
	Handle(ctx context.Context, args BuildDataExportArgs) error
}

/*
BuildDataExport builds requested data exports.
*/
type BuildDataExport struct {
	Build DataExportBuilder
}

/*
Handle builds the export.
*/
func (w *BuildDataExport) Handle(ctx context.Context, job *queue.Job[BuildDataExportArgs]) error {
	return w.Build.Handle(tenancy.With(ctx, job.Args.WorkspaceID.String()), job.Args)
}

/*
ExpireDataExportsArgs names the job; it takes no arguments.
*/
type ExpireDataExportsArgs struct{}

/*
Kind names the job.
*/
func (ExpireDataExportsArgs) Kind() string { return "expire_data_exports" }

/*
Options run one round an hour, tried three times.
*/
func (ExpireDataExportsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByPeriod: time.Hour}}
}

/*
DataExportExpirer expires a batch of exports.
*/
type DataExportExpirer interface {
	/*
		Handle expires a batch and returns how many it expired.
	*/
	Handle(ctx context.Context) (int, error)
}

/*
ExpireDataExports deletes the files of expired exports.
*/
type ExpireDataExports struct {
	Expire DataExportExpirer
}

/*
Handle expires a batch, and runs again at once while a batch comes back
with exports in it.
*/
func (w *ExpireDataExports) Handle(ctx context.Context, _ *queue.Job[ExpireDataExportsArgs]) error {
	for {
		expired, err := w.Expire.Handle(ctx)
		if err != nil || expired == 0 {
			return err
		}
	}
}
