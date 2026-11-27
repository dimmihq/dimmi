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
	return r.w.TicketWriter(ctx, tx, at)
}

/*
TicketWriter returns a ticket writer acting as the import at the time,
setting nothing off.
*/
func (w *Writer) TicketWriter(ctx context.Context, tx *database.Tx, at time.Time) (*tickets.Writer, error) {
	if at.IsZero() {
		at = w.Clock.Now()
	}

	cw := &conversations.Writer{
		Q: models.New(tx), Tx: tx, Events: w.Events, Now: at, Imported: true,
		Actor: conversations.Actor{Type: "integration", Name: w.Actor},
	}
	if w.ActorID != nil {
		cw.Actor = conversations.Actor{Type: "member", ID: *w.ActorID, Name: w.Actor}
	}

	tw, err := w.Tickets.Wrap(ctx, cw, w.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("ticket import writer: %w", err)
	}

	return tw, nil
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
		var err error
		out, created, err = r.w.Write(ctx, tx, t)

		return err
	})

	return out, created, err
}

/*
Write brings one ticket in inside the transaction, through the mapping,
reporting whether it made it; a ticket already in with the same id is
returned as it is.
*/
func (w *Writer) Write(ctx context.Context, tx *database.Tx, t Ticket) (models.Ticket, bool, error) {
	w.ensureMapping()

	tw, err := w.TicketWriter(ctx, tx, t.CreatedAt)
	if err != nil {
		return models.Ticket{}, false, err
	}

	in, err := w.input(ctx, tw, t)
	if err != nil {
		return models.Ticket{}, false, err
	}

	made, err := tw.Create(ctx, w.WorkspaceID, in)
	if err != nil {
		return models.Ticket{}, false, err
	}

	out, created := made.Ticket, made.New
	if created {
		if out, err = w.replay(ctx, tw, out, t); err != nil {
			return out, created, err
		}
	}

	kept := touched{kind: "ticket", external: t.ExternalID, id: out.ID, created: created}

	return out, created, w.keep(ctx, tw.Q, kept)
}

/*
input shapes the create: type, starting state, values, requester,
teammate and conversation.
*/
func (w *Writer) input(ctx context.Context, tw *tickets.Writer, t Ticket) (tickets.CreateInput, error) {
	ty, ok := w.Mapping.Types[t.TypeID]
	if !ok {
		return tickets.CreateInput{}, ErrUnknownType
	}

	start, ok := w.Mapping.States[startOf(t)]
	if !ok {
		return tickets.CreateInput{}, ErrUnknownState
	}

	title := cut(strings.TrimSpace(t.Title), 200)
	if title == "" {
		title = "Imported ticket " + t.ExternalID
	}

	in := tickets.CreateInput{
		TypeID: ty.ID, StateID: &start, Title: title, Description: cut(t.Description, 20000),
		Fields: fit(tw.Setup.FieldsOf(ty.ID), t.Values), Priority: t.Priority, ExternalID: &t.ExternalID,
	}

	if !tw.Setup.Allows(ty.ID, start) {
		in.StateID = nil
	}

	d := &draft{ty: ty, src: t, in: in}
	if err := w.people(ctx, tw, d); err != nil {
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
people finds the requester, the teammate and the conversation. Ids the
caller already resolved win over the emails and ids of the other tool.
*/
func (w *Writer) people(ctx context.Context, tw *tickets.Writer, d *draft) error {
	t, in := d.src, &d.in

	in.AssigneeMember = w.assignee(ctx, tw, t)

	if in.ConversationID = w.conversation(ctx, tw, t); in.ConversationID != nil {
		return nil
	}

	if t.ContactID != nil {
		in.ContactID = t.ContactID

		return nil
	}

	if t.RequesterEmail == "" && t.RequesterName == "" {
		if d.ty.Kind == tickets.KindCustomer {
			return ErrNoRequester
		}

		return nil
	}

	contact, err := w.requester(ctx, tw.Q, t)
	if err != nil {
		return err
	}

	in.ContactID = &contact

	return nil
}

/*
assignee returns the teammate the ticket was with: the one resolved
already, else the member with the email, nil for none.
*/
func (w *Writer) assignee(ctx context.Context, tw *tickets.Writer, t Ticket) *ids.UUID {
	if t.AssigneeID != nil || t.AssigneeEmail == "" {
		return t.AssigneeID
	}

	m, err := tw.Q.FindWorkspaceMemberByEmail(ctx, models.FindWorkspaceMemberByEmailParams{
		WorkspaceID: w.WorkspaceID, Email: t.AssigneeEmail,
	})
	if err != nil {
		return nil
	}

	return &m.ID
}

/*
conversation returns the conversation the ticket came from, when it was
brought in and is no ticket yet: the one resolved already, else the one
imported with its id, nil for none.
*/
func (w *Writer) conversation(ctx context.Context, tw *tickets.Writer, t Ticket) *ids.UUID {
	if t.ConversationID != nil && !w.isTicket(ctx, tw, *t.ConversationID) {
		return t.ConversationID
	}

	if t.Conversation == "" {
		return nil
	}

	c, err := tw.Q.FindConversationByImportExternalID(ctx, models.FindConversationByImportExternalIDParams{
		WorkspaceID: w.WorkspaceID, ExternalID: t.Conversation,
	})
	if err != nil || c.DeletedAt != nil || w.isTicket(ctx, tw, c.ID) {
		return nil
	}

	return &c.ID
}

/*
isTicket reports whether the conversation is a ticket already.
*/
func (w *Writer) isTicket(ctx context.Context, tw *tickets.Writer, id ids.UUID) bool {
	_, err := tw.Q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: w.WorkspaceID, ConversationID: &id,
	})

	return err == nil
}

/*
requester finds the customer with the email, adding them when there is
none.
*/
func (w *Writer) requester(ctx context.Context, q *models.Queries, t Ticket) (ids.UUID, error) {
	email := strings.ToLower(strings.TrimSpace(t.RequesterEmail))
	if email != "" {
		c, err := q.FindImportContactByEmail(ctx, models.FindImportContactByEmailParams{
			WorkspaceID: w.WorkspaceID, Email: email,
		})
		if err == nil {
			return c.ID, nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return ids.Nil(), fmt.Errorf("find requester: %w", err)
		}
	}

	c, err := q.CreateContact(ctx, models.CreateContactParams{
		WorkspaceID: w.WorkspaceID, Kind: "user", Email: orNil(email), Name: orNil(t.RequesterName),
		CustomAttributes: []byte("{}"), Now: w.Clock.Now(),
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
func (w *Writer) replay(ctx context.Context, tw *tickets.Writer, t models.Ticket, src Ticket) (models.Ticket, error) {
	moves := src.History
	if len(moves) > 0 {
		moves = moves[1:]
	}

	if last := len(moves); (last == 0 && startOf(src) != src.StateID) ||
		(last > 0 && moves[last-1].StateID != src.StateID) {
		moves = append(moves, Move{At: src.UpdatedAt, StateID: src.StateID})
	}

	for _, m := range moves {
		id, ok := w.Mapping.States[m.StateID]
		if !ok || id == t.StateID || !tw.Setup.Allows(t.TypeID, id) {
			continue
		}

		if !m.At.IsZero() && m.At.After(tw.Now) {
			tw.Now = m.At
		}

		var err error
		if t, err = tw.Apply(ctx, t, tickets.Change{StateID: &id}); err != nil {
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
