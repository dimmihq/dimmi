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

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
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

	return outcome(err)
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

	return outcome(err)
}

/*
outcome reads a change's error: a message the inbox does not hold was
simply not handled.
*/
func outcome(err error) (bool, error) {
	if errors.Is(err, conversations.ErrInboundMessageNotFound) {
		return false, nil
	}

	return err == nil, err
}
