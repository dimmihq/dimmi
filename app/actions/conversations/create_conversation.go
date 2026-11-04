/*
|--------------------------------------------------------------------------
| Create Conversation
|--------------------------------------------------------------------------
|
| A conversation starts one of two ways through the API. A developer
| relays what a customer wrote, on an inbox's API channel; the
| conversation then waits for the team, held by nobody unless the
| request names someone. Or a teammate reaches out first, on a channel
| that lets the business start threads; the conversation is then the
| teammate's own unless the request hands it to someone else or nobody.
|
| The customer is named by contact id, external id or email address, and
| added when they are new. Numbers are handed out per workspace in the
| same transaction, so they never repeat.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateConversationInput is who starts which conversation. ByContact
says the customer wrote the first message, which then needs InboxID or
an api ConnectionID; otherwise the caller wrote it and ConnectionID is
required. AssignSet says Assignee was sent, nil meaning nobody.
MutationID is the id the client gave the change, empty when it gave
none.
*/
type CreateConversationInput struct {
	Membership   authz.Membership
	ByContact    bool
	InboxID      *ids.UUID
	ConnectionID *ids.UUID
	Contact      contacts.Ref
	Subject      *string
	Body         string
	AssignSet    bool
	Assignee     *ids.UUID
	MutationID   string
}

/*
CreateConversation starts a conversation with its first message.

Rules:
  - The caller needs conversations:write.
  - A customer's message goes through an api channel; a teammate's only
    through a channel that lets the business start threads.
  - A teammate's conversation is theirs unless an assignee is sent.

Events: [appevents.ConversationCreated], [appevents.MessageCreated]
and [appevents.ConversationAssigned] when someone holds it.

Errors: [exceptions.ErrForbidden], [exceptions.ErrInboxNotFound],
[exceptions.ErrNoAPIChannel], [exceptions.ErrChannelNotAPI],
[exceptions.ErrChannelCannotInitiate], [exceptions.ErrChannelUnsupported],
[exceptions.ErrContactUnreachable], [exceptions.ErrContactKeyRequired],
[exceptions.ErrMemberInactive], or a wrapped database error.
*/
type CreateConversation struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Clock    clock.Clock
	Delivery Delivery
}

/*
Handle starts the conversation and returns it.
*/
func (a *CreateConversation) Handle(ctx context.Context, in CreateConversationInput) (Loaded, error) {
	if err := CanWrite(in.Membership); err != nil {
		return Loaded{}, err
	}

	var out Loaded

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := NewWriter(ctx, tx, Services{Events: a.Events, Clock: a.Clock}, in.Membership)
		if err != nil {
			return err
		}

		w.MutationID = in.MutationID

		c, err := a.start(ctx, w, in)
		if err != nil {
			return err
		}

		out, err = LoadOne(ctx, w.Q, in.Membership.Member.ID, c)

		return err
	})

	return out, err
}

/*
opening is a conversation about to start: the request, the channel it
starts on, the customer and how its first message travels.
*/
type opening struct {
	in      CreateConversationInput
	conn    models.ChannelConnection
	contact models.Contact
	route   Route
}

/*
start writes the conversation, its first message and its assignment.
*/
func (a *CreateConversation) start(
	ctx context.Context, w *Writer, in CreateConversationInput,
) (models.Conversation, error) {
	o, err := a.plan(ctx, w, in)
	if err != nil {
		return models.Conversation{}, err
	}

	c, err := o.insert(ctx, w)
	if err != nil {
		return c, err
	}

	if c, err = a.firstMessage(ctx, w, c, o); err != nil {
		return c, err
	}

	to := Assignment{Member: in.Assignee}
	if !in.AssignSet && !in.ByContact {
		to.Member = &in.Membership.Member.ID
	}

	c, _, err = w.Assign(ctx, c, to)

	return c, err
}

