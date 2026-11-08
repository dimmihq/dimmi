/*
|--------------------------------------------------------------------------
| What The Customer May Read
|--------------------------------------------------------------------------
|
| dimmi answers a customer only from articles they could read
| themselves, decided again on every search, so a customer who just
| upgraded gets the new articles on the next answer. A teammate trying
| dimmi out in the playground has no customer, so nothing is held back.
|
*/

package pipeline

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/models"
)

/*
readerAudiences lists the saved audiences the run's customer is in,
among those articles need, and whether to filter by them at all.
*/
func (r *Runner) readerAudiences(ctx context.Context, st *run) ([]ids.UUID, bool, error) {
	contact := st.env.Contact
	if contact == nil {
		return nil, false, nil
	}

	ws := st.env.Workspace.ID
	var allowed []ids.UUID
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		restrictions, err := audiences.LoadRestrictions(ctx, q, ws)
		if err != nil || len(restrictions.Needed()) == 0 {
			return err
		}

		people, err := audiences.LoadPeople(ctx, q, ws, []models.Contact{*contact})
		if err != nil {
			return err
		}

		allowed, err = audiences.Allowed(ctx, q, audiences.Reader{
			WorkspaceID: ws, Person: &people[0], Needed: restrictions.Needed(), Now: r.Clock.Now(),
		})

		return err
	})

	return allowed, true, err
}
