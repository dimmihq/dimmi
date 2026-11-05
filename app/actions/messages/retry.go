/*
|--------------------------------------------------------------------------
| Retry Delivery
|--------------------------------------------------------------------------
|
| A reply that could not be delivered says so in the thread, with the
| reason, so nobody believes the customer has it. Once the cause is
| dealt with, any teammate who may write in the conversation can send it
| again: it goes back in line as pending and its channel tries once
| more. Only a failed reply can be sent again, so a reply that is on its
| way or arrived is never sent twice.
|
*/

package messages

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RetryInput is who sends which reply of which conversation again, and
the id the client gave the change.
*/
type RetryInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	MessageID  ids.UUID
	MutationID string
}

/*
RetryDelivery sends a reply that was not delivered again.

Rules:
  - The caller needs conversations:write.
  - Only a reply whose delivery failed can be sent again.

Events: [appevents.MessageDeliveryUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMessageNotFailed], or a wrapped database error.
*/
type RetryDelivery struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Clock    clock.Clock
	Delivery conversations.Delivery
}

/*
Handle puts the reply back in line and returns it.
*/
func (a *RetryDelivery) Handle(ctx context.Context, in RetryInput) (Written, error) {
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

		if s.msg, err = a.requeue(ctx, s, in.MessageID); err != nil {
			return err
		}

		if out, err = s.written(ctx); err != nil {
			return err
		}

		out.Author, err = authorOf(ctx, s, out.Author)

		return err
	})

	return out, err
}

/*
requeue marks the failed reply pending, hands it to its channel and
tells open screens.
*/
func (a *RetryDelivery) requeue(ctx context.Context, s *session, id ids.UUID) (models.Message, error) {
	msg, err := s.w.Q.RetryMessageDelivery(ctx, models.RetryMessageDeliveryParams{
		Now: s.w.Now, WorkspaceID: s.c.WorkspaceID, ConversationID: s.c.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return msg, notFailed(ctx, s, id)
	}

	if err != nil {
		return msg, fmt.Errorf("retry delivery: %w", err)
	}

	if err := a.Delivery.Dispatch(ctx, s.w.Tx, msg); err != nil {
		return msg, err
	}

	return msg, s.w.Dispatch(ctx, appevents.MessageDeliveryUpdated{
		Base: s.w.Base(s.c.WorkspaceID), Status: string(enums.DeliveryStatusPending),
		MessageRef: appevents.MessageRef{
			ConversationRef: appevents.ConversationRef{ConversationID: s.c.ID}, MessageID: msg.ID,
		},
	})
}

/*
notFailed tells a message that is not in the conversation from one
that did not fail.
*/
func notFailed(ctx context.Context, s *session, id ids.UUID) error {
	msg, err := s.w.Q.FindMessage(ctx, models.FindMessageParams{
		WorkspaceID: s.c.WorkspaceID, ConversationID: s.c.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && msg.DeletedAt != nil) {
		return exceptions.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("find message: %w", err)
	}

	return exceptions.ErrMessageNotFailed
}

/*
authorOf returns the teammate who wrote the reply, which may not be the
one sending it again.
*/
func authorOf(ctx context.Context, s *session, caller conversations.Member) (conversations.Member, error) {
	author := s.msg.AuthorID
	if author == nil || s.msg.AuthorType != string(enums.AuthorTypeMember) || *author == caller.Member.ID {
		return caller, nil
	}

	members, err := conversations.Members(ctx, s.w.Q, s.c.WorkspaceID, []ids.UUID{*s.msg.AuthorID})
	if err != nil {
		return caller, fmt.Errorf("load author: %w", err)
	}

	return members[*s.msg.AuthorID], nil
}
