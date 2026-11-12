/*
|--------------------------------------------------------------------------
| History and Notes
|--------------------------------------------------------------------------
|
| A ticket's history is its event log, read oldest first behind a
| cursor. The workspace's whole log is an export cursor too: a client
| that remembers the last event id it read never misses a change, the
| moves to the trash included. Notes are internal; a customer ticket's
| note also lands in its conversation, where the team works.
|
*/

package tickets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrNoteInvalid refuses an empty or oversized note.
*/
var ErrNoteInvalid = fault.New(fault.Invalid, "note_invalid", "Notes are 1 to 20000 characters.").OnField("body")

/*
Note logs an internal note and, for a customer ticket, adds it to the
conversation.
*/
func (w *Writer) Note(ctx context.Context, t models.Ticket, body string) (models.TicketEvent, error) {
	body = strings.TrimSpace(body)
	if n := len([]rune(body)); n < 1 || n > 20000 {
		return models.TicketEvent{}, ErrNoteInvalid
	}

	data := map[string]any{"body": body, "actor_name": w.Actor.Name}

	ev, err := w.Q.CreateTicketEvent(ctx, models.CreateTicketEventParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID, Kind: "note", ActorType: w.actorType(), ActorID: w.actorID(),
		Data: rawObject(data), Now: w.Now,
	})
	if err != nil {
		return ev, fmt.Errorf("log ticket note: %w", err)
	}

	if t.ConversationID != nil {
		c, err := w.Q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: t.WorkspaceID, ID: *t.ConversationID,
		})
		if err == nil && c.DeletedAt == nil {
			if _, err := w.AddNote(ctx, c, body); err != nil {
				return ev, err
			}
		}
	}

	changes := map[string]appevents.TicketChange{"note": change(nil, body)}

	return ev, w.emit(ctx, appevents.TicketNoteCreated{TicketChanged: w.changed(t, changes)})
}

/*
EventPage is a page of history and the cursor of the next, nil at the
end.
*/
type EventPage struct {
	Events []models.TicketEvent
	Next   *ids.UUID
}

/*
History returns a ticket's history after the cursor.
*/
func (t *Tickets) History(ctx context.Context, m authz.Membership, ref Ref, after *ids.UUID) (EventPage, error) {
	if err := canRead(m); err != nil {
		return EventPage{}, err
	}

	var out EventPage

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := findAny(ctx, q, m, ref)
		if err != nil {
			return err
		}

		rows, err := q.ListTicketEvents(ctx, models.ListTicketEventsParams{
			WorkspaceID: m.Workspace.ID, TicketID: found.ID, After: after, RowLimit: maxPage + 1,
		})
		if err != nil {
			return fmt.Errorf("list ticket events: %w", err)
		}

		out = pageEvents(rows)

		return nil
	})

	return out, err
}

/*
pageEvents cuts a page and finds its cursor.
*/
func pageEvents(rows []models.TicketEvent) EventPage {
	out := EventPage{Events: rows}
	if len(rows) > maxPage {
		out.Events = rows[:maxPage]
		out.Next = &out.Events[maxPage-1].ID
	}

	return out
}

/*
ExportEvent is one change of the workspace's log with its ticket's
number.
*/
type ExportEvent struct {
	models.TicketEvent
	Number int64
}

/*
ExportPage is a page of the workspace's log and the next cursor.
*/
type ExportPage struct {
	Events []ExportEvent
	Next   *ids.UUID
}

/*
Export returns the workspace's ticket changes after the cursor, or
since the time, oldest first. Only members who read every ticket may
follow it.
*/
func (t *Tickets) Export(
	ctx context.Context, m authz.Membership, after *ids.UUID, since *time.Time,
) (ExportPage, error) {
	if err := canRead(m); err != nil {
		return ExportPage{}, err
	}

	if m.ConversationScope().Restricted() {
		return ExportPage{}, errForbidden()
	}

	var out ExportPage

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListWorkspaceTicketEvents(ctx, models.ListWorkspaceTicketEventsParams{
			WorkspaceID: m.Workspace.ID, After: after, Since: since, RowLimit: maxPage + 1,
		})
		if err != nil {
			return fmt.Errorf("export ticket events: %w", err)
		}

		for _, r := range rows {
			out.Events = append(out.Events, ExportEvent{TicketEvent: models.TicketEvent{
				ID: r.ID, WorkspaceID: r.WorkspaceID, TicketID: r.TicketID, Kind: r.Kind, ActorType: r.ActorType,
				ActorID: r.ActorID, Data: r.Data, CreatedAt: r.CreatedAt,
			}, Number: r.TicketNumber})
		}

		if len(out.Events) > maxPage {
			out.Events = out.Events[:maxPage]
			out.Next = &out.Events[maxPage-1].ID
		}

		return nil
	})

	return out, err
}
