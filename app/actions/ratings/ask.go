/*
|--------------------------------------------------------------------------
| Asking For A Rating
|--------------------------------------------------------------------------
|
| A close asks when the workspace turned asking on, or when a workflow's
| close step asked. Conversations the agent closed ask only when the
| workspace says so, test conversations never do, and only the
| messenger and email channels can carry an ask. The ask is due straight
| away or after the workspace's delay, and goes out only if the
| conversation is still the closed one it was: a customer who wrote
| again in the meantime is not asked to rate a conversation that is
| open.
|
*/

package ratings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Ask opens the rating ask of a close, when the close asks for one, and
sends it at once when no delay applies.
*/
func (r *Ratings) Ask(ctx context.Context, args jobs.AskConversationRatingArgs) error {
	return r.inWorkspace(ctx, args.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		c, ok, err := findClosed(ctx, q, args)
		if err != nil || !ok {
			return err
		}

		s, err := load(ctx, q, args.WorkspaceID)
		if err != nil {
			return err
		}

		channel, ok, err := r.channelOf(ctx, q, s, args)
		if err != nil || !ok {
			return err
		}

		now := r.Clock.Now()
		row, err := q.CreateConversationRating(ctx, models.CreateConversationRatingParams{
			WorkspaceID: c.WorkspaceID, ConversationID: c.ID, ContactID: c.ContactID, MemberID: c.AssigneeMemberID,
			Channel: channel, ClosedAt: *c.ClosedAt, AskAt: now.Add(time.Duration(s.DelayMinutes) * time.Minute),
			Now: now,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("open rating ask: %w", err)
		}

		return r.schedule(ctx, txn{tx: tx, q: q}, row, s.DelayMinutes)
	})
}

/*
findClosed returns the conversation the ask is for, and false when it
is gone, no longer closed, or a test conversation.
*/
func findClosed(
	ctx context.Context, q *models.Queries, args jobs.AskConversationRatingArgs,
) (models.Conversation, bool, error) {
	c, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: args.WorkspaceID, ID: args.ConversationID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return c, false, nil
	}
	if err != nil {
		return c, false, fmt.Errorf("find rated conversation: %w", err)
	}

	return c, closedNow(c) && !c.IsTest, nil
}

/*
schedule sends the ask now when there is no delay, and otherwise queues
it for when it comes due.
*/
func (r *Ratings) schedule(ctx context.Context, t txn, row models.ConversationRating, delay int) error {
	if delay == 0 {
		return r.deliver(ctx, t, row)
	}

	args := jobs.SendConversationRatingArgs{WorkspaceID: row.WorkspaceID, RatingID: row.ID}
	if err := r.Queue.PushTx(ctx, t.tx, args, queue.At(row.AskAt)); err != nil {
		return fmt.Errorf("time rating ask: %w", err)
	}

	return nil
}

/*
channelOf returns the channel the close asks on, and false when it asks
on none: asking is off, the agent closed it and the workspace leaves
those out, or its channel cannot carry an ask. A workflow's ask passes
every switch but still needs a channel that can carry it.
*/
func (r *Ratings) channelOf(
	ctx context.Context, q *models.Queries, s Settings, args jobs.AskConversationRatingArgs,
) (string, bool, error) {
	if !args.Forced && (!s.Enabled || (args.Actor == string(enums.AuthorTypeAgent) && !s.IncludeAgentClosed)) {
		return "", false, nil
	}

	kind, err := q.FindConversationChannelType(ctx, models.FindConversationChannelTypeParams{
		WorkspaceID: args.WorkspaceID, ID: args.ConversationID,
	})
	if err != nil {
		return "", false, fmt.Errorf("find rated conversation channel: %w", err)
	}

	switch {
	case kind == ChannelWidget && (args.Forced || s.Messenger):
		return ChannelWidget, true, nil
	case kind == ChannelEmail && (args.Forced || s.Email):
		return ChannelEmail, true, nil
	}

	return "", false, nil
}

/*
Send sends an ask that came due.
*/
func (r *Ratings) Send(ctx context.Context, args jobs.SendConversationRatingArgs) error {
	return r.inWorkspace(ctx, args.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		row, err := q.FindConversationRating(ctx, models.FindConversationRatingParams{
			WorkspaceID: args.WorkspaceID, ID: args.RatingID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find rating ask: %w", err)
		}

		return r.deliver(ctx, txn{tx: tx, q: q}, row)
	})
}

/*
deliver marks the ask as gone out and tells the customer: the messenger
hears of it and shows its card, and an email conversation's customer
gets the email with the faces from a job of its own.
*/
func (r *Ratings) deliver(ctx context.Context, t txn, row models.ConversationRating) error {
	if row.AskedAt != nil {
		return nil
	}

	q := t.q
	c, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: row.WorkspaceID, ID: row.ConversationID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find rated conversation: %w", err)
	}

	if !closedNow(c) || !c.ClosedAt.Equal(row.ClosedAt) {
		return nil
	}

	now := r.Clock.Now()
	row, err = q.MarkConversationRatingAsked(ctx, models.MarkConversationRatingAskedParams{
		WorkspaceID: row.WorkspaceID, ID: row.ID, Now: &now,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("mark rating asked: %w", err)
	}

	if row.Channel == ChannelEmail {
		args := jobs.MailConversationRatingArgs{WorkspaceID: row.WorkspaceID, RatingID: row.ID}
		if err := r.Queue.PushTx(ctx, t.tx, args); err != nil {
			return fmt.Errorf("queue rating email: %w", err)
		}
	}

	return r.Events.Dispatch(ctx, t.tx, appevents.ConversationRatingRequested{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: row.WorkspaceID, OccurredAt: now,
			Actor: appevents.Actor{Type: string(enums.AuthorTypeSystem)},
		},
		ConversationRef: appevents.ConversationRef{ConversationID: row.ConversationID},
		RatingID:        row.ID, Channel: row.Channel,
	})
}

/*
closedNow reports whether the conversation is closed and still in the
inbox.
*/
func closedNow(c models.Conversation) bool {
	return c.Status == string(enums.ConversationStatusClosed) && c.ClosedAt != nil && c.DeletedAt == nil && !c.Spam
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
