/*
|--------------------------------------------------------------------------
| Workspace Actions In The Loop
|--------------------------------------------------------------------------
|
| The workspace's published actions join the agent's tools for a
| conversation turn, filtered to what the channel and the customer's
| proven identity allow. A read runs inline and its result goes back to
| the model; a write may wait for the customer's confirmation or a
| teammate's approval, and either way ends the tool rounds, so the agent
| makes one write per turn and then tells the customer where it stands.
| The reply that follows carries the confirmation card, and the calls
| are linked to it when it is committed.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
oneWritePerTurn is what the model reads when it asks for a second write
in one turn.
*/
const oneWritePerTurn = `{"error":"one_write_per_turn","message":"only one change per turn; ` +
	`tell the customer about the first one"}`

/*
actionState is what a turn did with the workspace's actions: what it
offered, who the customer is, the provider's id of the tool call being
handled, whether tools are closed after a write, whether a result
reached the model, the calls made, the call whose card the reply
carries, the call a follow-up reports on, and whether writes are kept
out.
*/
type actionState struct {
	offered    []actionkit.Live
	identity   actionkit.Identity
	toolCallID string
	closed     bool
	answered   bool
	waiting    bool
	calls      []models.AiActionCall
	card       *models.AiActionCall
	followUp   *models.AiActionCall
	noWrites   bool
}

/*
offerActions loads the actions the turn may call and the brief of the
conversation's requests. A failure to load them costs only the actions:
the agent still answers from its knowledge.
*/
func (r *Runner) offerActions(ctx context.Context, st *run) {
	e := st.env
	if r.Actions == nil || !e.Agent.ActionsEnabled || e.Conversation.ID == (ids.UUID{}) ||
		st.args.Kind == playgroundKind {
		return
	}
	if st.actions.followUp == nil {
		st.actions.identity = identityOf(e.ChannelType, e.Trigger, e.Contact)
	}
	offered, err := r.Actions.Offer(ctx, actionkit.OfferInput{
		WorkspaceID: e.Workspace.ID, ChannelType: e.ChannelType, Identity: st.actions.identity,
		NoWrites: st.actions.noWrites, Agent: actionkit.AgentDimmi,
	})
	if err == nil {
		st.actions.offered = offered
	}
	if brief, err := r.Actions.Brief(ctx, e.Workspace.ID, e.Conversation.ID); err == nil {
		st.loop.brief = brief
	}
}

/*
identityOf is what the trigger proves about the customer: a signed-in
session proves who they are, an email code only their email, and an
anonymous visitor may still prove their email when an action needs it.
*/
func identityOf(channelType string, trigger models.Message, contact *models.Contact) actionkit.Identity {
	verified := actionkit.VerifiedContext(channelType, trigger)
	proven, codes := actionkit.EmailContext(channelType, trigger)
	return actionkit.Identity{
		Verified: verified, EmailProven: proven && !verified, EmailCodes: codes && !verified, Contact: contact,
	}
}

/*
actionHandler is the tool handler of one offered action.
*/
func (r *Runner) actionHandler(live actionkit.Live) func(context.Context, *run, json.RawMessage) (string, error) {
	return func(ctx context.Context, st *run, args json.RawMessage) (string, error) {
		write := live.Spec.SideEffect == actionkit.SideEffectWrite
		if write && st.actions.closed {
			return oneWritePerTurn, nil
		}
		e := st.env
		reply, err := r.Actions.Call(ctx, actionkit.CallInput{
			WorkspaceID: e.Workspace.ID, ConversationID: e.Conversation.ID, ContactID: e.Conversation.ContactID,
			RunID: st.rec.Run.ID, Live: live, ToolCallID: st.actions.toolCallID, Args: args,
			Identity: st.actions.identity,
			Agent:    conversations.Actor{Type: string(enums.AuthorTypeAgent), ID: e.Agent.ID, Name: e.Agent.Name},
		})
		if err != nil {
			return "", err
		}
		st.record(live, reply)
		return reply.Text, nil
	}
}

