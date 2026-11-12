/*
|--------------------------------------------------------------------------
| Slack Threads For Tickets
|--------------------------------------------------------------------------
|
| What turning a Slack thread into a ticket needs from the channel. A
| teammate marks a customer's thread, so the thread's conversation is
| found, or opened from its first message when nothing opened it yet,
| and who marked it is told apart from the customers in the room. The
| ticket itself is made by the tickets intake, which claims its share
| of Slack's deliveries ahead of the channel's own, so the trigger emoji
| of a room both opens the conversation and makes the ticket.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OnEventFirst claims events of the kind ahead of every handler already
registered for it, for a feature that must see a delivery before the
channel's own claims take it.
*/
func (r *Router) OnEventFirst(kind string, h EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events[kind] = append([]EventHandler{h}, r.events[kind]...)
}

/*
ThreadMark is a teammate marking a message of a room: the connection,
the Slack channel and message, who marked it, and the message itself
when the delivery carried it. Trigger is what opens the conversation
when the thread has none yet.
*/
type ThreadMark struct {
	Conn    models.ChannelConnection
	Channel string
	TS      string
	User    string
	Message *slackdriver.Event
	Trigger enums.RoomTrigger
}

/*
MarkedThread is the conversation of a marked thread: its room, its
first message when the mark opened it, and the teammate who marked
it, nil when they are not linked to a member.
*/
type MarkedThread struct {
	ConversationID ids.UUID
	Room           models.ChannelRoom
	Root           slackdriver.Event
	MemberID       *ids.UUID
	MemberName     string
}

/*
The reasons a mark leaves a thread alone.
*/
var (
	ErrMarkUnwatched = errors.New("dimmi does not watch this channel")
	ErrMarkCustomer  = errors.New("only a teammate can make a ticket")
	ErrMarkNoThread  = errors.New("only a customer's thread can become a ticket")
)

/*
MarkThread finds the conversation of the marked message's thread,
opening it from the thread's first message when nothing has yet. It
refuses an unwatched or internal room, a mark by anyone but a teammate,
and a thread a customer did not start.
*/
func (s *Slack) MarkThread(ctx context.Context, mark ThreadMark) (MarkedThread, error) {
	room, ok, err := s.watchedRoom(ctx, mark.Conn, mark.Channel)
	if err != nil {
		return MarkedThread{}, err
	}

	if !ok || room.Mode == string(enums.RoomModeInternal) {
		return MarkedThread{}, ErrMarkUnwatched
	}

	marker, err := s.classify(ctx, mark.Conn, &room, mark.User)
	if err != nil {
		return MarkedThread{}, err
	}

	if marker.kind != authorTeammate {
		return MarkedThread{}, ErrMarkCustomer
	}

	out := MarkedThread{Room: room, MemberID: marker.memberID, MemberName: marker.memberName}

	if out.ConversationID, err = s.markedConversation(ctx, mark); err != nil || out.ConversationID != ids.Nil() {
		return out, err
	}

	if out.Root, err = s.markedRoot(ctx, mark); err != nil {
		return out, err
	}

	out.ConversationID, err = s.openMarked(ctx, mark, &room, out.Root)

	return out, err
}

/*
markedConversation returns the conversation that already holds the
marked message or its thread, a zero id for none.
*/
func (s *Slack) markedConversation(ctx context.Context, mark ThreadMark) (ids.UUID, error) {
	keys := []string{slackdriver.MessageID(mark.Channel, mark.TS)}
	if mark.Message != nil && mark.Message.RootTS() != mark.TS {
		keys = append(keys, slackdriver.MessageID(mark.Channel, mark.Message.RootTS()))
	}

	var found ids.UUID

	err := s.read(ctx, func(q *models.Queries) error {
		for _, key := range keys {
			msg, err := q.FindChannelMessage(ctx, models.FindChannelMessageParams{
				WorkspaceID: mark.Conn.WorkspaceID, ChannelConnectionID: &mark.Conn.ID, ChannelMessageID: &key,
			})
			if errors.Is(err, database.ErrNotFound) {
				continue
			}

			if err != nil {
				return fmt.Errorf("find marked message: %w", err)
			}

			found = msg.ConversationID

			return nil
		}

		return nil
	})

	return found, err
}

/*
markedRoot returns the first message of the marked thread, read from
Slack unless the delivery carried it.
*/
func (s *Slack) markedRoot(ctx context.Context, mark ThreadMark) (slackdriver.Event, error) {
	if mark.Message != nil && !mark.Message.InThread() {
		return *mark.Message, nil
	}

	root := mark.TS
	if mark.Message != nil {
		root = mark.Message.RootTS()
	}

	api, err := s.api(ctx, mark.Conn)
	if err != nil {
		return slackdriver.Event{}, err
	}

	msg, found, err := api.Message(ctx, mark.Channel, root)
	if err != nil {
		return msg, err
	}

	if !found {
		return msg, ErrMarkNoThread
	}

	return msg, nil
}

/*
openMarked opens the conversation of a thread nothing opened yet from
its first message, which a customer must have written.
*/
func (s *Slack) openMarked(
	ctx context.Context, mark ThreadMark, room *models.ChannelRoom, root slackdriver.Event,
) (ids.UUID, error) {
	if root.BotID != "" || root.User == "" {
		return ids.Nil(), ErrMarkNoThread
	}

	a, err := s.classify(ctx, mark.Conn, room, root.User)
	if err != nil {
		return ids.Nil(), err
	}

	if a.kind != authorContact {
		return ids.Nil(), ErrMarkNoThread
	}

	root.ChannelRaw = quotedChannel(mark.Channel)

	result, handled, err := s.ingest(ctx, ingestion{
		conn: mark.Conn, room: room, ev: root, author: a, trigger: mark.Trigger,
		threadKey: slackdriver.RoomThreadKey(room.ID, root.RootTS()),
	})
	if err != nil {
		return ids.Nil(), err
	}

	if !handled {
		return ids.Nil(), ErrMarkNoThread
	}

	return result.ConversationID, nil
}

/*
Whisper shows the teammate who used a shortcut a note only they see.
*/
func (s *Slack) Whisper(ctx context.Context, in SlackInteraction, text string) {
	s.tell(ctx, in, text)
}

/*
RoomSetting returns a room's setting by key, empty when unset.
*/
func RoomSetting(room models.ChannelRoom, key string) string {
	return settingOf(&room, key)
}
