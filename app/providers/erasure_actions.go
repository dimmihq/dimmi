/*
|--------------------------------------------------------------------------
| Erasure Actions
|--------------------------------------------------------------------------
|
| Builds the actions that answer a request to erase a customer and the
| job that carries it out, reading the grace period from the contacts
| settings. Parts of the product that keep personal data register their
| erasers on the application before the job ever runs.
|
*/

package providers

import (
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/erasures"
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

	return nil
}
