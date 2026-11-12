/*
|--------------------------------------------------------------------------
| Import Actions
|--------------------------------------------------------------------------
|
| Spreadsheets of customers and companies are kept as drafts, tried in
| dry runs, and run by a job. A customer row goes through the same
| upsert the API uses. A finished import can be undone in the background,
| and a nightly sweep empties old files.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/imports"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootImports builds the import actions and registers their job.
*/
func bootImports(a *app.App) error {
	acts := a.Actions.Inbox
	acts.Imports = &imports.Imports{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Limiter: a.Limiter,
		Upsert:        &contacts.UpsertIdentifiedContact{DB: a.DB, Events: a.Events, Clock: a.Clock},
		UpdateContact: &contacts.UpdateContact{DB: a.DB, Events: a.Events, Clock: a.Clock},
		DeleteContact: &contacts.DeleteContact{DB: a.DB, Events: a.Events, Clock: a.Clock},
		UpdateCompany: &companies.UpdateCompany{DB: a.DB, Events: a.Events, Clock: a.Clock},
		DeleteCompany: &companies.DeleteCompany{DB: a.DB, Events: a.Events, Clock: a.Clock},
	}

	return errors.Join(
		queue.Register(a.Queue, &jobs.RunImport{Imports: acts.Imports}),
		queue.Register(a.Queue, &jobs.RollBackImport{Imports: acts.Imports}),
		queue.Register(a.Queue, &jobs.ExpireImports{Expire: &imports.ExpireImports{DB: a.DB, Clock: a.Clock}}),
	)
}
