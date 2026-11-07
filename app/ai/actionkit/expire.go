/*
|--------------------------------------------------------------------------
| Expiring Calls
|--------------------------------------------------------------------------
|
| A call never waits forever. A confirmation the customer left unanswered
| simply lapses. An approval the team did not give in time lapses too,
| and then the customer is not left wondering: the agent is woken to say
| it is taking longer, and the conversation goes to the team with high
| priority.
|
*/

package actionkit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Expire ends a waiting call whose deadline has passed. A deadline still
ahead returns how long to wait; a call that moved on, or whose
deadline is not the one given (a zero at matches any), returns zero
and does nothing.
*/
func (k *Kit) Expire(ctx context.Context, workspaceID, callID ids.UUID, at time.Time) (time.Duration, error) {
	var wait time.Duration
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now := models.New(tx), k.Clock.Now()
		call, err := q.FindActionCall(ctx, models.FindActionCallParams{WorkspaceID: workspaceID, ID: callID})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find action call: %w", err)
		}
		switch {
		case call.ExpiresAt == nil || !waiting(call):
			return nil
		case !at.IsZero() && call.ExpiresAt.UnixMilli() != at.UnixMilli():
			return nil
		case call.ExpiresAt.After(now):
			wait = call.ExpiresAt.Sub(now)
			return nil
		}
		return k.lapse(ctx, tx, call)
	})
	return wait, err
}

/*
lapse ends the call and, for a lapsed approval, wakes the agent.
*/
func (k *Kit) lapse(ctx context.Context, tx *database.Tx, call models.AiActionCall) error {
	q := models.New(tx)
	wasApproval := call.State == "pending_approval"
	row, err := q.ExpireActionCall(ctx, models.ExpireActionCallParams{
		WorkspaceID: call.WorkspaceID, ID: call.ID, Now: k.Clock.Now(),
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("expire action call: %w", err)
	}
	if err := k.note(ctx, tx, step{row, agentActor(ctx, q, row.WorkspaceID), EventExpired}); err != nil {
		return err
	}
	if !wasApproval {
		return nil
	}
	return k.wake(ctx, tx, row)
}

/*
waiting reports whether a call waits for someone's decision.
*/
func waiting(call models.AiActionCall) bool {
	return call.State == "awaiting_confirmation" || call.State == "pending_approval"
}

/*
WasAwaitingApproval reports whether an expired call lapsed while it
waited for the team rather than for the customer.
*/
func WasAwaitingApproval(call models.AiActionCall) bool {
	return call.State == "expired" && call.Approval != ApprovalNone &&
		(call.Confirmation == ConfirmationNone || call.ConfirmedAt != nil)
}
