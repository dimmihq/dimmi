/*
|--------------------------------------------------------------------------
| Data Export Actions
|--------------------------------------------------------------------------
|
| Members' exports of conversations and customers are built by a job and
| stored on the default disk, and an hourly round removes the files
| whose export period ran out.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/dataexports"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootDataExports builds the data export actions and registers their
jobs.
*/
func bootDataExports(a *app.App) error {
	acts := a.Actions.Inbox
	selector := conversations.Selector{IDs: a.IDs, Clock: a.Clock}

	acts.RequestDataExport = &dataexports.RequestDataExport{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Queue: a.Queue, Selector: selector,
	}
	acts.ListDataExports = &dataexports.ListDataExports{DB: a.DB}
	acts.ShowDataExport = &dataexports.ShowDataExport{DB: a.DB}
	acts.DownloadDataExport = &dataexports.DownloadDataExport{DB: a.DB, Storage: a.Storage}
	acts.DeleteDataExport = &dataexports.DeleteDataExport{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Storage: a.Storage,
	}
	acts.BuildDataExport = &dataexports.BuildDataExport{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Storage: a.Storage,
		DiskName: a.Application.Config.Filesystems.Default, Selector: selector,
	}
	acts.ExpireDataExports = &dataexports.ExpireDataExports{DB: a.DB, Clock: a.Clock, Storage: a.Storage}

	return errors.Join(
		queue.Register(a.Queue, &jobs.BuildDataExport{Build: acts.BuildDataExport}),
		queue.Register(a.Queue, &jobs.ExpireDataExports{Expire: acts.ExpireDataExports}),
	)
}
