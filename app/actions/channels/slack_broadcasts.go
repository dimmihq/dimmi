/*
|--------------------------------------------------------------------------
| Answers To Slack Broadcasts
|--------------------------------------------------------------------------
|
| A broadcast posted into a customer channel starts no conversation by
| itself. When someone answers in its thread, the conversation that
| answer opens starts with the post, so the team reads what is being
| answered, and the broadcast counts the reply.
|
*/

package channels

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackBroadcasts opens the conversations answers to broadcast posts start.
*/
type SlackBroadcasts interface {
	/*
		SlackOpened writes the broadcast post the thread answers into the
		new conversation; it does nothing for a thread no broadcast
		started.
	*/
	SlackOpened(ctx context.Context, tx *database.Tx, conv models.Conversation, thread SlackThread) error

	/*
		SlackReacted counts a reaction added to, or taken off, a
		broadcast post; it does nothing for any other message.
	*/
	SlackReacted(ctx context.Context, post SlackThread, delta int) error
}

/*
SlackThread is the Slack thread an answer was posted in: the room and
the timestamp of the post that started the thread.
*/
type SlackThread struct {
	Room   models.ChannelRoom
	RootTS string
}

/*
broadcastOpening is the hook a thread reply carries, so a conversation
it starts under a broadcast post begins with that post.
*/
func (s *Slack) broadcastOpening(room models.ChannelRoom, rootTS string) contracts.BeforeFirst {
	if s.Broadcasts == nil || rootTS == "" {
		return nil
	}

	return func(ctx context.Context, tx *database.Tx, conv models.Conversation) error {
		return s.Broadcasts.SlackOpened(ctx, tx, conv, SlackThread{Room: room, RootTS: rootTS})
	}
}

/*
onBroadcastReaction counts reactions on broadcast posts. It never
claims the event, so emoji actions on the same message still run.
*/
func (s *Slack) onBroadcastReaction(ctx context.Context, ev SlackEvent) (bool, error) {
	item := ev.Event.Item
	if s.Broadcasts == nil || item.Type != "message" || item.Channel == "" || item.TS == "" {
		return false, nil
	}

	room, ok, err := s.watchedRoom(ctx, ev.Connection, item.Channel)
	if err != nil || !ok {
		return false, err
	}

	delta := 1
	if ev.Event.Type == "reaction_removed" {
		delta = -1
	}

	return false, s.Broadcasts.SlackReacted(ctx, SlackThread{Room: room, RootTS: item.TS}, delta)
}