/*
record keeps what one action call did to the turn.
*/
func (st *run) record(live actionkit.Live, reply actionkit.CallReply) {
	if reply.Call == nil {
		return
	}
	st.actions.calls = append(st.actions.calls, *reply.Call)
	st.flags = appendOnce(st.flags, "action_called")
	if reply.Succeeded {
		st.actions.answered = true
	}
	if live.Spec.SideEffect == actionkit.SideEffectWrite {
		st.actions.closed = true
	}
	if reply.Awaiting || reply.Pending {
		st.actions.waiting = true
	}
	if reply.Awaiting {
		call := *reply.Call
		st.actions.card = &call
	}
}

/*
linkActions ties the turn's calls to the reply that told the customer
about them, and marks a follow-up's call reported.
*/
func (r *Runner) linkActions(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	if d.answerID == nil {
		return nil
	}
	q, now, ws := models.New(tx), r.Clock.Now(), st.env.Workspace.ID
	if len(st.actions.calls) > 0 {
		callIDs := make([]ids.UUID, len(st.actions.calls))
		for i, c := range st.actions.calls {
			callIDs[i] = c.ID
		}
		if err := q.SetActionCallsMessage(ctx, models.SetActionCallsMessageParams{
			WorkspaceID: ws, MessageID: d.answerID, Ids: callIDs, Now: now,
		}); err != nil {
			return err
		}
	}
	if f := st.actions.followUp; f != nil {
		return q.MarkActionCallReported(ctx, models.MarkActionCallReportedParams{
			WorkspaceID: ws, ID: f.ID, ReportRunID: &st.rec.Run.ID, Now: now,
		})
	}
	return nil
}

/*
actionMetadata adds the turn's calls to the reply's metadata: the
confirmation card the widget renders, and the calls teammates see.
*/
func actionMetadata(st *run, ai map[string]any) {
	if c := st.actions.card; c != nil {
		ai["confirmation"] = map[string]any{
			"action_call_id": models.PublicID(models.PrefixAiActionCall, c.ID), "title": c.Title,
			"text": c.Summary, "status": c.State, "expires_at": c.ExpiresAt, "details": details(*c),
		}
	}
	if len(st.actions.calls) == 0 {
		return
	}
	ai["action_call"] = reportedCall(st)
	calls := make([]map[string]any, 0, len(st.actions.calls))
	for _, c := range st.actions.calls {
		calls = append(calls, map[string]any{
			"id": models.PublicID(models.PrefixAiActionCall, c.ID), "title": c.Title, "status": c.State,
		})
	}
	ai["action_calls"] = calls
}

/*
reportedCall is the call the reply reports in full: the turn's write
when it made one, else its last call.
*/
func reportedCall(st *run) map[string]any {
	c := st.actions.calls[len(st.actions.calls)-1]
	for _, call := range st.actions.calls {
		if call.SideEffect == actionkit.SideEffectWrite {
			c = call
		}
	}
	name := ""
	for _, live := range st.actions.offered {
		if live.ActionID == c.ActionID {
			name = live.Spec.Name
		}
	}
	args := map[string]any{}
	_ = json.Unmarshal(c.Input, &args)
	out := actionkit.MessagePatch(c, conversations.Actor{})
	out["id"] = models.PublicID(models.PrefixAiActionCall, c.ID)
	out["action"] = map[string]any{"name": name, "title": c.Title}
	out["summary"], out["arguments"], out["expires_at"] = c.Summary, args, c.ExpiresAt
	return out
}

/*
details are the card's lines: each argument the customer confirms, by
name.
*/
func details(c models.AiActionCall) []map[string]string {
	args := map[string]any{}
	_ = json.Unmarshal(c.Input, &args)
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		value, _ := json.Marshal(args[k])
		out = append(out, map[string]string{
			"label": strings.ReplaceAll(k, "_", " "), "value": strings.Trim(string(value), `"`),
		})
	}
	return out
}

/*
appendOnce adds a flag unless it is there.
*/
func appendOnce(flags []string, flag string) []string {
	if slices.Contains(flags, flag) {
		return flags
	}
	return append(flags, flag)
}
