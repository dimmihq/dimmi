/*
|--------------------------------------------------------------------------
| Playing A Conversation
|--------------------------------------------------------------------------
|
| A conversation starts with the customer's first message through the
| same action the API uses, then alternates: the pipeline answers the
| newest customer message exactly as the answer job would, and the
| simulated customer writes the next one. It ends when the customer is
| done, the agent hands over or steps aside, the turn limit is reached,
| or a spend limit would be passed. Customer messages arrive on the
| API connection, the one channel the actions accept a customer message
| on; the agent answers each one as the messenger conversation the
| scenario stands for.
|
*/

package costsim

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/models"
)

/*
turn is one conversation to play: the run's options, the prepared
application, the customer, its place in the run and its scenario.
*/
type turn struct {
	o        Options
	p        *prepared
	customer Customer
	index    int
	sc       Scenario
}

/*
playing is a conversation in progress: where it is, its trigger
message, what was said and how it is going.
*/
type playing struct {
	conv       models.Conversation
	api        ids.UUID
	trigger    ids.UUID
	transcript []Turn
	res        *ConversationResult
}

/*
conversation plays one scenario to its end and reads back its cost.
*/
func (h *Harness) conversation(ctx context.Context, t turn) ConversationResult {
	h.Budget.StartConversation()
	res := ConversationResult{Index: t.index + 1, Scenario: t.sc.Key, Kind: t.sc.Kind, Models: map[string]int{}}
	pl, err := h.open(ctx, t, &res)
	if err != nil {
		res.Outcome, res.Error = "create_failed", err.Error()
		return res
	}
	for h.answer(ctx, t, pl) && h.next(ctx, t, pl) {
		res.Turns++
	}
	if err := h.read(ctx, pl.conv, &res); err != nil && res.Error == "" {
		res.Error = err.Error()
	}
	return res
}

/*
open starts the conversation with the customer's first message.
*/
func (h *Harness) open(ctx context.Context, t turn, res *ConversationResult) (*playing, error) {
	first, done, err := t.customer.Next(ctx, t.sc, 0, nil)
	if err != nil || done {
		return nil, fmt.Errorf("no opening message: %w", err)
	}
	email := fmt.Sprintf("sim-%s-%03d@customers.costsim.test", t.o.Label, t.index+1)
	name, subject := fmt.Sprintf("Sim %s %d", t.sc.Key, t.index+1), t.sc.Subject
	loaded, err := h.App.Actions.Inbox.CreateConversation.Handle(ctx, conversations.CreateConversationInput{
		Membership: t.o.Owner, ByContact: true, InboxID: &t.o.Inbox,
		Contact: contacts.Ref{WorkspaceID: t.o.Owner.Workspace.ID, Email: &email, Name: &name},
		Subject: &subject, Body: first,
	})
	if err != nil {
		return nil, err
	}
	pl := &playing{
		conv: loaded.Conversation, api: loaded.Connection.ID, res: res,
		transcript: []Turn{{Who: "customer", Text: first}},
	}
	res.ConversationID = models.PublicID(models.PrefixConversation, pl.conv.ID)
	res.Turns = 1
	res.Outcome = "customer_done"
	account := fmt.Sprintf("acct_sim_%s_%03d", t.o.Label, t.index+1)
	if err := h.mark(ctx, pl.conv, t.sc, account); err != nil {
		return nil, err
	}
	switch {
	case loaded.LastMessage != nil:
		pl.trigger = loaded.LastMessage.ID
	case pl.conv.LastMessageID != nil:
		pl.trigger = *pl.conv.LastMessageID
	default:
		return nil, fmt.Errorf("the conversation has no first message")
	}
	return pl, nil
}

/*
answer runs the pipeline on the newest customer message and reports
whether the conversation goes on.
*/
func (h *Harness) answer(ctx context.Context, t turn, pl *playing) bool {
	if err := h.channel(ctx, pl.conv, t.o.Channel); err != nil {
		pl.res.Outcome, pl.res.Error = "channel_failed", err.Error()
		return false
	}
	out, err := h.App.AI.Run(ctx, pipeline.Args{
		WorkspaceID: pl.conv.WorkspaceID, ConversationID: pl.conv.ID, TriggerMessageID: pl.trigger,
		Kind: "conversation", QueuedAt: h.App.Clock.Now(),
	})
	if err != nil {
		pl.res.Outcome, pl.res.Error = "pipeline_error", err.Error()
		return false
	}
	if by := h.Budget.Refused(); by != "" {
		pl.res.Outcome = "stopped_at_" + by + "_limit"
		return false
	}
	if out.AnswerID != nil {
		if text := h.body(ctx, pl.conv, *out.AnswerID); text != "" {
			pl.transcript = append(pl.transcript, Turn{Who: "agent", Text: text})
		}
	}
	switch {
	case out.Outcome == "handoff":
		pl.res.Outcome = "handed_off"
	case out.Outcome == "skipped":
		pl.res.Outcome = "skipped_" + out.SkipReason
	case t.o.MaxTurns > 0 && pl.res.Turns >= t.o.MaxTurns:
		pl.res.Outcome = "turn_limit"
	case !h.convRoom(t.p):
		pl.res.Outcome = "stopped_at_conversation_limit"
	default:
		return true
	}
	return false
}

/*
convRoom reports whether the conversation's budget has room for
another turn.
*/
func (h *Harness) convRoom(p *prepared) bool {
	_, left := h.Budget.Remaining()
	return left < 0 || left >= h.turnFloor(p.agent)
}

/*
next has the customer write the next message and adds it, reporting
whether the conversation goes on.
*/
func (h *Harness) next(ctx context.Context, t turn, pl *playing) bool {
	text, done, err := t.customer.Next(ctx, t.sc, pl.res.Turns, pl.transcript)
	switch {
	case err != nil && IsLimit(err):
		pl.res.Outcome = "stopped_at_conversation_limit"
		return false
	case err != nil:
		pl.res.Outcome, pl.res.Error = "customer_error", err.Error()
		return false
	case done:
		return false
	}
	if err := h.channel(ctx, pl.conv, pl.api); err != nil {
		pl.res.Outcome, pl.res.Error = "channel_failed", err.Error()
		return false
	}
	written, err := h.App.Actions.Inbox.AddContactMessage.Handle(ctx, messages.ContactMessageInput{
		Membership: t.o.Owner, Ref: conversations.Ref{ID: pl.conv.ID}, Body: text,
	})
	if err != nil {
		pl.res.Outcome, pl.res.Error = "message_failed", err.Error()
		return false
	}
	pl.trigger = written.Message.ID
	pl.transcript = append(pl.transcript, Turn{Who: "customer", Text: text})
	return true
}
