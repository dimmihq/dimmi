/*
|--------------------------------------------------------------------------
| Working Tickets
|--------------------------------------------------------------------------
|
| What a teammate, an API token or MCP does to tickets. Each call checks
| the caller's permission, finds the ticket on the caller's terms and
| hands the change to the writer, in one transaction.
|
*/

package tickets

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
write runs the function with a writer acting as the member.
*/
func (t *Tickets) write(ctx context.Context, m authz.Membership, fn func(ctx context.Context, w *Writer) error) error {
	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		cw, err := conversations.NewWriter(ctx, tx, t.services(), m)
		if err != nil {
			return err
		}

		w, err := t.writerFor(ctx, cw, m.Workspace.ID)
		if err != nil {
			return err
		}

		return fn(ctx, w)
	})
}

/*
Create makes a ticket for the member. A customer ticket given only a
contact starts its conversation first, with the description as the
customer's opening words.
*/
func (t *Tickets) Create(ctx context.Context, m authz.Membership, in CreateInput) (Created, error) {
	if err := canWrite(m); err != nil {
		return Created{}, err
	}

	var out Created

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		if in.ExternalID == nil || !t.knownExternal(ctx, w, m.Workspace.ID, *in.ExternalID) {
			if err := t.openConversation(ctx, w, m, &in); err != nil {
				return err
			}
		}

		var err error
		out, err = w.Create(ctx, m.Workspace.ID, in)

		return err
	})

	return out, err
}

/*
knownExternal reports whether a ticket already has the external id.
*/
func (t *Tickets) knownExternal(ctx context.Context, w *Writer, ws ids.UUID, external string) bool {
	_, err := w.Q.FindTicketByExternalID(ctx, models.FindTicketByExternalIDParams{
		WorkspaceID: ws, ExternalID: &external,
	})

	return err == nil
}

/*
openConversation starts the conversation of a customer ticket made with
only a contact.
*/
func (t *Tickets) openConversation(ctx context.Context, w *Writer, m authz.Membership, in *CreateInput) error {
	ty, ok := w.Setup.Type(in.TypeID)
	if !ok || ty.Kind != KindCustomer || in.ConversationID != nil || in.ContactID == nil {
		return nil
	}

	body := in.Description
	if body == "" {
		body = in.Title
	}

	inbox, err := w.inboxOf(ctx, m.Workspace.ID, planned{in: *in, kind: ty})
	if err != nil {
		return err
	}

	in.InboxID = &inbox

	loaded, err := t.Starter.Handle(ctx, conversations.CreateConversationInput{
		Membership: m, InboxID: &inbox, Contact: contacts.Ref{WorkspaceID: m.Workspace.ID, ID: in.ContactID},
		Subject: &in.Title, Body: body, ByContact: true,
	})
	if err != nil {
		return err
	}

	in.ConversationID = &loaded.Conversation.ID

	return nil
}

/*
Convert makes the conversation a customer ticket of the type.
*/
func (t *Tickets) Convert(
	ctx context.Context, m authz.Membership, conversation conversations.Ref, in CreateInput,
) (Created, error) {
	if err := canWrite(m); err != nil {
		return Created{}, err
	}

	if in.AIFill {
		proposed, err := t.Suggest(ctx, m, conversation, in.TypeID)
		if err != nil {
			return Created{}, err
		}

		in.Suggestions = proposed
	}

	var out Created

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		c, err := conversations.Find(ctx, w.Q, m, conversation)
		if err != nil {
			return err
		}

		in.ConversationID, in.ContactID = &c.ID, &c.ContactID
		if in.Title == "" && c.Subject != nil {
			in.Title = *c.Subject
		}

		if in.Title == "" {
			in.Title = "Conversation #" + itoa(c.Number)
		}

		out, err = w.Create(ctx, m.Workspace.ID, in)

		return err
	})

	return out, err
}

/*
Update changes a ticket the member may see.
*/
func (t *Tickets) Update(ctx context.Context, m authz.Membership, ref Ref, c Change) (models.Ticket, error) {
	if err := canWrite(m); err != nil {
		return models.Ticket{}, err
	}

	var out models.Ticket

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		found, err := find(ctx, w.Q, m, ref)
		if err != nil {
			return err
		}

		locked, err := w.lock(ctx, m.Workspace.ID, found.ID)
		if err != nil {
			return err
		}

		out, err = w.Apply(ctx, locked, c)

		return err
	})

	return out, err
}

