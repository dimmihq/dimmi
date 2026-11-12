/*
|--------------------------------------------------------------------------
| Building Facts
|--------------------------------------------------------------------------
|
| The builder works out the facts of a batch of conversations of one
| workspace in one transaction: it reads their rows, logs, messages,
| ratings and the inboxes' office hours with a handful of queries,
| computes each conversation's facts, and replaces what was stored.
| Builds of one workspace take turns, so two never interleave.
|
*/

package reports

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
Builder works out and stores conversation facts.
*/
type Builder struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Build works out the facts of the workspace's conversations again.
*/
func (b *Builder) Build(ctx context.Context, workspaceID ids.UUID, conversationIDs []ids.UUID) error {
	if len(conversationIDs) == 0 {
		return nil
	}

	ctx = tenancy.With(ctx, workspaceID.String())

	return b.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if err := tx.AdvisoryLock(ctx, "report_facts:"+workspaceID.String()); err != nil {
			return fmt.Errorf("lock facts: %w", err)
		}

		l := loader{q: q, ws: workspaceID, ids: conversationIDs}

		inputs, err := l.inputs(ctx, b.Clock.Now())
		if err != nil {
			return err
		}

		return l.store(ctx, inputs)
	})
}

/*
loader reads the rows related to a batch of conversations.
*/
type loader struct {
	q   *models.Queries
	ws  ids.UUID
	ids []ids.UUID
}

/*
store replaces the batch's facts with the ones worked out.
*/
func (l loader) store(ctx context.Context, inputs []Input) error {
	q := l.q

	var (
		rows    []FactRow
		replies []ReplyRow
		members []MemberRow
	)

	for _, in := range inputs {
		f := Compute(in)
		if !f.Counted {
			continue
		}

		rows = append(rows, f.Row)
		replies = append(replies, f.Replies...)
		members = append(members, f.Members...)
	}

	if err := q.DeleteReportFacts(ctx, models.DeleteReportFactsParams{WorkspaceID: l.ws, Ids: l.ids}); err != nil {
		return fmt.Errorf("delete facts: %w", err)
	}

	writes := []struct {
		rows any
		n    int
		fn   func(context.Context, json.RawMessage) error
	}{
		{rows, len(rows), q.InsertConversationFacts},
		{replies, len(replies), q.InsertReplyFacts},
		{members, len(members), q.InsertMemberFacts},
	}

	for _, w := range writes {
		if w.n == 0 {
			continue
		}

		data, err := json.Marshal(w.rows)
		if err != nil {
			return fmt.Errorf("encode facts: %w", err)
		}

		if err := w.fn(ctx, data); err != nil {
			return fmt.Errorf("write facts: %w", err)
		}
	}

	return nil
}

/*
inputs reads what the batch's facts are worked out from, one input per
conversation that still exists.
*/
func (l loader) inputs(ctx context.Context, now time.Time) ([]Input, error) {
	convs, err := l.q.ReportConversationInputs(ctx, models.ReportConversationInputsParams{
		WorkspaceID: l.ws, Ids: l.ids,
	})
	if err != nil {
		return nil, fmt.Errorf("read conversations: %w", err)
	}

	related, err := l.related(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Input, 0, len(convs))
	for _, c := range convs {
		out = append(out, Input{
			Conversation: c, Messages: related.messages[c.ID], Events: related.events[c.ID],
			Rating: related.ratings[c.ID], Helped: related.helped[c.ID], Schedule: related.schedules[c.InboxID],
			Now: now,
		})
	}

	return out, nil
}

/*
relatedRows is what the batch's conversations have, by conversation.
*/
type relatedRows struct {
	messages  map[ids.UUID][]models.ReportMessageInputsRow
	events    map[ids.UUID][]models.ReportEventInputsRow
	ratings   map[ids.UUID]*models.ReportRatingInputsRow
	helped    map[ids.UUID]bool
	schedules map[ids.UUID]*officehours.Schedule
}

/*
related reads the messages, log, ratings, AI feedback and office hours.
*/
func (l loader) related(ctx context.Context) (relatedRows, error) {
	out := relatedRows{
		messages: map[ids.UUID][]models.ReportMessageInputsRow{}, events: map[ids.UUID][]models.ReportEventInputsRow{},
		ratings: map[ids.UUID]*models.ReportRatingInputsRow{}, helped: map[ids.UUID]bool{},
	}

	msgs, err := l.q.ReportMessageInputs(ctx, models.ReportMessageInputsParams{WorkspaceID: l.ws, Ids: l.ids})
	if err != nil {
		return out, fmt.Errorf("read messages: %w", err)
	}

	for _, m := range msgs {
		out.messages[m.ConversationID] = append(out.messages[m.ConversationID], m)
	}

	events, err := l.q.ReportEventInputs(ctx, models.ReportEventInputsParams{WorkspaceID: l.ws, Ids: l.ids})
	if err != nil {
		return out, fmt.Errorf("read events: %w", err)
	}

	for _, e := range events {
		out.events[e.ConversationID] = append(out.events[e.ConversationID], e)
	}

	if err := l.outcomes(ctx, &out); err != nil {
		return out, err
	}

	out.schedules, err = Schedules(ctx, l.q, l.ws)

	return out, err
}

/*
outcomes reads the ratings and the AI feedback.
*/
func (l loader) outcomes(ctx context.Context, out *relatedRows) error {
	ratings, err := l.q.ReportRatingInputs(ctx, models.ReportRatingInputsParams{WorkspaceID: l.ws, Ids: l.ids})
	if err != nil {
		return fmt.Errorf("read ratings: %w", err)
	}

	for i := range ratings {
		out.ratings[ratings[i].ConversationID] = &ratings[i]
	}

	helped, err := l.q.ReportHelpedInputs(ctx, models.ReportHelpedInputsParams{WorkspaceID: l.ws, Ids: l.ids})
	if err != nil {
		return fmt.Errorf("read feedback: %w", err)
	}

	for _, id := range helped {
		out.helped[id] = true
	}

	return nil
}

/*
Schedules returns each inbox's office hours, nil for an inbox with none
or with hours that cannot be read.
*/
func Schedules(
	ctx context.Context, q *models.Queries, workspaceID ids.UUID,
) (map[ids.UUID]*officehours.Schedule, error) {
	rows, err := q.ReportInboxSchedules(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("read office hours: %w", err)
	}

	out := make(map[ids.UUID]*officehours.Schedule, len(rows))
	for _, r := range rows {
		if r.Timezone == nil {
			continue
		}

		if s, err := officehours.ParseStored(*r.Timezone, r.Weekly, r.Holidays); err == nil {
			out[r.InboxID] = s
		}
	}

	return out, nil
}
