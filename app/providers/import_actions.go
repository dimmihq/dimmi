/*
|--------------------------------------------------------------------------
| Import Actions
|--------------------------------------------------------------------------
|
| Spreadsheets of customers and companies are kept as drafts, tried in
| dry runs, and run by a job. A customer row goes through the same
| upsert the API uses.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
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
		DB: a.DB, Clock: a.Clock, Queue: a.Queue,
		Upsert: &contacts.UpsertIdentifiedContact{DB: a.DB, Events: a.Events, Clock: a.Clock},
	}

	return queue.Register(a.Queue, &jobs.RunImport{Imports: acts.Imports})
}
