/*
|--------------------------------------------------------------------------
| Notes And Customer Messages
|--------------------------------------------------------------------------
|
| A note is for the team only: it never leaves the workspace, changes
| neither the status nor who holds the conversation, and is allowed on
| closed and spam conversations alike.
|
| A developer relays what a customer wrote on a conversation of the API
| channel, and only there. It reaches the team like any customer
| message: a closed conversation opens again and a sleeping one wakes.
| A dedupe key makes a relayed message count once however often it is
| sent.
|
*/

package messages

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NoteInput is who adds a note to which conversation, and the note.
ClientMessageID makes a retried request find its first note.
MutationID is the id the client gave the change, empty when it gave
none.
*/
type NoteInput struct {
	Membership      authz.Membership
	Ref             conversations.Ref
	Body            string
	ClientMessageID *string
	MutationID      string
}

/*
AddNote adds an internal note to a conversation.

Rules:
  - The caller needs conversations:write.
  - The same client message id returns the note already written.

Events: [appevents.MessageCreated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type AddNote struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	IDs    *ids.Registry
}

/*
Handle adds the note and returns it.
*/
func (a *AddNote) Handle(ctx context.Context, in NoteInput) (Written, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Written{}, err
	}

	var out Written

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		services := conversations.Services{Events: a.Events, Clock: a.Clock}

		s, err := open(ctx, tx, services, target{in.Membership, in.Ref, in.MutationID})
		if err != nil {
			return err
		}

		key := dedupeKey("note", s.c, in.Membership, in.ClientMessageID)
		if out, err = s.replay(ctx, key); err != nil || out.Replayed {
			return err
		}

		if err := s.note(ctx, a.IDs, in, key); err != nil {
			return err
		}

		out, err = s.written(ctx)

		return err
	})

	return out, err
}

/*
note writes the note, ends the caller's note draft and wakes a snoozed
conversation someone else holds.
*/
func (s *session) note(ctx context.Context, reg *ids.Registry, in NoteInput, key *string) error {
	m, err := s.resolve(ctx, reg, in.Body)
	if err != nil {
		return err
	}

	metadata, err := m.metadata()
	if err != nil {
		return err
	}

	if s.msg, s.c, err = s.w.Append(ctx, s.c, conversations.NewMessage{
		Type: string(enums.MessageTypeNote), Body: in.Body, DedupeKey: key, Metadata: metadata,
		Mentioned: m.ids(),
	}); err != nil {
		return err
	}

	if _, err := s.record(ctx, m); err != nil {
		return err
	}

	if err := s.consume(ctx, conversations.DraftNote); err != nil {
		return err
	}

	if s.c.AssigneeMemberID != nil && *s.c.AssigneeMemberID == in.Membership.Member.ID {
		return nil
	}

	s.c, err = s.w.WakeIf(ctx, in.Membership.Workspace, s.c, conversations.ReasonOthersNote)

	return err
}

/*
ContactMessageInput is who relays which customer message to which
conversation. DedupeKey, when given, makes the message count once.
MutationID is the id the client gave the change, empty when it gave
none.
*/
type ContactMessageInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	Body       string
	DedupeKey  *string
	MutationID string
}

/*
AddContactMessage relays a customer's message to a conversation of the
API channel.

Rules:
  - The caller needs conversations:write.
  - Only conversations on an api channel take relayed messages.
  - A closed conversation reopens and a snoozed one wakes.
  - The same dedupe key returns the message already relayed.

Events: [appevents.MessageCreated], and the reopening or waking.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrChannelNotAPI], or a wrapped database error.
*/
type AddContactMessage struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle relays the message and returns it.
*/
func (a *AddContactMessage) Handle(ctx context.Context, in ContactMessageInput) (Written, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Written{}, err
	}

	var out Written

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		services := conversations.Services{Events: a.Events, Clock: a.Clock}

		s, err := open(ctx, tx, services, target{in.Membership, in.Ref, in.MutationID})
		if err != nil {
			return err
		}

		var key *string
		if in.DedupeKey != nil {
			key = new("api:" + *in.DedupeKey)
		}

		if out, err = s.replay(ctx, key); err != nil || out.Replayed {
			return err
		}

		if err := s.relay(ctx, in.Body, key); err != nil {
			return err
		}

		out, err = s.written(ctx)

		return err
	})

	return out, err
}

/*
relay writes the message as the customer and wakes the conversation.
*/
func (s *session) relay(ctx context.Context, body string, key *string) error {
	conn, err := conversations.Connection(ctx, s.w.Q, s.c)
	if err != nil {
		return err
	}

	if conn.Type != "api" {
		return exceptions.ErrChannelNotAPI
	}

	contact, err := s.w.Q.FindContact(ctx, models.FindContactParams{WorkspaceID: s.c.WorkspaceID, ID: s.c.ContactID})
	if err != nil {
		return fmt.Errorf("find contact: %w", err)
	}

	customer := s.w.AsContact(contact)

	if s.msg, s.c, err = customer.Append(ctx, s.c, conversations.NewMessage{
		Type: string(enums.MessageTypeMessage), Body: body, DedupeKey: key,
	}); err != nil {
		return err
	}

	if s.c.Status == string(enums.ConversationStatusOpen) {
		return nil
	}

	s.c, _, err = customer.ChangeStatus(ctx, s.c, conversations.StatusChange{
		Status: string(enums.ConversationStatusOpen), Reason: conversations.ReasonCustomerMessage,
	})

	return err
}
