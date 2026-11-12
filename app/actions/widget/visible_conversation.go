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

import (
	"context"

	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Author is who wrote a message, as a visitor sees them.
*/
type Author struct {
	Type      string  `json:"type"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
VisibleAttachment is a file on a message. URL is a short-lived link, there
once a scan cleared the file and the disk can sign one; Blocked says the
scan refused it.
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

/*
presentAttachment links a file the scan cleared, when its disk signs links,
and marks one the scan refused.
*/
func presentAttachment(ctx context.Context, disk filesystem.Filesystem, row models.Attachment) VisibleAttachment {
	file := VisibleAttachment{Row: row, Blocked: row.ScanStatus == "infected"}

	if row.ScanStatus != "clean" && row.ScanStatus != "skipped" {
		return file
	}

	if url, err := messages.Link(ctx, disk, row, attachmentLinkTTL); err == nil {
		file.URL = &url
	}

	return file
}
