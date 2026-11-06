/*
|--------------------------------------------------------------------------
| Contact Export Jobs
|--------------------------------------------------------------------------
|
| Building a customer's export runs in the background, scoped to their
| workspace, and is retried when it fails. Once an hour the exports past
| their expiry lose their zip.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/contacts"
)

/*
BuildContactExport builds requested exports.
*/
type BuildContactExport struct {
	Build *contacts.BuildContactExport
}

/*
Handle builds the export.
*/
func (w *BuildContactExport) Handle(ctx context.Context, job *queue.Job[contacts.BuildExportArgs]) error {
	return w.Build.Handle(tenancy.With(ctx, job.Args.WorkspaceID.String()), job.Args)
}

/*
ExpireContactExportsArgs names the job; it takes no arguments.
*/
type ExpireContactExportsArgs struct{}

/*
Kind names the job.
*/
func (ExpireContactExportsArgs) Kind() string { return "expire_contact_exports" }

/*
Options run one round an hour, tried three times.
*/
func (ExpireContactExportsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByPeriod: time.Hour}}
}

/*
ExpireContactExports deletes expired exports.
*/
type ExpireContactExports struct {
	Expire *contacts.ExpireContactExports
}

/*
Handle expires a batch, and runs again at once while a batch comes back
full.
*/
func (w *ExpireContactExports) Handle(ctx context.Context, _ *queue.Job[ExpireContactExportsArgs]) error {
	for {
		expired, err := w.Expire.Handle(ctx)
		if err != nil || expired == 0 {
			return err
		}
	}
}
