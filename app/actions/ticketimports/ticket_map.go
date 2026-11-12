/*
|--------------------------------------------------------------------------
| Bringing A Ticket In
|--------------------------------------------------------------------------
|
| Each ticket comes in through the tickets writer, in a transaction of
| its own, as the import: created at the time it was created, in the
| state it started in, then moved through each later state at the time
| it moved, so its log reads as it happened and time-in-state reports
| count it. The writer is marked imported, so nothing reacts: no
| webhooks, workflows or customer notices. The requester is found by
| email or added; the teammate is the member with the same email. The
| conversation it came from ties to it when that was imported too. A
| ticket an earlier run brought in is found by its id there and left
| as it is.
|
*/

package ticketimports

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The refusals of one ticket, kept on the import.
*/
var (
	ErrUnknownType = fault.New(fault.Invalid, "ticket_import_unknown_type",
		"The ticket's type was not among the types the tool listed.")
	ErrUnknownState = fault.New(fault.Invalid, "ticket_import_unknown_state",
		"The ticket's state was not among the states the tool listed.")
	ErrNoRequester = fault.New(fault.Invalid, "ticket_import_no_requester",
		"The ticket has no requester to tie it to.")
)

/*
writer returns a ticket writer acting as the import at the time,
setting nothing off.
*/
func (r *run) writer(ctx context.Context, tx *database.Tx, at time.Time) (*tickets.Writer, error) {
	if at.IsZero() {
		at = r.importer.Clock.Now()
	}

	cw := &conversations.Writer{
		Q: models.New(tx), Tx: tx, Events: r.importer.Events, Now: at, Imported: true,
		Actor: conversations.Actor{Type: "integration", Name: strings.ToUpper(r.src.Name()[:1]) + r.src.Name()[1:]},
	}

	w, err := r.importer.Tickets.Wrap(ctx, cw, r.row.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("ticket import writer: %w", err)
	}

	return w, nil
}

/*
ticket brings one ticket in, reporting whether it made it.
*/
func (r *run) ticket(ctx context.Context, t Ticket) (models.Ticket, bool, error) {
	var (
		out     models.Ticket
		created bool
	)

	err := r.importer.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := r.writer(ctx, tx, t.CreatedAt)
		if err != nil {
			return err
		}

		in, err := r.input(ctx, w, t)
		if err != nil {
			return err
		}

		made, err := w.Create(ctx, r.row.WorkspaceID, in)
		if err != nil {
			return err
		}

		out, created = made.Ticket, made.New
		if created {
			if out, err = r.replay(ctx, w, out, t); err != nil {
				return err
			}
		}

		return r.record(ctx, w.Q, touched{kind: "ticket", external: t.ExternalID, id: out.ID, created: created})
	})

	return out, created, err
}

/*
input shapes the create: type, starting state, values, requester,
teammate and conversation.
*/
func (r *run) input(ctx context.Context, w *tickets.Writer, t Ticket) (tickets.CreateInput, error) {
	ty, ok := r.mapped.types[t.TypeID]
	if !ok {
		return tickets.CreateInput{}, ErrUnknownType
	}

	start, ok := r.mapped.states[startOf(t)]
	if !ok {
		return tickets.CreateInput{}, ErrUnknownState
	}

	title := cut(strings.TrimSpace(t.Title), 200)
	if title == "" {
		title = "Imported ticket " + t.ExternalID
	}

	in := tickets.CreateInput{
		TypeID: ty.ID, StateID: &start, Title: title, Description: cut(t.Description, 20000),
		Fields: fit(w.Setup.FieldsOf(ty.ID), t.Values), Priority: t.Priority, ExternalID: &t.ExternalID,
	}

	if !w.Setup.Allows(ty.ID, start) {
		in.StateID = nil
	}

	d := &draft{ty: ty, src: t, in: in}
	if err := r.people(ctx, w, d); err != nil {
		return in, err
	}

	return d.in, nil
}

