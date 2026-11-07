/*
|--------------------------------------------------------------------------
| Erasing A Customer's Runs
|--------------------------------------------------------------------------
|
| A run keeps what it collected from a customer in its variables and
| what triggered it in its trigger data, either of which can name them.
| When the customer is erased, every run made for them forgets both and
| lets go of the contact; the runs stay as a record that they happened.
| Erasing again finds nothing new to forget.
|
*/

package workflows

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
RunEraser forgets what an erased customer's runs held.
*/
type RunEraser struct {
	Engine *Engine
}

/*
Name names what the eraser removes.
*/
func (r RunEraser) Name() string {
	return "workflow_runs"
}

/*
Erase scrubs every run made for the contact.
*/
func (r RunEraser) Erase(ctx context.Context, workspaceID, contactID ids.UUID) error {
	e := r.Engine
	err := e.transaction(ctx, workspaceID, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).ScrubWorkflowRunsForContact(ctx, models.ScrubWorkflowRunsForContactParams{
			Now: e.Clock.Now(), WorkspaceID: workspaceID, ContactID: &contactID,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("scrub workflow runs: %w", err)
	}

	return nil
}
