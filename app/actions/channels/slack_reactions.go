/*
|--------------------------------------------------------------------------
| Trigger Emoji
|--------------------------------------------------------------------------
|
| In a room whose trigger is an emoji, nothing becomes a conversation
| when it is posted. Anyone in the room marks a customer's top-level
| message with the trigger emoji, ticket unless the room names another,
| and that message opens a conversation. Marking it twice, or by two
| people, opens it once. Other reactions are left for other features.
|
*/

package channels

import (
	"cmp"
	"context"
	"encoding/json"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
onTriggerEmoji opens a conversation from a message marked with the
room's trigger emoji.
*/
func (s *Slack) onTriggerEmoji(ctx context.Context, ev SlackEvent) (bool, error) {
	item := ev.Event.Item
	if item.Type != "message" || item.Channel == "" {
		return false, nil
	}

	room, ok, err := s.watchedRoom(ctx, ev.Connection, item.Channel)
	if err != nil || !ok || room.Trigger != string(enums.RoomTriggerEmoji) {
		return false, err
	}

	if ev.Event.Reaction != cmp.Or(settingOf(&room, "trigger_emoji"), s.Config.TriggerEmoji) {
		return false, nil
	}

	api, err := s.api(ctx, ev.Connection)
	if err != nil {
		return false, err
	}

	msg, found, err := api.Message(ctx, item.Channel, item.TS)
	if err != nil || !found || msg.BotID != "" || msg.User == "" || msg.InThread() {
		return false, err
	}

	msg.ChannelRaw = quotedChannel(item.Channel)

	a, err := s.classify(ctx, ev.Connection, &room, msg.User)
	if err != nil || a.kind != authorContact {
		return false, err
	}

	_, handled, err := s.ingest(ctx, ingestion{
		conn: ev.Connection, room: &room, ev: msg, author: a, trigger: enums.RoomTriggerEmoji,
		threadKey: slackdriver.RoomThreadKey(room.ID, msg.TS),
	})

	return handled, err
}

/*
quotedChannel returns a channel id the way an event carries it.
*/
func quotedChannel(channel string) json.RawMessage {
	raw, _ := json.Marshal(channel)

	return raw
}
