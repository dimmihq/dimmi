/*
|--------------------------------------------------------------------------
| Channels
|--------------------------------------------------------------------------
|
| The product's channels are named after what they follow, a namespace
| and a public id, such as conversation:conv_01jb.... Raw row ids never
| appear in a channel name.
|
| The workspace channel tells every member about changes to the lists
| they look at and who is online. A conversation's channel carries its
| full timeline and who is viewing it right now, which is how teammates
| avoid answering the same customer twice. A member's channel carries
| what only they should see, and a contact's channel what the messenger
| widget shows that contact. Each keeps ten minutes of history, so a
| screen that loses its connection briefly catches up without a reload.
|
*/

package broadcasting

import (
	"time"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The channel patterns, with the placeholder each authorizer reads.
*/
const (
	WorkspacePattern     = "workspace:{workspace}"
	ConversationPattern  = "conversation:{conversation}"
	MemberPattern        = "member:{member}"
	WidgetContactPattern = "widget_contact:{contact}"
)

/*
historyTTL is how long every channel keeps its broadcasts for clients
that reconnect.
*/
const historyTTL = 10 * time.Minute

/*
WorkspaceOptions tracks the members online and keeps the last 300
broadcasts.
*/
func WorkspaceOptions() fbroadcasting.ChannelOptions {
	return fbroadcasting.ChannelOptions{Presence: true, HistorySize: 300, HistoryTTL: historyTTL}
}

/*
ConversationOptions tracks who is viewing, announces arrivals and
departures, and keeps the last 300 broadcasts.
*/
func ConversationOptions() fbroadcasting.ChannelOptions {
	return fbroadcasting.ChannelOptions{Presence: true, JoinLeave: true, HistorySize: 300, HistoryTTL: historyTTL}
}

/*
MemberOptions keeps the last 100 broadcasts of one member's channel.
*/
func MemberOptions() fbroadcasting.ChannelOptions {
	return fbroadcasting.ChannelOptions{HistorySize: 100, HistoryTTL: historyTTL}
}

/*
WidgetContactOptions tracks which of the contact's messengers are open,
so a reply nobody is watching can be emailed instead, and keeps the last
300 broadcasts.
*/
func WidgetContactOptions() fbroadcasting.ChannelOptions {
	return fbroadcasting.ChannelOptions{Presence: true, HistorySize: 300, HistoryTTL: historyTTL}
}

/*
WorkspaceChannel names the workspace's channel.
*/
func WorkspaceChannel(id ids.UUID) string {
	return "workspace:" + models.PublicID(models.PrefixWorkspace, id)
}

/*
ConversationChannel names the conversation's channel.
*/
func ConversationChannel(id ids.UUID) string {
	return "conversation:" + models.PublicID(models.PrefixConversation, id)
}

/*
MemberChannel names the member's own channel.
*/
func MemberChannel(id ids.UUID) string {
	return "member:" + models.PublicID(models.PrefixMember, id)
}

/*
WidgetContactChannel names the channel the contact's messenger widget
listens on.
*/
func WidgetContactChannel(id ids.UUID) string {
	return "widget_contact:" + models.PublicID(models.PrefixContact, id)
}
