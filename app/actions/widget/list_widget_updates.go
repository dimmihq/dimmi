/*
|--------------------------------------------------------------------------
| List Widget Updates
|--------------------------------------------------------------------------
|
| Some networks block or buffer event streams, so a messenger falls back to
| asking every few seconds what changed. The answer comes from the database,
| not the stream's short history, so a visitor who was away for hours still
| catches up. A change is stamped when its write starts but seen only once
| it commits, so each poll's cursor trails the clock: changes that commit
| late still land in the next poll, and the messenger drops the ones it
| already has. It holds the same events the stream carries for stored
| changes; typing is never in it, and neither is anything a visitor may not
| see. A page holds at most a hundred changes and says when more wait.
|
*/

package widget

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxUpdates is the most changes one poll returns, and pollOverlap how far
each poll's cursor trails the clock, longer than any write takes to
commit.
*/
const (
	maxUpdates  = 100
	pollOverlap = 10 * time.Second
)

/*
Update is one change a polling messenger learns about: a message, or a
conversation that closed or otherwise changed.
*/
type Update struct {
	Event        string
	Message      *VisibleMessage
	Conversation *models.Conversation
}

/*
UpdatePage is a poll's changes, where the next poll starts, and whether
more changes wait already.
*/
type UpdatePage struct {
	Items   []Update
	Next    Cursor
	HasMore bool
}

/*
ListWidgetUpdates answers polling messengers.
*/
type ListWidgetUpdates struct {
	DB    *database.DB
	Clock clock.Clock
	Read  *ListWidgetConversations
}

/*
Handle returns what changed for the visitor after the cursor; a first poll,
with no cursor, starts from now.
*/
func (a *ListWidgetUpdates) Handle(ctx context.Context, auth Authenticated, since *Cursor) (UpdatePage, error) {
	now := a.Clock.Now()
	if since == nil {
		return UpdatePage{Items: []Update{}, Next: Cursor{At: now, ID: lastID()}}, nil
	}

	var out UpdatePage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		s, widget := auth.Session, models.PublicID(models.PrefixWidget, auth.Resolved.Widget.ID)

		messages, err := q.ListWidgetMessageChanges(ctx, models.ListWidgetMessageChangesParams{
			WorkspaceID: s.WorkspaceID, ContactID: s.ContactID, WidgetID: widget, InboxID: auth.Resolved.InboxID,
			SinceAt: since.At, SinceID: since.ID, RowLimit: maxUpdates + 1,
		})
		if err != nil {
			return fmt.Errorf("list widget message changes: %w", err)
		}

		out.HasMore = len(messages) > maxUpdates
		if out.HasMore {
			messages = messages[:maxUpdates]
		}

		until := now
		out.Next = trailing(*since, now)
		if out.HasMore {
			last := messages[len(messages)-1]
			out.Next, until = Cursor{At: last.UpdatedAt, ID: last.ID}, last.UpdatedAt
		}

		convs, err := q.ListWidgetConversationChanges(ctx, models.ListWidgetConversationChangesParams{
			WorkspaceID: s.WorkspaceID, ContactID: s.ContactID, WidgetID: widget, InboxID: auth.Resolved.InboxID,
			SinceAt: since.At, UntilAt: until, RowLimit: maxUpdates,
		})
		if err != nil {
			return fmt.Errorf("list widget conversation changes: %w", err)
		}

		found := changes{ws: s.WorkspaceID, messages: messages, convs: convs, since: since.At}
		out.Items, err = a.updates(ctx, q, found)

		return err
	})

	return out, err
}

/*
updates names each change by the event the stream would have carried.
*/
func (a *ListWidgetUpdates) updates(ctx context.Context, q *models.Queries, c changes) ([]Update, error) {
	messages, convs, since := c.messages, c.convs, c.since

	visible, err := a.Read.visible(ctx, q, c.ws, messages)
	if err != nil {
		return nil, err
	}

	out := make([]Update, 0, len(visible)+len(convs))

	for i := range visible {
		m := visible[i].Message

		event := "message.updated"

		switch {
		case m.DeletedAt != nil:
			event = "message.deleted"
		case m.CreatedAt.After(since):
			event = "message.created"
		}

		out = append(out, Update{Event: event, Message: &visible[i]})
	}

	for i := range convs {
		event := "conversation.updated"
		if convs[i].Status == string(enums.ConversationStatusClosed) {
			event = "conversation.closed"
		}

		out = append(out, Update{Event: event, Conversation: &convs[i]})
	}

	return out, nil
}

/*
trailing returns where the next poll starts when this one returned
everything: the overlap before now, but never before where this poll
started, so the cursor only moves forward.
*/
func trailing(since Cursor, now time.Time) Cursor {
	at := now.Add(-pollOverlap)
	if at.Before(since.At) {
		return since
	}

	return Cursor{At: at, ID: lastID()}
}

/*
lastID sorts after every id, so a cursor at a moment covers everything that
changed at that moment.
*/
func lastID() ids.UUID {
	last, _ := ids.Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	return last
}

/*
changes is what changed in a workspace's rows after a time.
*/
type changes struct {
	ws       ids.UUID
	messages []models.Message
	convs    []models.Conversation
	since    time.Time
}
