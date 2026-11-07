/*
|--------------------------------------------------------------------------
| Executing Approved Calls
|--------------------------------------------------------------------------
|
| An approved call runs exactly once. Claiming it writes sent_at before
| any byte leaves, in a statement only one worker can win, so racing
| approvals or a retried job never send it twice. A job that finds a
| call claimed but never finished knows the request may have run, marks
| it outcome_unknown and never sends it again. Whatever the outcome, the
| team sees it on the timeline and the agent is woken to tell the
| customer.
|
*/

package actionkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Execute runs one approved call, once.
*/
func (k *Kit) Execute(ctx context.Context, workspaceID, callID ids.UUID) error {
	var claimed *models.AiActionCall
	var target Target
	var actor conversations.Actor
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		actor = agentActor(ctx, q, workspaceID)
		call, err := q.ClaimActionCall(ctx, models.ClaimActionCallParams{
			WorkspaceID: workspaceID, ID: callID, Now: k.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return k.abandon(ctx, tx, models.AiActionCall{WorkspaceID: workspaceID, ID: callID}, actor)
		}
		if err != nil {
			return fmt.Errorf("claim action call: %w", err)
		}
		claimed = &call
		s, err := specOf(ctx, q, call)
		if err != nil {
			return err
		}
		target, err = k.target(ctx, tx, call, s)
		return err
	})
	if err != nil || claimed == nil {
		return err
	}

	out := k.Send(ctx, target)
	_, err = k.finish(ctx, finishing{call: *claimed, out: out, actor: actor, followUp: true})
	return err
}

/*
abandon fails a call a previous attempt claimed and never finished: its
request may have left, so it is never sent again.
*/
func (k *Kit) abandon(ctx context.Context, tx *database.Tx, ref models.AiActionCall, actor conversations.Actor) error {
	call, err := models.New(tx).AbandonActionCall(ctx, models.AbandonActionCallParams{
		WorkspaceID: ref.WorkspaceID, ID: ref.ID, Now: k.Clock.Now(),
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("abandon action call: %w", err)
	}
	if err := k.note(ctx, tx, step{call, actor, EventCompleted}); err != nil {
		return err
	}
	return k.wake(ctx, tx, call)
}

/*
finishing is a call that came back: the row, the outcome, who the
timeline names, and whether the agent must be woken to report it.
*/
type finishing struct {
	call     models.AiActionCall
	out      Outcome
	actor    conversations.Actor
	followUp bool
}

/*
finish records the outcome on the call and the connection's health.
*/
func (k *Kit) finish(ctx context.Context, f finishing) (models.AiActionCall, error) {
	var done models.AiActionCall
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now, out := models.New(tx), k.Clock.Now(), f.out
		state := "failed"
		if out.Succeeded {
			state = "succeeded"
		}
		params := models.FinishActionCallParams{
			WorkspaceID: f.call.WorkspaceID, ID: f.call.ID, State: state, ErrorCode: nilString(out.ErrorCode),
			Request: out.Request, Response: out.Response, Result: out.Result, LatencyMs: millis(out), Now: now,
		}
		if out.HTTPStatus != 0 {
			params.HttpStatus = new(sat32(int64(out.HTTPStatus)))
		}
		row, err := q.FinishActionCall(ctx, params)
		if err != nil {
			return fmt.Errorf("finish action call: %w", err)
		}
		done = row
		if err := k.health(ctx, q, row, out); err != nil {
			return err
		}
		if err := k.note(ctx, tx, step{row, f.actor, EventCompleted}); err != nil {
			return err
		}
		if !f.followUp {
			return nil
		}
		return k.wake(ctx, tx, row)
	})
	return done, err
}

/*
FinishTest records how an admin's test call came back.
*/
func (k *Kit) FinishTest(ctx context.Context, call models.AiActionCall, out Outcome) (models.AiActionCall, error) {
	return k.finish(ctx, finishing{call: call, out: out})
}

/*
health notes what the call says about its connection: reached, refused
credentials, or a blocked host.
*/
func (k *Kit) health(ctx context.Context, q *models.Queries, call models.AiActionCall, out Outcome) error {
	now := k.Clock.Now()
	if out.Reached && !out.AuthFailure {
		return q.RecordActionConnectionSuccess(ctx, models.RecordActionConnectionSuccessParams{
			WorkspaceID: call.WorkspaceID, ID: call.ConnectionID, Now: now,
		})
	}
	if !out.AuthFailure && !out.Blocked && out.ErrorCode != "unavailable" {
		return nil
	}
	return q.RecordActionConnectionFailure(ctx, models.RecordActionConnectionFailureParams{
		WorkspaceID: call.WorkspaceID, ID: call.ConnectionID, LastError: nilString(out.ErrorCode),
		AuthFailure: out.AuthFailure, Blocked: out.Blocked, Now: now,
	})
}

/*
wake queues the follow-up that tells the customer how the call ended.
*/
func (k *Kit) wake(ctx context.Context, tx *database.Tx, call models.AiActionCall) error {
	if k.Queue == nil || call.ConversationID == nil || call.Mode != "live" {
		return nil
	}
	return k.Queue.PushTx(ctx, tx, FollowUpArgs{WorkspaceID: call.WorkspaceID, CallID: call.ID})
}

/*
target loads what executing a call needs: its connection and secret,
and its frozen input and bindings.
*/
func (k *Kit) target(ctx context.Context, tx *database.Tx, call models.AiActionCall, s Spec) (Target, error) {
	conn, err := models.New(tx).FindActionConnection(ctx, models.FindActionConnectionParams{
		WorkspaceID: call.WorkspaceID, ID: call.ConnectionID,
	})
	if err != nil {
		return Target{}, fmt.Errorf("find action connection: %w", err)
	}
	creds, err := OpenCredentials(ctx, k.Keys, conn)
	if err != nil {
		return Target{}, err
	}
	t := Target{Conn: conn, Creds: creds, Spec: s, CallID: call.ID, WorkspaceID: call.WorkspaceID}
	_ = json.Unmarshal(call.Input, &t.Input)
	_ = json.Unmarshal(call.Bindings, &t.Bindings)
	return t, nil
}

/*
specOf loads the frozen definition a call was made with.
*/
func specOf(ctx context.Context, q *models.Queries, call models.AiActionCall) (Spec, error) {
	if call.ActionVersionID == nil {
		return Spec{}, fmt.Errorf("action call %s has no version", call.ID)
	}
	v, err := q.FindActionVersion(ctx, models.FindActionVersionParams{
		WorkspaceID: call.WorkspaceID, ID: *call.ActionVersionID,
	})
	if err != nil {
		return Spec{}, fmt.Errorf("find action version: %w", err)
	}
	return FromVersion(v)
}

/*
millis is an outcome's latency in whole milliseconds, NULL when none
was measured.
*/
func millis(out Outcome) *int32 {
	if out.Latency <= 0 {
		return nil
	}
	return new(sat32(out.Latency.Milliseconds()))
}

/*
sat32 clamps a count into int32 for the columns that take one.
*/
func sat32(n int64) int32 {
	return int32(max(min(n, 2147483647), -2147483648))
}
