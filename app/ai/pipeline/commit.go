/*
|--------------------------------------------------------------------------
| Commit
|--------------------------------------------------------------------------
|
| The last short transaction. The conversation row is locked, so a
| teammate's reply and the agent's never cross, and the run looks once
| more at what landed while the model wrote: a newer customer message
| (the newer job answers both), a teammate's reply, a handoff. Any of
| those discards the reply. Otherwise the reply, the conversation's
| counters, the run row and the month's spend are written together.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
close commits the decision and returns the run's result.
*/
func (r *Runner) close(ctx context.Context, st *run, d *decision) (*Result, error) {
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		d, err = r.commit(ctx, tx, st, d)
		return err
	})
	if err != nil {
		return nil, err
	}
	return d.result(st.rec.Run.ID), nil
}

/*
commit locks the conversation, rechecks it and writes the decision,
returning the decision actually written.
*/
func (r *Runner) commit(ctx context.Context, tx *database.Tx, st *run, d *decision) (*decision, error) {
	conv, err := models.New(tx).LockConversationForAi(ctx, models.LockConversationForAiParams{
		WorkspaceID: st.env.Conversation.WorkspaceID, ID: st.env.Conversation.ID,
	})
	if err != nil {
		return d, fmt.Errorf("lock conversation: %w", err)
	}
	st.env.Conversation = conv

	if d.writesMessage() {
		stale, err := r.recheck(ctx, tx, st)
		if err != nil {
			return d, err
		}
		if stale != nil {
			d = stale
		}
	}
	if d.writesMessage() && d.outcome != outcomeError {
		if err := r.applyChanges(ctx, tx, st); err != nil {
			return d, err
		}
	}
	if err := r.write(ctx, tx, st, d); err != nil {
		return d, err
	}
	if err := r.linkActions(ctx, tx, st, d); err != nil {
		return d, fmt.Errorf("link action calls: %w", err)
	}
	if err := r.finishRun(ctx, tx, st, d); err != nil {
		return d, fmt.Errorf("finish run: %w", err)
	}
	return d, r.recordSpend(ctx, tx, st, d)
}

/*
recheck returns the decision that replaces a reply overtaken while the
model wrote, or nil when the reply still stands.
*/
func (r *Runner) recheck(ctx context.Context, tx *database.Tx, st *run) (*decision, error) {
	conv := st.env.Conversation
	switch {
	case conv.DeletedAt != nil || conv.Spam:
		return superseded("conversation_ineligible"), nil
	case conv.AiState == aiStateHandedOff:
		return superseded("handed_off"), nil
	}

	after, err := models.New(tx).ListPublicMessagesAfter(ctx, models.ListPublicMessagesAfterParams{
		WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, AfterID: st.env.Newest,
	})
	if err != nil {
		return nil, fmt.Errorf("list newer messages: %w", err)
	}
	for _, m := range after {
		switch m.AuthorType {
		case string(enums.AuthorTypeContact):
			return superseded("newer_customer_message"), nil
		case string(enums.AuthorTypeMember):
			return superseded("human_replied"), nil
		case string(enums.AuthorTypeAgent):
			return skipped("already_answered"), nil
		}
	}
	return nil, nil
}

/*
superseded is a reply discarded because the conversation moved on.
*/
func superseded(reason string) *decision {
	return &decision{outcome: outcomeSuperseded, skipReason: reason}
}

/*
write turns the decision into rows.
*/
func (r *Runner) write(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	e := st.env
	switch d.outcome {
	case outcomeAnswered:
		return r.writeAnswer(ctx, tx, st, d)
	case outcomeGreeted, outcomeRefused:
		return r.post(ctx, tx, st, d)
	case outcomeUnknown:
		return r.writeUnknown(ctx, tx, st, d)
	case outcomeHandoff:
		return r.writeHandoff(ctx, tx, st, d)
	case outcomeError:
		if e.Conversation.AiState != aiStateActive {
			return nil
		}
		d.replyKey = e.handoffKey()
		if e.availability() == "unavailable" {
			d.replyKey = "error"
		}
		return r.writeHandoff(ctx, tx, st, d)
	}
	return nil
}

