/*
|--------------------------------------------------------------------------
| Ticket Lifecycle
|--------------------------------------------------------------------------
|
| A customer ticket and its conversation move together. Closing the
| conversation resolves the ticket in its type's resolved state, so the
| fields a resolve needs are required there too. A teammate opening it
| again puts the ticket back in progress. When the customer writes:
| a ticket waiting on them goes back in progress; a resolved one reopens
| within its type's reopen window, and after it the conversation moves
| to a new follow-up ticket linked to the old one, which keeps its
| history. An inbox may turn every new conversation into a ticket.
|
| Tickets waiting on the customer resolve themselves after their type's
| number of days, the customer told a day before.
|
*/

package tickets

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Listen hears the conversation events tickets follow.
*/
func (t *Tickets) Listen(d *events.Dispatcher) {
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationClosed) error {
		return t.onClosed(ctx, e)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationReopened) error {
		return t.onReopened(ctx, e)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.MessageCreated) error {
		return t.onMessage(ctx, e)
	})
}

/*
withTicket runs the function on the conversation's live customer
ticket, locked, with a writer carrying the event's chain. A
conversation that is no ticket runs nothing.
*/
func (t *Tickets) withTicket(
	ctx context.Context, base appevents.Base, conversation ids.UUID,
	fn func(ctx context.Context, w *Writer, ticket models.Ticket) error,
) error {
	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := q.FindConversationTicket(ctx, models.FindConversationTicketParams{
			WorkspaceID: base.WorkspaceID, ConversationID: &conversation,
		})
		if isNoRows(err) || (err == nil && found.DeletedAt != nil) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find conversation ticket: %w", err)
		}

		w, err := t.systemWriter(ctx, tx, base.WorkspaceID)
		if err != nil {
			return err
		}

		w.Chain, w.MutationID = base.WorkflowChain, base.ClientMutationID
		if base.Actor.Type != "" {
			w.Actor = conversations.Actor{Type: base.Actor.Type, ID: base.Actor.ID}
		}

		locked, err := w.lock(ctx, base.WorkspaceID, found.ID)
		if err != nil {
			return err
		}

		return fn(ctx, w, locked)
	})
}

/*
onClosed resolves the ticket of a closed conversation.
*/
func (t *Tickets) onClosed(ctx context.Context, e appevents.ConversationClosed) error {
	return t.withTicket(ctx, e.Base, e.ConversationID, func(
		ctx context.Context, w *Writer, ticket models.Ticket,
	) error {
		if ticket.StateGroup == GroupResolved {
			return nil
		}

		st, ok := w.Setup.DefaultResolved(ticket.TypeID)
		if !ok {
			return nil
		}

		_, err := w.move(ctx, ticket, st)

		return err
	})
}

/*
onReopened puts the ticket of a conversation a teammate opened again
back in progress. A customer's message is handled where it arrives.
*/
func (t *Tickets) onReopened(ctx context.Context, e appevents.ConversationReopened) error {
	if e.Reason == conversations.ReasonCustomerMessage {
		return nil
	}

	return t.withTicket(ctx, e.Base, e.ConversationID, func(
		ctx context.Context, w *Writer, ticket models.Ticket,
	) error {
		if ticket.StateGroup != GroupResolved {
			return nil
		}

		return w.toProgress(ctx, ticket)
	})
}

/*
toProgress moves the ticket to its type's first in-progress state.
*/
func (w *Writer) toProgress(ctx context.Context, ticket models.Ticket) error {
	st, ok := w.Setup.FirstOf(ticket.TypeID, GroupInProgress)
	if !ok {
		return nil
	}

	_, err := w.move(ctx, ticket, st)

	return err
}

/*
onMessage applies the reopen rules to a customer's message, and makes
a new conversation a ticket when its inbox asks for it.
*/
func (t *Tickets) onMessage(ctx context.Context, e appevents.MessageCreated) error {
	if e.Type != string(enums.MessageTypeMessage) || e.AuthorType != string(enums.AuthorTypeContact) {
		return nil
	}

	if e.ConversationCreated {
		return t.inboxTicket(ctx, e)
	}

	return t.withTicket(ctx, e.Base, e.ConversationID, func(
		ctx context.Context, w *Writer, ticket models.Ticket,
	) error {
		if err := w.customerWrote(ctx, ticket); err != nil {
			return err
		}

		changes := map[string]appevents.TicketChange{"message_id": change(nil, e.MessageID)}

		return w.emit(ctx, appevents.TicketCustomerReplied{TicketChanged: w.changed(ticket, changes)})
	})
}

/*
customerWrote applies the reopen rules.
*/
func (w *Writer) customerWrote(ctx context.Context, ticket models.Ticket) error {
	switch ticket.StateGroup {
	case GroupWaitingCustomer:
		return w.toProgress(ctx, ticket)
	case GroupResolved:
		ty, _ := w.Setup.Type(ticket.TypeID)
		if ticket.ResolvedAt == nil || w.Now.Sub(*ticket.ResolvedAt) <= days(ty.ReopenDays) {
			return w.toProgress(ctx, ticket)
		}

		return w.followUp(ctx, ticket)
	}

	return nil
}

/*
days returns a number of days as a duration.
*/
func days(n int32) time.Duration {
	return time.Duration(n) * 24 * time.Hour
}

