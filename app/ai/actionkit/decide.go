/*
|--------------------------------------------------------------------------
| Agreeing To A Call
|--------------------------------------------------------------------------
|
| The customer confirms or cancels a call proposed in their own
| conversation; a teammate approves or declines one waiting for the
| team. Each decision moves the call in one guarded statement, so a
| double click, two teammates at once or a decision after the deadline
| never runs anything twice. The customer's own repeated click is
| answered with where the call stands; a teammate deciding a call that
| is no longer waiting is told so. A confirmed call either waits for its
| approval or goes to execution; an approved one goes to execution; a
| declined one wakes the agent to tell the customer.
|
*/

package actionkit

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Ref names a call in a conversation, as a visitor reaches it.
*/
type Ref struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	CallID         ids.UUID
}

/*
Confirm records the customer's yes. A call they already confirmed is
returned as it stands.
*/
func (k *Kit) Confirm(ctx context.Context, ref Ref) (models.AiActionCall, error) {
	var out models.AiActionCall
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		call, err := findInConversation(ctx, q, ref)
		if err != nil {
			return err
		}
		if call.State != "awaiting_confirmation" || call.ExpiresAt == nil || !call.ExpiresAt.After(k.Clock.Now()) {
			out = call
			return k.alreadyConfirmed(call)
		}
		out, err = k.moveConfirmed(ctx, tx, call)
		return err
	})
	return out, err
}

/*
alreadyConfirmed answers a confirmation of a call that no longer waits
for one: fine when it was confirmed, a conflict otherwise.
*/
func (k *Kit) alreadyConfirmed(call models.AiActionCall) error {
	switch {
	case call.ConfirmedAt != nil && call.State != "canceled" && call.State != "expired":
		return nil
	case call.State == "expired", call.State == "awaiting_confirmation":
		return exceptions.ErrActionCallExpired
	}
	return exceptions.ErrActionCallNotPending
}

/*
moveConfirmed moves a waiting call on: to the team's approval when the action
asks for one, else to execution.
*/
func (k *Kit) moveConfirmed(ctx context.Context, tx *database.Tx, call models.AiActionCall,
) (models.AiActionCall, error) {
	q, now := models.New(tx), k.Clock.Now()
	params := models.ConfirmActionCallParams{
		WorkspaceID: call.WorkspaceID, ID: call.ID, NextState: "approved", Now: now,
	}
	if call.Approval != ApprovalNone {
		s, err := specOf(ctx, q, call)
		if err != nil {
			return call, err
		}
		expires := now.Add(approvalTTL(s))
		params.NextState, params.ExpiresAt = "pending_approval", &expires
	}
	row, err := q.ConfirmActionCall(ctx, params)
	if errors.Is(err, database.ErrNotFound) {
		return call, exceptions.ErrActionCallNotPending
	}
	if err != nil {
		return call, fmt.Errorf("confirm action call: %w", err)
	}
	if err := k.note(ctx, tx, step{row, contactActor(row), EventConfirmed}); err != nil {
		return row, err
	}
	if row.State == "pending_approval" {
		if err := k.askApproval(ctx, tx, row); err != nil {
			return row, err
		}
		return row, k.scheduleExpiry(ctx, tx, row)
	}
	return row, k.run(ctx, tx, row)
}

/*
Cancel records the customer's no. A call already canceled is returned
as it stands.
*/
func (k *Kit) Cancel(ctx context.Context, ref Ref) (models.AiActionCall, error) {
	var out models.AiActionCall
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		call, err := findInConversation(ctx, q, ref)
		if err != nil {
			return err
		}
		out = call
		if call.State == "canceled" {
			return nil
		}
		row, err := q.CancelActionCall(ctx, models.CancelActionCallParams{
			WorkspaceID: call.WorkspaceID, ID: call.ID, CancelReason: new("customer"), Now: k.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrActionCallNotPending
		}
		if err != nil {
			return fmt.Errorf("cancel action call: %w", err)
		}
		out = row
		return k.note(ctx, tx, step{row, contactActor(row), EventCanceled})
	})
	return out, err
}

