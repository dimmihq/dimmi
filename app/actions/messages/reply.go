/*
|--------------------------------------------------------------------------
| Reply To Conversation
|--------------------------------------------------------------------------
|
| A teammate's reply reaches the customer through the conversation's
| channel. Before it is written, the reply is checked against what
| happened while it was typed: a reply that did not see the newest
| message is refused with what arrived, so nobody answers a question
| that changed. A retried request carrying the same client message id
| finds the reply it already wrote instead of sending twice.
|
| Replying takes a conversation nobody holds, when the inbox and the
| replier's own rule say so. Afterwards the conversation stays where it is, or opens,
| closes or sleeps, as the reply asks; a closed conversation cannot go
| straight to sleep, and then nothing of the reply is kept. A
| conversation marked as spam takes no replies.
|
*/

package messages

import (
	"context"
	"errors"
	"fmt"
	"time"

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
What a conversation does after a reply.
*/
const (
	ThenKeep   = "keep"
	ThenOpen   = "open"
	ThenClose  = "close"
	ThenSnooze = "snooze"
)

/*
ReplyInput is who replies to which conversation, and the reply.
ClientMessageID makes a retried request find its first reply.
ExpectedLastMessageID is the newest message the replier saw. Then is
one of the Then values, with Snooze saying until when. Delay holds the
reply for the caller's undo window on a channel that cannot take it
back. MutationID is the id the client gave the change, empty when it
gave none.
*/
type ReplyInput struct {
	Membership            authz.Membership
	Ref                   conversations.Ref
	Body                  string
	ClientMessageID       *string
	ExpectedLastMessageID *ids.UUID
	Then                  string
	Snooze                SnoozeInput
	Delay                 bool
	MutationID            string
}

/*
SnoozeInput is when a snooze ends: a time, a preset, or the key of one
of the conversation's snooze options.
*/
type SnoozeInput struct {
	Until  *time.Time
	Preset string
	Option string
}

/*
Written is an entry a request wrote, the conversation as it now is, and
whether the request was a retry that found the entry already written.
*/
type Written struct {
	Message      models.Message
	Conversation conversations.Loaded
	Author       conversations.Member
	Replayed     bool
}

/*
ReplyToConversation sends a teammate's reply to the customer.

Rules:
  - The caller needs conversations:write.
  - Conversations marked as spam take no replies.
  - The newest message must be the one the replier saw, when they say.
  - The same client message id returns the reply already written.
  - A conversation nobody holds becomes the replier's, as
    conversations.AssignOnReply decides.

Events: [appevents.MessageCreated], [appevents.ConversationAssigned]
and the status event the reply asked for.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrConversationSpam], [exceptions.ErrNewMessagesSince],
[exceptions.ErrChannelUnsupported], [exceptions.ErrContactUnreachable],
[exceptions.ErrInvalidTransition], [exceptions.ErrSnoozeInPast],
[exceptions.ErrSnoozeTooFar], or a wrapped database error.
*/
type ReplyToConversation struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Clock    clock.Clock
	Delivery conversations.Delivery
}

/*
Handle writes the reply, hands it to its channel and returns it.
*/
func (a *ReplyToConversation) Handle(ctx context.Context, in ReplyInput) (Written, error) {
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

		key := dedupeKey("reply", s.c, in.Membership, in.ClientMessageID)
		if out, err = s.replay(ctx, key); err != nil || out.Replayed {
			return err
		}

		if s.c.Spam {
			return exceptions.ErrConversationSpam
		}

		if err := s.guard(ctx, in.ExpectedLastMessageID); err != nil {
			return err
		}

		if err := a.write(ctx, s, in, key); err != nil {
			return err
		}

		out, err = s.written(ctx)

		return err
	})

	return out, err
}

/*
write writes the reply, holds it for the undo window when asked, hands
it to its channel, ends the caller's reply draft, takes the
conversation and applies what comes after.
*/
func (a *ReplyToConversation) write(ctx context.Context, s *session, in ReplyInput, key *string) error {
	route, err := a.route(ctx, s)
	if err != nil {
		return err
	}

	before, window := stateOf(s.c), time.Duration(0)
	if in.Delay {
		window = undoWindow(s.m, route)
	}

	if s.msg, s.c, err = s.w.Append(ctx, s.c, conversations.NewMessage{
		Type: string(enums.MessageTypeMessage), Body: in.Body, DeliveryStatus: &route.Status, DedupeKey: key,
	}); err != nil {
		return err
	}

	if window > 0 {
		if err := s.hold(ctx, window, before); err != nil {
			return err
		}
	}

	if err := a.Delivery.Dispatch(ctx, s.w.Tx, s.msg); err != nil {
		return err
	}

	if err := s.consume(ctx, conversations.DraftReply); err != nil {
		return err
	}

	if s.c, err = s.w.TakeOnReply(ctx, s.c, s.m.Member); err != nil {
		return err
	}

	if err := s.then(ctx, in); err != nil || window == 0 {
		return err
	}

	return s.settle(ctx, before)
}

/*
route finds how the reply travels to the conversation's customer.
*/
func (a *ReplyToConversation) route(ctx context.Context, s *session) (conversations.Route, error) {
	conn, err := conversations.Connection(ctx, s.w.Q, s.c)
	if err != nil {
		return conversations.Route{}, err
	}

	contact, err := s.w.Q.FindContact(ctx, models.FindContactParams{WorkspaceID: s.c.WorkspaceID, ID: s.c.ContactID})
	if err != nil {
		return conversations.Route{}, fmt.Errorf("find contact: %w", err)
	}

	return a.Delivery.Plan(conn, contact)
}

/*
then moves the conversation as the reply asked.
*/
func (s *session) then(ctx context.Context, in ReplyInput) error {
	to := conversations.StatusChange{Reason: conversations.ReasonReply}

	switch in.Then {
	case ThenOpen:
		to.Status = string(enums.ConversationStatusOpen)
	case ThenClose:
		to.Status = string(enums.ConversationStatusClosed)
	case ThenSnooze:
		until, err := conversations.SnoozeEnd(ctx, s.w, s.c, conversations.SnoozeWhen{
			Until: in.Snooze.Until, Preset: in.Snooze.Preset, Option: in.Snooze.Option,
			Location: conversations.Location(s.w.Timezone, s.m.Workspace),
		})
		if err != nil {
			return err
		}

		to.Status, to.Until = string(enums.ConversationStatusSnoozed), until
	default:
		c, err := s.w.WakeIf(ctx, s.m.Workspace, s.c, conversations.ReasonMemberReply)
		s.c = c

		return err
	}

	c, _, err := s.w.ChangeStatus(ctx, s.c, to)
	s.c = c

	return err
}

/*
guard refuses a reply that did not see the newest message, listing what
arrived since.
*/
func (s *session) guard(ctx context.Context, expected *ids.UUID) error {
	if expected == nil {
		return nil
	}

	latest, err := s.w.Q.LatestConversationMessage(ctx, models.LatestConversationMessageParams{
		WorkspaceID: s.c.WorkspaceID, ConversationID: s.c.ID,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && latest.ID == *expected) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find latest message: %w", err)
	}

	since, err := s.w.Q.MessagesAfter(ctx, models.MessagesAfterParams{
		WorkspaceID: s.c.WorkspaceID, ConversationID: s.c.ID, AfterID: *expected,
	})
	if err != nil {
		return fmt.Errorf("find new messages: %w", err)
	}

	return exceptions.ErrNewMessagesSince.With("data", s.summaries(ctx, since))
}
