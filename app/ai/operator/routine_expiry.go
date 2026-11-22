/*
|--------------------------------------------------------------------------
| Stale Suggestions
|--------------------------------------------------------------------------
|
| A routine's suggestion that nobody decided within four weeks has gone
| stale: what it was based on has moved on. Once a day such suggestions
| are declined, saying why, so the review queues show only what still
| matters and a later run may suggest the change again.
|
*/

package operator

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ExpireSuggestions declines the stale routine suggestions of every
workspace and returns how many.
*/
func (e *Engine) ExpireSuggestions(ctx context.Context) (int64, error) {
	e.prepare()
	workspaces, err := models.New(e.DB).ListWorkspaces(ctx)
	if err != nil {
		return 0, fmt.Errorf("list workspaces: %w", err)
	}
	var total int64
	var errs []error
	for _, ws := range workspaces {
		scoped := tenancy.With(ctx, ws.ID.String())
		err := e.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			n, err := models.New(tx).ExpireOperatorChangesets(ctx, models.ExpireOperatorChangesetsParams{
				WorkspaceID: ws.ID, Now: e.Clock.Now(),
			})
			total += n
			return err
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return total, errors.Join(errs...)
}
