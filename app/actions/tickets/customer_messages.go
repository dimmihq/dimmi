/*
|--------------------------------------------------------------------------
| Telling the Customer
|--------------------------------------------------------------------------
|
| Messages the ticket system writes to a customer go into the ticket's
| own conversation and out on its own channel: the messenger, the email
| thread or the Slack thread the customer used. A state notice waits
| two minutes and then sends only the state the ticket is in by then,
| and nothing when a teammate already wrote in that window or the state
| was told before.
|
*/

package tickets

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
defaultReopenDays is how long a resolved ticket reopens on a reply.
*/
const defaultReopenDays = 7

/*
isNoRows reports a query that matched nothing.
*/
func isNoRows(err error) bool {
	return errors.Is(err, database.ErrNotFound)
}

/*
sendInput is one automatic message: the conversation it goes to and
the text, with a key that keeps it from being sent twice.
*/
type sendInput struct {
	conversation ids.UUID
	body         string
	dedupe       string
}

/*
send writes the message as dimmi and dispatches it, returning the
message id, or nil when the customer cannot be reached.
*/
func (t *Tickets) send(
	ctx context.Context, cw *conversations.Writer, ws ids.UUID, in sendInput,
) (*ids.UUID, error) {
	c, err := cw.Q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: in.conversation})
	if err != nil {
		return nil, fmt.Errorf("find conversation to tell: %w", err)
	}

	route, err := t.route(ctx, cw.Q, c)
	if errors.Is(err, exceptions.ErrContactUnreachable) || errors.Is(err, exceptions.ErrChannelUnsupported) {
		return nil, nil //nolint:nilnil // nobody to tell is not a failure
	}

	if err != nil {
		return nil, err
	}

	msg := conversations.NewMessage{
		Type: string(enums.MessageTypeMessage), Body: in.body, BodyFormat: new("markdown"),
		DeliveryStatus: &route.Status, Automatic: true,
	}
	if in.dedupe != "" {
		msg.DedupeKey = &in.dedupe
	}

	sent, _, err := cw.AsSystem().Append(ctx, c, msg)
	if err != nil {
		return nil, fmt.Errorf("tell customer: %w", err)
	}

	if err := t.Delivery.Dispatch(ctx, cw.Tx, sent); err != nil {
		return nil, err
	}

	return &sent.ID, nil
}

/*
route finds how a message reaches the conversation's customer.
*/
func (t *Tickets) route(ctx context.Context, q *models.Queries, c models.Conversation) (conversations.Route, error) {
	conn, err := conversations.Connection(ctx, q, c)
	if err != nil {
		return conversations.Route{}, err
	}

	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: c.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return conversations.Route{}, fmt.Errorf("find contact: %w", err)
	}

	return t.Delivery.Plan(conn, contact)
}

/*
systemWriter returns a writer acting as dimmi in the transaction.
*/
func (t *Tickets) systemWriter(ctx context.Context, tx *database.Tx, ws ids.UUID) (*Writer, error) {
	cw := &conversations.Writer{
		Q: models.New(tx), Tx: tx, Events: t.Events, Now: t.Clock.Now(),
		Actor: conversations.Actor{Type: string(enums.AuthorTypeSystem)},
	}

	return t.writerFor(ctx, cw, ws)
}

/*
NotifyState sends a customer the state their ticket is in now, when it
tells the customer, was not told before, and no teammate wrote since
the move.
*/
func (t *Tickets) NotifyState(ctx context.Context, args jobs.NotifyTicketStateArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := t.systemWriter(ctx, tx, args.WorkspaceID)
		if err != nil {
			return err
		}

		ticket, err := w.lockRow(ctx, args.WorkspaceID, args.TicketID)
		if err != nil || ticket.DeletedAt != nil || ticket.ConversationID == nil {
			return nil //nolint:nilerr // a ticket gone needs no notice
		}

		state, ok := w.Setup.State(ticket.StateID)
		if !ok || !state.NotifyCustomer || sameState(ticket.NotifiedStateID, state.ID) {
			return nil
		}

		if replied, err := t.teamWroteSince(ctx, w, ticket); err != nil || replied {
			return err
		}

		return t.tellState(ctx, w, ticket, state)
	})
}

/*
sameState reports whether the customer was last told of the state.
*/
func sameState(told *ids.UUID, state ids.UUID) bool {
	return told != nil && *told == state
}

/*
teamWroteSince reports whether a teammate replied after the ticket
entered its state, which already tells the customer where it stands.
*/
func (t *Tickets) teamWroteSince(ctx context.Context, w *Writer, ticket models.Ticket) (bool, error) {
	c, err := w.Q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: ticket.WorkspaceID, ID: *ticket.ConversationID,
	})
	if err != nil {
		return false, fmt.Errorf("find ticket conversation: %w", err)
	}

	return c.LastReplyAt != nil && !c.LastReplyAt.Before(ticket.StateSince), nil
}

/*
tellState writes the state's customer label to the conversation and
remembers it was told.
*/
func (t *Tickets) tellState(ctx context.Context, w *Writer, ticket models.Ticket, state models.TicketState) error {
	body := fmt.Sprintf("**%s** · %s\n\n%s", Reference(ticket.Number), ticket.Title, state.CustomerLabel)
	key := "ticket-state:" + ticket.ID.String() + ":" + state.ID.String() + ":" + ticket.StateSince.String()

	if _, err := t.send(ctx, w.Writer, ticket.WorkspaceID, sendInput{
		conversation: *ticket.ConversationID, body: body, dedupe: key,
	}); err != nil {
		return err
	}

	err := w.Q.MarkTicketNotified(ctx, models.MarkTicketNotifiedParams{
		StateID: &state.ID, WorkspaceID: ticket.WorkspaceID, ID: ticket.ID,
	})
	if err != nil {
		return fmt.Errorf("remember ticket notice: %w", err)
	}

	return w.log(ctx, ticket, "customer_notified", map[string]any{
		"state": state.ID.String(), "label": state.CustomerLabel,
	})
}

/*
errorIs reports whether the error is the refusal.
*/
func errorIs(err, target error) bool {
	return err != nil && errors.Is(err, target)
}