/*
writeAnswer posts the answer and counts it on the conversation.
*/
func (r *Runner) writeAnswer(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	d.substantive = true
	if err := r.post(ctx, tx, st, d); err != nil {
		return err
	}
	_, err := models.New(tx).BumpConversationAiAnswer(ctx, models.BumpConversationAiAnswerParams{
		WorkspaceID: st.env.Conversation.WorkspaceID, ID: st.env.Conversation.ID,
		CostMicros: st.rec.costMicros, Now: r.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("count answer: %w", err)
	}
	return nil
}

/*
writeUnknown posts the no-answer. A second no-answer in a row, an
agent set to hand off, or a channel with no button to offer a person
hands off; otherwise the widget offers a person with a button.
*/
func (r *Runner) writeUnknown(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	e := st.env
	streak, err := models.New(tx).CountConversationOutcomes(ctx, models.CountConversationOutcomesParams{
		WorkspaceID: e.Conversation.WorkspaceID, ConversationID: &e.Conversation.ID, Outcome: new(outcomeUnknown),
	})
	if err != nil {
		return fmt.Errorf("count unknown streak: %w", err)
	}

	available := e.availability() != "unavailable"
	switch {
	case available && streak > 0:
		d.outcome, d.handoffReason = outcomeHandoff, "unknown_streak"
		d.reply = r.reply(e, d) + "\n\n" + r.line(e, e.handoffKey())
		return r.writeHandoff(ctx, tx, st, d)
	case available && (e.Agent.UnknownBehavior == "handoff" || !offersButtons(e.ChannelType)):
		d.outcome, d.handoffReason = outcomeHandoff, "unknown"
		return r.writeHandoff(ctx, tx, st, d)
	}
	d.offerHandoff = available
	return r.post(ctx, tx, st, d)
}

/*
writeHandoff posts the handoff line and hands the conversation to the
team. When no human handoff is configured the conversation still
needs the team's attention, but the line offers no teammate.
*/
func (r *Runner) writeHandoff(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	e := st.env
	if err := r.post(ctx, tx, st, d); err != nil {
		return err
	}
	if err := r.routeHandoff(ctx, tx, st, d); err != nil {
		return err
	}
	now := r.Clock.Now()
	if err := models.New(tx).HandoffConversationAi(ctx, models.HandoffConversationAiParams{
		WorkspaceID: e.Conversation.WorkspaceID, ID: e.Conversation.ID,
		Reason: nilable(d.handoffReason), Now: &now,
	}); err != nil {
		return fmt.Errorf("hand off conversation: %w", err)
	}
	return r.writeNote(ctx, tx, st, d)
}

/*
post writes the decision's reply as the agent.
*/
func (r *Runner) post(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	e := st.env
	text := withConfirmation(r.reply(e, d), st, d)
	msg, conv, err := r.agentWriter(tx, e).Append(ctx, e.Conversation, conversations.NewMessage{
		Type: string(enums.MessageTypeMessage), Body: text, BodyFormat: new("markdown"),
		Metadata: r.metadata(st, d), Automatic: !d.substantive,
	})
	if err != nil {
		return fmt.Errorf("post %s reply: %w", d.outcome, err)
	}
	e.Conversation = conv
	d.answerID = &msg.ID
	return nil
}

/*
reply is the decision's text, or its default line.
*/
func (r *Runner) reply(e *env, d *decision) string {
	if d.reply != "" {
		return d.reply
	}
	return r.line(e, d.replyKey)
}

/*
metadata is what every agent reply carries under "ai" for the widget,
the channels and the trace: generated, the run, the outcome, the
sources it cites as title and url, whether a person is offered or on
the way, and the action calls it reports with any confirmation card.
*/
func (r *Runner) metadata(st *run, d *decision) json.RawMessage {
	ai := map[string]any{
		"run_id": st.rec.Run.ID.String(), "generated": true, "outcome": d.outcome,
		"model": strv(st.env.Agent.AnswerModel),
	}
	if len(d.citations) > 0 {
		sources := make([]map[string]any, 0, len(d.citations))
		for _, c := range d.citations {
			sources = append(sources, map[string]any{"n": c.N, "title": c.Title, "url": c.URL})
		}
		ai["sources"] = sources
	}
	if d.offerHandoff {
		ai["actions"] = []string{"handoff"}
	}
	if d.handoffReason != "" {
		ai["handoff_reason"] = d.handoffReason
		ai["handoff_available"] = st.env.availability() != "unavailable"
	}
	actionMetadata(st, ai)
	data, err := json.Marshal(map[string]any{"ai": ai})
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}

/*
offersButtons reports whether a channel can show the customer a
"talk to a person" button under a reply.
*/
func offersButtons(channel string) bool {
	return channel == "widget" || channel == "api"
}