/*
followUp moves the conversation from a ticket resolved too long ago to
a new ticket of the same type, linked as its follow-up.
*/
func (w *Writer) followUp(ctx context.Context, old models.Ticket) error {
	conversation := old.ConversationID
	old.ConversationID = nil

	saved, err := w.save(ctx, old)
	if err != nil {
		return err
	}

	values := objectOf(saved.Fields)

	made, err := w.Create(ctx, old.WorkspaceID, CreateInput{
		TypeID: old.TypeID, Title: old.Title, ConversationID: conversation, Fields: values, InboxID: &old.InboxID,
		Links: nil,
	})
	if err != nil {
		return err
	}

	_, err = w.Link(ctx, saved, LinkInput{Kind: LinkFollowUp, TicketID: &made.Ticket.ID})

	return err
}

/*
inboxTicket makes a new conversation a ticket when its inbox turns
conversations into tickets of a type.
*/
func (t *Tickets) inboxTicket(ctx context.Context, e appevents.MessageCreated) error {
	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: e.WorkspaceID, ID: e.ConversationID,
		})
		if err != nil {
			return nil //nolint:nilerr // a conversation gone makes no ticket
		}

		typeID, err := q.FindInboxTicketType(ctx, models.FindInboxTicketTypeParams{
			WorkspaceID: e.WorkspaceID, ID: c.InboxID,
		})
		if err != nil || typeID == nil {
			return nil //nolint:nilerr // an inbox without a ticket type makes none
		}

		w, err := t.systemWriter(ctx, tx, e.WorkspaceID)
		if err != nil {
			return err
		}

		title := "Conversation #" + itoa(c.Number)
		if c.Subject != nil && *c.Subject != "" {
			title = cutText(*c.Subject, 200)
		}

		_, err = w.Create(ctx, e.WorkspaceID, CreateInput{
			TypeID: *typeID, Title: title, ConversationID: &c.ID, Customer: true,
		})
		if isTicketRefusal(err) {
			return nil
		}

		return err
	})
}

/*
isTicketRefusal reports a create refused for its input, which an
automatic create skips rather than failing the message.
*/
func isTicketRefusal(err error) bool {
	for _, refusal := range []error{
		ErrMissingFields, ErrTypeArchived, ErrTypeNotFound, ErrAlreadyTicket, ErrStateNotAllowed,
	} {
		if errorIs(err, refusal) {
			return true
		}
	}

	return false
}

/*
autoResolveBatch is how many tickets one sweep resolves.
*/
const autoResolveBatch = 200

/*
Sweep resolves tickets that waited on the customer for their type's
number of days, telling the customer a day ahead.
*/
func (t *Tickets) Sweep(ctx context.Context) error {
	var due []models.TicketsDueAutoResolveRow

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		due, err = models.New(tx).TicketsDueAutoResolve(ctx, models.TicketsDueAutoResolveParams{
			DueAt: t.Clock.Now(), RowLimit: autoResolveBatch,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("list tickets to resolve: %w", err)
	}

	for _, d := range due {
		if err := t.autoResolve(ctx, d.WorkspaceID, d.ID); err != nil {
			return err
		}
	}

	return nil
}

/*
autoResolve warns or resolves one ticket waiting on its customer.
*/
func (t *Tickets) autoResolve(ctx context.Context, ws, id ids.UUID) error {
	ctx = tenancy.With(ctx, ws.String())

	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.systemWriter(ctx, tx, ws)
		if err != nil {
			return err
		}

		ticket, err := w.lock(ctx, ws, id)
		if err != nil || ticket.StateGroup != GroupWaitingCustomer || ticket.DeletedAt != nil {
			return nil //nolint:nilerr // a ticket that moved on needs nothing
		}

		ty, _ := w.Setup.Type(ticket.TypeID)
		if ty.AutoResolveDays == nil {
			return nil
		}

		if w.Now.Sub(ticket.StateSince) < days(*ty.AutoResolveDays) {
			return w.warnAutoResolve(ctx, ticket, ty)
		}

		return w.resolveWaiting(ctx, ticket, ty)
	})
}

/*
warnAutoResolve tells the customer once that the ticket resolves
itself tomorrow.
*/
func (w *Writer) warnAutoResolve(ctx context.Context, ticket models.Ticket, ty models.TicketType) error {
	if ticket.AutoResolveWarnedAt != nil || ticket.ConversationID == nil {
		return nil
	}

	body := fmt.Sprintf("We haven't heard back about **%s**. We'll close this request tomorrow unless you reply.",
		ticket.Title)
	if _, err := w.Owner.send(ctx, w.Writer, ticket.WorkspaceID, sendInput{
		conversation: *ticket.ConversationID, body: body, dedupe: "ticket-auto-warn:" + ticket.ID.String(),
	}); err != nil {
		return err
	}

	_ = ty

	return w.Q.MarkTicketAutoResolveWarned(ctx, models.MarkTicketAutoResolveWarnedParams{
		Now: &w.Now, WorkspaceID: ticket.WorkspaceID, ID: ticket.ID,
	})
}

/*
resolveWaiting resolves the ticket, with the type's message when it
has one.
*/
func (w *Writer) resolveWaiting(ctx context.Context, ticket models.Ticket, ty models.TicketType) error {
	st, ok := w.Setup.DefaultResolved(ticket.TypeID)
	if !ok {
		return nil
	}

	if ty.AutoResolveMessage != nil && ticket.ConversationID != nil {
		if _, err := w.Owner.send(ctx, w.Writer, ticket.WorkspaceID, sendInput{
			conversation: *ticket.ConversationID, body: *ty.AutoResolveMessage,
			dedupe: "ticket-auto-resolve:" + ticket.ID.String(),
		}); err != nil {
			return err
		}
	}

	if _, err := w.move(ctx, ticket, st); err != nil && !errorIs(err, ErrMissingFields) {
		return err
	}

	return w.log(ctx, ticket, "auto_resolved", map[string]any{"days": *ty.AutoResolveDays})
}
