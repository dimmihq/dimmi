/*
|--------------------------------------------------------------------------
| Erasing A Customer's Action Calls
|--------------------------------------------------------------------------
|
| An action call keeps what the agent sent on a customer's behalf and what
| came back, which can name them, their orders or their money. When the
| customer is erased, every call made for them forgets all of that; the
| rows stay only as a record that a call happened, so audits still count
| it. Erasing again finds nothing new to forget.
|
*/

package agentactions

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
CallEraser forgets what an erased customer's action calls held.
*/
type CallEraser struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Name names what the eraser removes.
*/
func (e *CallEraser) Name() string {
	return "ai_action_calls"
}

/*
Erase scrubs every call made for the contact.
*/
func (e *CallEraser) Erase(ctx context.Context, workspaceID, contactID ids.UUID) error {
	scoped := tenancy.With(ctx, workspaceID.String())
	err := e.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).ScrubActionCallsForContact(ctx, models.ScrubActionCallsForContactParams{
			Now: e.Clock.Now(), WorkspaceID: workspaceID, ContactID: &contactID,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("scrub action calls: %w", err)
	}
	return nil
}