/*
startOf returns the state the ticket was created in.
*/
func startOf(t Ticket) string {
	if len(t.History) > 0 {
		return t.History[0].StateID
	}

	return t.StateID
}

/*
draft is a create being shaped: the type, the other tool's ticket and
the input so far.
*/
type draft struct {
	ty  models.TicketType
	src Ticket
	in  tickets.CreateInput
}

/*
people finds the requester, the teammate and the conversation.
*/
func (r *run) people(ctx context.Context, w *tickets.Writer, d *draft) error {
	ws, t, in := r.row.WorkspaceID, d.src, &d.in

	if t.AssigneeEmail != "" {
		m, err := w.Q.FindWorkspaceMemberByEmail(ctx, models.FindWorkspaceMemberByEmailParams{
			WorkspaceID: ws, Email: t.AssigneeEmail,
		})
		if err == nil {
			in.AssigneeMember = &m.ID
		}
	}

	if t.Conversation != "" {
		c, err := w.Q.FindConversationByImportExternalID(ctx, models.FindConversationByImportExternalIDParams{
			WorkspaceID: ws, ExternalID: t.Conversation,
		})
		if err == nil && c.DeletedAt == nil && !r.isTicket(ctx, w, c.ID) {
			in.ConversationID = &c.ID

			return nil
		}
	}

	if t.RequesterEmail == "" && t.RequesterName == "" {
		if d.ty.Kind == tickets.KindCustomer {
			return ErrNoRequester
		}

		return nil
	}

	contact, err := r.requester(ctx, w.Q, t)
	if err != nil {
		return err
	}

	in.ContactID = &contact

	return nil
}

/*
isTicket reports whether the conversation is a ticket already.
*/
func (r *run) isTicket(ctx context.Context, w *tickets.Writer, id ids.UUID) bool {
	_, err := w.Q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: r.row.WorkspaceID, ConversationID: &id,
	})

	return err == nil
}

/*
requester finds the customer with the email, adding them when there is
none.
*/
func (r *run) requester(ctx context.Context, q *models.Queries, t Ticket) (ids.UUID, error) {
	email := strings.ToLower(strings.TrimSpace(t.RequesterEmail))
	if email != "" {
		c, err := q.FindImportContactByEmail(ctx, models.FindImportContactByEmailParams{
			WorkspaceID: r.row.WorkspaceID, Email: email,
		})
		if err == nil {
			return c.ID, nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return ids.Nil(), fmt.Errorf("find requester: %w", err)
		}
	}

	c, err := q.CreateContact(ctx, models.CreateContactParams{
		WorkspaceID: r.row.WorkspaceID, Kind: "user", Email: orNil(email), Name: orNil(t.RequesterName),
		CustomAttributes: []byte("{}"), Now: r.importer.Clock.Now(),
	})
	if err != nil {
		return ids.Nil(), fmt.Errorf("add requester: %w", err)
	}

	return c.ID, nil
}

/*
replay moves the new ticket through its later states at their times,
ending in the state it is in now.
*/
func (r *run) replay(ctx context.Context, w *tickets.Writer, t models.Ticket, src Ticket) (models.Ticket, error) {
	moves := src.History
	if len(moves) > 0 {
		moves = moves[1:]
	}

	if last := len(moves); (last == 0 && startOf(src) != src.StateID) ||
		(last > 0 && moves[last-1].StateID != src.StateID) {
		moves = append(moves, Move{At: src.UpdatedAt, StateID: src.StateID})
	}

	for _, m := range moves {
		id, ok := r.mapped.states[m.StateID]
		if !ok || id == t.StateID || !w.Setup.Allows(t.TypeID, id) {
			continue
		}

		if !m.At.IsZero() && m.At.After(w.Now) {
			w.Now = m.At
		}

		var err error
		if t, err = w.Apply(ctx, t, tickets.Change{StateID: &id}); err != nil {
			return t, err
		}
	}

	return t, nil
}

/*
orNil returns a pointer to the text, nil when it is empty.
*/
func orNil(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
