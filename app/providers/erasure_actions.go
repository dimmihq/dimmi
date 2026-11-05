/*
|--------------------------------------------------------------------------
| Erasure Actions
|--------------------------------------------------------------------------
|
| Builds the actions that answer a request to erase a customer and the
| job that carries it out, reading the grace period from the contacts
| settings, and the actions and jobs that export a customer's data.
| Parts of the product that keep personal data register their erasers
| and exporters on the application before the jobs ever run.
|
*/

package providers

import (
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/erasures"
	"github.com/dimmihq/dimmi/app/actions/retention"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootErasures builds the erasure actions and registers the erase job.
*/
func bootErasures(a *app.App) error {
	acts := a.Actions.Inbox
	grace := time.Duration(a.Config.Contacts.EraseGraceDays) * 24 * time.Hour

	acts.RequestErasure = &erasures.RequestErasure{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Queue: a.Queue, Keys: a.Keys, Grace: grace,
		ConfirmWindow: a.Config.Auth.PasswordConfirmation.TTL,
	}
	acts.CancelErasure = &erasures.CancelErasure{DB: a.DB, Events: a.Events, Clock: a.Clock, Keys: a.Keys}
	acts.ListErasures = &erasures.ListErasures{DB: a.DB}
	acts.ShowErasure = &erasures.ShowErasure{DB: a.DB}

	acts.EraseContact = &erasures.EraseContact{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Storage: a.Storage, Retention: a.Retention,
		Erasers: a.ContactErasers,
	}
	if err := queue.Register(a.Queue, &jobs.EraseContact{Erase: acts.EraseContact}); err != nil {
		return fmt.Errorf("erase job: %w", err)
	}

	return bootContactExports(a)
}

/*
bootContactExports builds the export and retention actions and
registers their jobs. Zips go on the default disk and can be downloaded
for ExportTTLHours.
*/
func bootContactExports(a *app.App) error {
	acts := a.Actions.Inbox

	acts.RequestContactExport = &contacts.RequestContactExport{DB: a.DB, Clock: a.Clock, Queue: a.Queue}
	acts.ShowContactExport = &contacts.ShowContactExport{DB: a.DB}
	acts.DownloadContactExport = &contacts.DownloadContactExport{DB: a.DB, Storage: a.Storage}
	acts.BuildContactExport = &contacts.BuildContactExport{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Storage: a.Storage,
		DiskName: a.Application.Config.Filesystems.Default, Exporters: a.ContactExporters,
		TTL: time.Duration(a.Config.Contacts.ExportTTLHours) * time.Hour,
	}
	acts.ExpireContactExports = &contacts.ExpireContactExports{DB: a.DB, Clock: a.Clock, Storage: a.Storage}

	acts.ApplyRetention = &retention.ApplyConversationRetention{DB: a.DB, Clock: a.Clock, Retention: a.Retention}
	acts.PreviewRetention = &retention.PreviewRetention{DB: a.DB, Clock: a.Clock}

	return errors.Join(
		queue.Register(a.Queue, &jobs.BuildContactExport{Build: acts.BuildContactExport}),
		queue.Register(a.Queue, &jobs.ExpireContactExports{Expire: acts.ExpireContactExports}),
		queue.Register(a.Queue, &jobs.ApplyConversationRetention{Apply: acts.ApplyRetention}),
	)
}