/*
plan finds the channel and the customer, and checks the conversation
may start there.
*/
func (a *CreateConversation) plan(ctx context.Context, w *Writer, in CreateConversationInput) (opening, error) {
	o := opening{in: in}
	ws := in.Membership.Workspace.ID

	var err error
	if o.conn, err = a.connection(ctx, w.Q, ws, in); err != nil {
		return o, err
	}

	ref := in.Contact
	ref.WorkspaceID = ws

	if o.contact, err = contacts.Resolve(ctx, w.Tx, w.Now, ref); err != nil {
		return o, err
	}

	if o.route, err = a.Delivery.Plan(o.conn, o.contact); err != nil {
		return o, err
	}

	if !in.ByContact && o.route.Capabilities.StartConversation == contracts.StartNever {
		return o, exceptions.ErrChannelCannotInitiate
	}

	return o, nil
}

/*
connection finds the channel the conversation starts on: the one named,
or the inbox's api channel.
*/
func (a *CreateConversation) connection(
	ctx context.Context, q *models.Queries, ws ids.UUID, in CreateConversationInput,
) (models.ChannelConnection, error) {
	if in.ConnectionID != nil {
		conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: ws, ID: *in.ConnectionID,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && conn.DeletedAt != nil) {
			return conn, exceptions.ErrChannelCannotInitiate
		}

		if err != nil {
			return conn, fmt.Errorf("find channel connection: %w", err)
		}

		if in.ByContact && conn.Type != "api" {
			return conn, exceptions.ErrChannelNotAPI
		}

		return conn, nil
	}

	if in.InboxID == nil {
		return models.ChannelConnection{}, exceptions.ErrInboxNotFound
	}

	w := Writer{Q: q}
	if err := w.checkInbox(ctx, ws, *in.InboxID); err != nil {
		return models.ChannelConnection{}, err
	}

	conn, err := q.FindInboxAPIConnection(ctx, models.FindInboxAPIConnectionParams{
		WorkspaceID: ws, InboxID: *in.InboxID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return conn, exceptions.ErrNoAPIChannel
	}

	if err != nil {
		return conn, fmt.Errorf("find api channel: %w", err)
	}

	return conn, nil
}

/*
insert writes the conversation with the workspace's next number.
*/
func (o opening) insert(ctx context.Context, w *Writer) (models.Conversation, error) {
	number, err := w.Q.AllocateConversationNumber(ctx, o.conn.WorkspaceID)
	if err != nil {
		return models.Conversation{}, fmt.Errorf("allocate conversation number: %w", err)
	}

	c, err := w.Q.CreateConversation(ctx, models.CreateConversationParams{
		WorkspaceID: o.conn.WorkspaceID, Number: number, InboxID: o.conn.InboxID, ChannelConnectionID: o.conn.ID,
		ContactID: o.contact.ID, Subject: o.in.Subject, Status: string(enums.ConversationStatusOpen),
		Source: json.RawMessage(`{}`), Now: w.Now,
	})
	if err != nil {
		return c, fmt.Errorf("create conversation: %w", err)
	}

	return c, nil
}

/*
firstMessage writes the opening message, by the customer or the
caller, and says the conversation started.
*/
func (a *CreateConversation) firstMessage(
	ctx context.Context, w *Writer, c models.Conversation, o opening,
) (models.Conversation, error) {
	author, initiatedBy := w, string(enums.AuthorTypeMember)
	msg := NewMessage{Type: string(enums.MessageTypeMessage), Body: o.in.Body, DeliveryStatus: &o.route.Status}

	if o.in.ByContact {
		author, initiatedBy, msg.DeliveryStatus = w.AsContact(o.contact), string(enums.AuthorTypeContact), nil
	}

	if err := w.Dispatch(ctx, appevents.ConversationCreated{
		Base: author.Base(c.WorkspaceID), ConversationRef: ref(c), ContactID: c.ContactID, InitiatedBy: initiatedBy,
	}); err != nil {
		return c, err
	}

	created, c, err := author.Append(ctx, c, msg)
	if err != nil {
		return c, err
	}

	return c, a.Delivery.Dispatch(ctx, w.Tx, created)
}