/*
DecideInput is a teammate's decision on a call waiting for the team:
approve or decline, the internal reason, and for a decline the message
the agent passes on to the customer.
*/
type DecideInput struct {
	Membership      authz.Membership
	CallID          ids.UUID
	Approve         bool
	Reason          string
	CustomerMessage string
}

/*
Decide records a teammate's approval or decline. Any member who may
approve calls decides a teammate approval; an admin approval needs a
member who manages actions.
*/
func (k *Kit) Decide(ctx context.Context, in DecideInput) (models.AiActionCall, error) {
	m := in.Membership
	if !m.Can("ai_action_calls:approve") {
		return models.AiActionCall{}, exceptions.ErrForbidden
	}
	var out models.AiActionCall
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		call, err := q.FindActionCall(ctx, models.FindActionCallParams{WorkspaceID: m.Workspace.ID, ID: in.CallID})
		if err != nil || call.Mode != "live" {
			return exceptions.ErrActionCallNotFound
		}
		if call.Approval == ApprovalAdmin && !m.Can("ai_actions:manage") {
			return exceptions.ErrForbidden
		}
		out, err = k.applyDecision(ctx, tx, call, in)
		return err
	})
	return out, err
}

/*
applyDecision moves the call and leaves the work it calls for.
*/
func (k *Kit) applyDecision(ctx context.Context, tx *database.Tx, call models.AiActionCall, in DecideInput,
) (models.AiActionCall, error) {
	q, now := models.New(tx), k.Clock.Now()
	next, event := "declined", EventDeclined
	if in.Approve {
		next, event = "approved", EventApproved
	}
	member := in.Membership.Member.ID
	row, err := q.DecideActionCall(ctx, models.DecideActionCallParams{
		WorkspaceID: call.WorkspaceID, ID: call.ID, NextState: next, MemberID: &member,
		DecisionReason: nilString(in.Reason), CustomerMessage: nilString(in.CustomerMessage), Now: now,
	})
	if errors.Is(err, database.ErrNotFound) {
		if call.State == "pending_approval" || call.State == "expired" {
			return call, exceptions.ErrActionCallExpired
		}
		return call, exceptions.ErrActionCallNotPending
	}
	if err != nil {
		return call, fmt.Errorf("decide action call: %w", err)
	}
	if err := k.note(ctx, tx, step{row, memberActor(ctx, q, in.Membership), event}); err != nil {
		return row, err
	}
	if in.Approve {
		return row, k.run(ctx, tx, row)
	}
	return row, k.wake(ctx, tx, row)
}

/*
run queues the execution of an approved call.
*/
func (k *Kit) run(ctx context.Context, tx *database.Tx, call models.AiActionCall) error {
	if k.Queue == nil {
		return nil
	}
	return k.Queue.PushTx(ctx, tx, ExecuteCallArgs{WorkspaceID: call.WorkspaceID, CallID: call.ID})
}

/*
findInConversation finds a live call of the conversation, or 404.
*/
func findInConversation(ctx context.Context, q *models.Queries, ref Ref) (models.AiActionCall, error) {
	call, err := q.FindConversationActionCall(ctx, models.FindConversationActionCallParams{
		WorkspaceID: ref.WorkspaceID, ConversationID: &ref.ConversationID, ID: ref.CallID,
	})
	if err != nil {
		return call, exceptions.ErrActionCallNotFound
	}
	return call, nil
}

/*
memberActor is the deciding teammate as the timeline names them.
*/
func memberActor(ctx context.Context, q *models.Queries, m authz.Membership) conversations.Actor {
	actor := conversations.Actor{Type: string(enums.AuthorTypeMember), ID: m.Member.ID}
	if members, err := conversations.Members(ctx, q, m.Workspace.ID, []ids.UUID{m.Member.ID}); err == nil {
		actor.Name = members[m.Member.ID].User.Name
	}
	return actor
}
