/*
|--------------------------------------------------------------------------
| Channel Rooms
|--------------------------------------------------------------------------
|
| A room is a place on a chat app a connection watches. Its mode says
| who is the customer there: in a shared room the other organization's
| people, in a community room everyone but linked teammates, in a
| helpdesk room the team's own colleagues, and in an internal room
| nobody, since the team talks among itself there. Its trigger says
| when a message becomes a conversation, and its status whether it is
| watched at all.
|
*/

package enums

/*
RoomMode says who counts as the customer in a room.
*/
type RoomMode string

/*
The room modes.
*/
const (
	RoomModeShared    RoomMode = "shared"
	RoomModeCommunity RoomMode = "community"
	RoomModeInternal  RoomMode = "internal"
	RoomModeHelpdesk  RoomMode = "helpdesk"
)

/*
RoomTrigger says when a top-level message in a room becomes a
conversation.
*/
type RoomTrigger string

/*
The room triggers. Shortcut is no room setting: it names a conversation
a person opened by hand from a message.
*/
const (
	RoomTriggerAuthorWindow RoomTrigger = "author_window"
	RoomTriggerEveryMessage RoomTrigger = "every_message"
	RoomTriggerEmoji        RoomTrigger = "emoji"
	RoomTriggerMention      RoomTrigger = "mention"
	RoomTriggerAI           RoomTrigger = "ai"
	RoomTriggerAuto         RoomTrigger = "auto"
	RoomTriggerOff          RoomTrigger = "off"
	RoomTriggerShortcut     RoomTrigger = "shortcut"
)

/*
RoomStatus says whether a room is watched.
*/
type RoomStatus string

/*
The room statuses.
*/
const (
	RoomStatusActive  RoomStatus = "active"
	RoomStatusPaused  RoomStatus = "paused"
	RoomStatusRemoved RoomStatus = "removed"
	RoomStatusError   RoomStatus = "error"
)

/*
RoomKindSlackChannel is the kind of a room that is a Slack channel.
*/
const RoomKindSlackChannel = "slack_channel"

/*
ChannelEventStatus is where a delivery from a chat app stands: waiting,
done, deliberately left alone, failed, or held while its workspace is
suspended.
*/
type ChannelEventStatus string

/*
The delivery statuses.
*/
const (
	ChannelEventPending   ChannelEventStatus = "pending"
	ChannelEventProcessed ChannelEventStatus = "processed"
	ChannelEventIgnored   ChannelEventStatus = "ignored"
	ChannelEventFailed    ChannelEventStatus = "failed"
	ChannelEventHeld      ChannelEventStatus = "held"
)