/*
Escalate makes back-office work for a customer ticket.
*/
func (t *Tickets) Escalate(ctx context.Context, m authz.Membership, ref Ref, in EscalateInput) (models.Ticket, error) {
	if err := canWrite(m); err != nil {
		return models.Ticket{}, err
	}

	var out models.Ticket

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		found, err := find(ctx, w.Q, m, ref)
		if err != nil {
			return err
		}

		locked, err := w.lock(ctx, m.Workspace.ID, found.ID)
		if err != nil {
			return err
		}

		out, err = w.Escalate(ctx, locked, in)

		return err
	})

	return out, err
}

/*
Trash moves a ticket to the trash.
*/
func (t *Tickets) Trash(ctx context.Context, m authz.Membership, ref Ref) (models.Ticket, error) {
	return t.inTrash(ctx, m, ref, (*Writer).trash)
}

/*
Restore takes a ticket out of the trash.
*/
func (t *Tickets) Restore(ctx context.Context, m authz.Membership, ref Ref) (models.Ticket, error) {
	return t.inTrash(ctx, m, ref, (*Writer).restore)
}

/*
inTrash runs a trash move on a ticket the member may delete.
*/
func (t *Tickets) inTrash(
	ctx context.Context, m authz.Membership, ref Ref,
	move func(*Writer, context.Context, models.Ticket) (models.Ticket, error),
) (models.Ticket, error) {
	if !m.Can("tickets:delete") {
		return models.Ticket{}, errForbidden()
	}

	var out models.Ticket

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		found, err := findAny(ctx, w.Q, m, ref)
		if err != nil {
			return err
		}

		out, err = move(w, ctx, found)

		return err
	})

	return out, err
}

/*
trash moves the ticket to the trash and announces it.
*/
func (w *Writer) trash(ctx context.Context, t models.Ticket) (models.Ticket, error) {
	saved, err := w.Q.TrashTicket(ctx, models.TrashTicketParams{Now: &w.Now, WorkspaceID: t.WorkspaceID, ID: t.ID})
	if err != nil {
		return t, notFound(err)
	}

	if err := w.log(ctx, saved, "deleted", nil); err != nil {
		return saved, err
	}

	return saved, w.emit(ctx, appevents.TicketDeleted{TicketChanged: w.changed(saved, nil)})
}

/*
restore takes the ticket out of the trash and announces it.
*/
func (w *Writer) restore(ctx context.Context, t models.Ticket) (models.Ticket, error) {
	saved, err := w.Q.RestoreTicket(ctx, models.RestoreTicketParams{
		Now: w.Now, WorkspaceID: t.WorkspaceID, ID: t.ID,
	})
	if err != nil {
		return t, notFound(err)
	}

	if err := w.log(ctx, saved, "restored", nil); err != nil {
		return saved, err
	}

	return saved, w.emit(ctx, appevents.TicketRestored{TicketChanged: w.changed(saved, nil)})
}

/*
Show returns a ticket the member may see, in the trash too.
*/
func (t *Tickets) Show(ctx context.Context, m authz.Membership, ref Ref) (Detail, error) {
	if err := canRead(m); err != nil {
		return Detail{}, err
	}

	var out Detail

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := findAny(ctx, q, m, ref)
		if err != nil {
			return err
		}

		out, err = detailOf(ctx, q, found, t.Clock.Now())

		return err
	})

	return out, err
}

/*
Note leaves an internal note on the ticket; a customer ticket's note
also goes into its conversation.
*/
func (t *Tickets) Note(ctx context.Context, m authz.Membership, ref Ref, body string) (models.TicketEvent, error) {
	if err := canWrite(m); err != nil {
		return models.TicketEvent{}, err
	}

	var out models.TicketEvent

	err := t.write(ctx, m, func(ctx context.Context, w *Writer) error {
		found, err := find(ctx, w.Q, m, ref)
		if err != nil {
			return err
		}

		out, err = w.Note(ctx, found, body)

		return err
	})

	return out, err
}
