/*
|--------------------------------------------------------------------------
| Call Records
|--------------------------------------------------------------------------
|
| Every step of a live call is visible to the team: a timeline entry on
| the conversation saying who did what ("dimmi requested a refund",
| "approved by Ana", "refund issued"), the confirmation card on the
| agent's message kept in step with the call, and an event open screens
| hear. All of it is written in the transaction that moved the call.
|
*/

package actionkit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The timeline entries a call writes.
*/
const (
	EventRequested = "ai.action_call_requested"
	EventConfirmed = "ai.action_call_confirmed"
	EventApproved  = "ai.action_call_approved"
	EventDeclined  = "ai.action_call_declined"
	EventCanceled  = "ai.action_call_canceled"
	EventExpired   = "ai.action_call_expired"
	EventCompleted = "ai.action_call_completed"
)

/*
step is one step of a live call: the call as it now is, who took the
step and the timeline entry it writes.
*/
type step struct {
	call  models.AiActionCall
	actor conversations.Actor
	name  string
}

/*
note records one step of a live call: the timeline entry, the card's
status and the event.
*/
func (k *Kit) note(ctx context.Context, tx *database.Tx, s step) error {
	call, actor, name := s.call, s.actor, s.name
	if call.ConversationID == nil || call.Mode != "live" {
		return nil
	}
	q := models.New(tx)
	now := k.Clock.Now()
	w := &conversations.Writer{Q: q, Tx: tx, Events: k.Events, Now: now, Actor: actor}

	data := map[string]any{
		"call_id": models.PublicID(models.PrefixAiActionCall, call.ID),
		"action":  map[string]any{"id": models.PublicID(models.PrefixAiAction, call.ActionID), "title": call.Title},
		"state":   call.State, "summary": call.Summary,
	}
	if call.ErrorCode != nil {
		data["error_code"] = *call.ErrorCode
	}
	conv := models.Conversation{WorkspaceID: call.WorkspaceID, ID: *call.ConversationID}
	if err := w.Record(ctx, conv, name, data); err != nil {
		return err
	}

	if err := k.updateMessage(ctx, q, s); err != nil {
		return err
	}

	return k.Events.Dispatch(ctx, tx, appevents.AiActionCallUpdated{
		Base: w.Base(call.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: conv.ID},
		CallID: call.ID, State: call.State,
	})
}

/*
updateMessage keeps the agent message that reported the call in step:
the confirmation card's status and the call's state, decider and
outcome.
*/
func (k *Kit) updateMessage(ctx context.Context, q *models.Queries, s step) error {
	call, now := s.call, k.Clock.Now()
	if call.MessageID == nil {
		return nil
	}
	if err := q.SetMessageConfirmationStatus(ctx, models.SetMessageConfirmationStatusParams{
		WorkspaceID: call.WorkspaceID, ID: *call.MessageID, Status: call.State, Now: now,
	}); err != nil {
		return fmt.Errorf("update confirmation card: %w", err)
	}
	patch, err := json.Marshal(MessagePatch(call, s.actor))
	if err != nil {
		return fmt.Errorf("encode action call news: %w", err)
	}
	if err := q.SetMessageActionCall(ctx, models.SetMessageActionCallParams{
		WorkspaceID: call.WorkspaceID, ID: *call.MessageID, Patch: patch, CallID: models.PublicID(
			models.PrefixAiActionCall, call.ID), Now: now,
	}); err != nil {
		return fmt.Errorf("update reported action call: %w", err)
	}
	return nil
}

/*
MessagePatch is a call's news as the message reporting it keeps it:
its state, who decided and why, and a short line of its result.
*/
func MessagePatch(call models.AiActionCall, actor conversations.Actor) map[string]any {
	out := map[string]any{"state": call.State}
	if call.DecidedByMemberID != nil && actor.Type == string(enums.AuthorTypeMember) {
		out["decided_by"] = map[string]any{
			"id": models.PublicID(models.PrefixMember, *call.DecidedByMemberID), "name": actor.Name,
		}
	}
	if call.DecisionReason != nil {
		out["decision_reason"] = *call.DecisionReason
	}
	if call.State == "succeeded" && len(call.Result) > 0 {
		out["result_summary"] = truncateText(string(call.Result), 200)
	}
	return out
}

/*
agentActor is the workspace's agent as the timeline names it.
*/
func agentActor(ctx context.Context, q *models.Queries, workspaceID ids.UUID) conversations.Actor {
	actor := conversations.Actor{Type: string(enums.AuthorTypeAgent)}
	if agent, err := q.FindAgent(ctx, workspaceID); err == nil {
		actor.ID, actor.Name = agent.ID, agent.Name
	}
	return actor
}

/*
contactActor is the customer as the timeline names them.
*/
func contactActor(call models.AiActionCall) conversations.Actor {
	actor := conversations.Actor{Type: string(enums.AuthorTypeContact)}
	if call.ContactID != nil {
		actor.ID = *call.ContactID
	}
	return actor
}
