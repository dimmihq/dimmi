/*
|--------------------------------------------------------------------------
| Suggestions Waiting
|--------------------------------------------------------------------------
|
| A run matters to a teammate while something it suggested still waits
| for them. Waiting counts those suggestions for each run of a routine,
| so the list of runs can say which ones still need a look.
|
*/

package operatorchat

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Waiting counts, by run, the suggestions of a routine nobody has applied
or declined yet. Runs with none are left out.
*/
func (a *Routines) Waiting(ctx context.Context, m authz.Membership, routine ids.UUID) (map[ids.UUID]int32, error) {
	if !m.Can("ai:read") {
		return nil, exceptions.ErrForbidden
	}
	out := map[ids.UUID]int32{}
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).CountWaitingOperatorRoutineSuggestions(ctx,
			models.CountWaitingOperatorRoutineSuggestionsParams{WorkspaceID: m.Workspace.ID, RoutineID: &routine})
		for _, r := range rows {
			out[r.RunID] = r.Waiting
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("count waiting suggestions: %w", err)
	}
	return out, nil
}
