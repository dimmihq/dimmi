/*
|--------------------------------------------------------------------------
| Visible Conversations
|--------------------------------------------------------------------------
|
| What a visitor may see of their conversations and messages: the rows the
| inbox keeps, with the author reduced to a first name and a face and each
| file linked only once a scan cleared it.
|
*/

package widget

import "github.com/dimmihq/dimmi/app/models"

/*
Author is who wrote a message, as a visitor sees them.
*/
type Author struct {
	Type      string  `json:"type"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
VisibleAttachment is a file on a message, with a short-lived link when the
scan cleared it.
*/
type VisibleAttachment struct {
	Row     models.Attachment
	URL     *string
	Blocked bool
}

/*
VisibleMessage is a message with what a visitor sees of its author and
files.
*/
type VisibleMessage struct {
	Message     models.Message
	Author      Author
	Attachments []VisibleAttachment
}

/*
VisibleConversation is one of a visitor's conversations, with its latest
message and how many of the team's replies wait unread.
*/
type VisibleConversation struct {
	Conversation models.Conversation
	ChannelType  string
	UnreadCount  int64
	Last         *VisibleMessage
}
