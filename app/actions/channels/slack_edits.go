/*
|--------------------------------------------------------------------------
| Slack Edits And Deletions
|--------------------------------------------------------------------------
|
| A person who corrects a message in Slack corrects it in the inbox too,
| and one who deletes it, often after pasting a secret, deletes its
| words and files here as well. Only messages the inbox holds change;
| an edit or deletion of anything else, a bot's included, is ignored.
| An edit never asks the agent to answer again.
|
*/

package channels

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
)

/*
How long after a message was written an edit or delete of it waits for
the message itself to arrive, and how often it looks again.
*/
const (
	changeWait  = 2 * time.Minute
	changeRetry = 3 * time.Second
)

/*
onEdited follows an edit of a message the inbox holds.
*/
func (s *Slack) onEdited(ctx context.Context, ev SlackEvent) (bool, error) {
	edited := ev.Event.Message
	if edited == nil || edited.BotID != "" || edited.User == "" {
		return false, nil
	}

	_, err := s.Edits.Edit(ctx, contracts.InboundEdit{
		ConnectionID: ev.Connection.ID, ChannelMessageID: slackdriver.MessageID(ev.Event.ChannelID(), edited.TS),
		Body:     slackdriver.ToMarkdown(edited.Text, edited.Blocks, s.names(ctx, ev.Connection)),
		EditedAt: s.Clock.Now(),
	})

	return s.outcome(err, edited.TS)
}

/*
onDeleted marks a message the inbox holds as deleted by its author,
keeping what it said.
*/
func (s *Slack) onDeleted(ctx context.Context, ev SlackEvent) (bool, error) {
	if ev.Event.DeletedTS == "" {
		return false, nil
	}

	_, err := s.Edits.MarkDeleted(ctx, contracts.InboundDeletion{
		ConnectionID:     ev.Connection.ID,
		ChannelMessageID: slackdriver.MessageID(ev.Event.ChannelID(), ev.Event.DeletedTS),
		Reason:           conversations.ReasonSourceDeleted,
	})

	return s.outcome(err, ev.Event.DeletedTS)
}

/*
outcome reads a change's error. A message the inbox does not hold was
simply not handled, unless it was written moments ago: then it may
still be on its way in, and the change is tried again shortly.
*/
func (s *Slack) outcome(err error, ts string) (bool, error) {
	if !errors.Is(err, conversations.ErrInboundMessageNotFound) {
		return err == nil, err
	}

	if s.Clock.Now().Sub(slackdriver.TimeOf(ts)) < changeWait {
		return false, queue.RetryAfter(changeRetry)
	}

	return false, nil
}
