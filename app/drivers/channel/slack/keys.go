/*
|--------------------------------------------------------------------------
| Slack Keys
|--------------------------------------------------------------------------
|
| The names the product gives Slack things. A conversation in a channel
| is keyed by the room's own id and its thread's root timestamp, never
| the channel id, because Slack changes a channel's id when it is shared
| with another organization. A conversation in a direct message is keyed
| by the direct message itself, one at a time. A message is known by its
| channel and timestamp, kept as text since a timestamp read as a number
| loses its precision. A Slack user is known by their home team and id,
| as people from two organizations meet in one shared channel.
|
| An answer's buttons carry the message and the control they stand for,
| short enough for every chat app's limit on a button's id.
|
*/

package slack

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Type is the connection type the driver serves.
*/
const Type = "slack"

/*
Prefixes of the thread keys, the action ids of answer controls, and the
callback id of the shortcut that opens a conversation from a message.
*/
const (
	roomThreadPrefix   = "slack:r:"
	directThreadPrefix = "slack:d:"
	controlPrefix      = "c:"
	ShortcutCreate     = "dimmi_create_conversation"
)

/*
RoomThreadKey names the conversation of a thread in a room.
*/
func RoomThreadKey(roomID ids.UUID, rootTS string) string {
	return roomThreadPrefix + roomID.String() + ":" + rootTS
}

/*
RoomThreadPrefix is what every thread key of the room starts with.
*/
func RoomThreadPrefix(roomID ids.UUID) string {
	return roomThreadPrefix + roomID.String() + ":"
}

/*
DirectThreadKey names the conversation of a direct message.
*/
func DirectThreadKey(channelID string) string {
	return directThreadPrefix + channelID
}

/*
Thread is where a thread key points: a room and the root of a thread in
it, or a direct message channel.
*/
type Thread struct {
	RoomID  ids.UUID
	RootTS  string
	Channel string
}

/*
Direct reports whether the thread is a direct message.
*/
func (t Thread) Direct() bool {
	return t.Channel != ""
}

/*
ParseThreadKey reads a thread key, reporting false for one that names
no Slack thread.
*/
func ParseThreadKey(key string) (Thread, bool) {
	if channel, ok := strings.CutPrefix(key, directThreadPrefix); ok && channel != "" {
		return Thread{Channel: channel}, true
	}

	rest, ok := strings.CutPrefix(key, roomThreadPrefix)
	if !ok {
		return Thread{}, false
	}

	room, ts, ok := strings.Cut(rest, ":")
	if !ok || ts == "" {
		return Thread{}, false
	}

	roomID, err := ids.Parse(room)
	if err != nil {
		return Thread{}, false
	}

	return Thread{RoomID: roomID, RootTS: ts}, true
}

/*
MessageID names a message by its channel and timestamp.
*/
func MessageID(channel, ts string) string {
	return channel + ":" + ts
}

/*
SplitMessageID reads a message id back into its channel and timestamp.
*/
func SplitMessageID(id string) (channel, ts string, ok bool) {
	channel, ts, ok = strings.Cut(id, ":")

	return channel, ts, ok && channel != "" && ts != ""
}

/*
FileID names a file shared with a message.
*/
func FileID(fileID string) string {
	return "file:" + fileID
}

/*
DedupeKey recognizes a message however it arrives: retried, over both
the socket and a webhook, or opened again by hand.
*/
func DedupeKey(teamID, channel, ts string) string {
	return "slack:" + teamID + ":" + channel + ":" + ts
}

/*
UserIdentity names a Slack user by their home team.
*/
func UserIdentity(teamID, userID string) string {
	return teamID + ":" + userID
}

/*
ExternalID is the connection's routing key: the app and the team it is
installed in.
*/
func ExternalID(appID, teamID string) string {
	return appID + ":" + teamID
}

/*
ControlActionID is the action id of an answer control.
*/
func ControlActionID(messagePublicID, control string) string {
	return controlPrefix + messagePublicID + ":" + control
}

/*
ControlActionPrefix is what every answer control's action id starts
with.
*/
func ControlActionPrefix() string {
	return controlPrefix
}

/*
ParseControlActionID reads an answer control's action id into the
message's public id and the control.
*/
func ParseControlActionID(actionID string) (messagePublicID, control string, ok bool) {
	rest, ok := strings.CutPrefix(actionID, controlPrefix)
	if !ok {
		return "", "", false
	}

	i := strings.LastIndex(rest, ":")
	if i <= 0 || i == len(rest)-1 {
		return "", "", false
	}

	return rest[:i], rest[i+1:], true
}
