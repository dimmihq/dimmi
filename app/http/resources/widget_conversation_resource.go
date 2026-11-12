/*
|--------------------------------------------------------------------------
| Widget Conversation Resources
|--------------------------------------------------------------------------
|
| Conversations and messages as a visitor sees them. They show the status a
| customer may know, never the one the team works with: a snoozed thread is
| simply with the team, a spam flag never shows, and nobody's assignment,
| tags or notes ever appear. A teammate is a first name and a face.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Customer statuses a visitor sees on a conversation.
*/
const (
	CustomerStatusWithTeam     = "with_team"
	CustomerStatusWaitingOnYou = "waiting_on_you"
	CustomerStatusClosed       = "closed"
)

/*
WidgetAttachment is a file on a message. URL is a short-lived link, null
while the file is unchecked, when it was blocked, or when its disk signs no
links. Blocked is set only when the malware scan refused the file.
*/
type WidgetAttachment struct {
	ID          string  `json:"id"`
	Object      string  `json:"object"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
	URL         *string `json:"url"`
	Blocked     bool    `json:"blocked,omitempty"`
	Width       *int32  `json:"width"`
	Height      *int32  `json:"height"`
}

/*
WidgetMessage is a message as a visitor sees it. A deleted message keeps
its place with no body. AI marks a reply the agent wrote, with the
sources it cites.
*/
type WidgetMessage struct {
	ID             string             `json:"id"`
	Object         string             `json:"object"`
	ConversationID string             `json:"conversation_id"`
	ClientID       *string            `json:"client_id"`
	Author         widget.Author      `json:"author"`
	Body           *string            `json:"body"`
	Deleted        bool               `json:"deleted,omitempty"`
	Attachments    []WidgetAttachment `json:"attachments"`
	CreatedAt      time.Time          `json:"created_at"`
	ReadAt         *time.Time         `json:"read_at"`
	AI             *MessageAI         `json:"ai,omitempty"`
	Broadcast      *WidgetBroadcast   `json:"broadcast,omitempty"`
}

/*
WidgetBroadcast marks a message a broadcast sent: Display is message or
post, and Title what a post card leads with.
*/
type WidgetBroadcast struct {
	Display string `json:"display"`
	Title   string `json:"title,omitempty"`
}

/*
WidgetLastMessage is the latest message of a conversation in a list.
*/
type WidgetLastMessage struct {
	ID        string        `json:"id"`
	Author    widget.Author `json:"author"`
	Excerpt   string        `json:"excerpt"`
	CreatedAt time.Time     `json:"created_at"`
}

/*
WidgetConversation is a conversation as a visitor sees it. SeenByTeamAt
is when a teammate last read it up to the customer's latest message,
left out when nobody has or the workspace keeps it from customers.
Ticket is the request the conversation carries, left out for a plain
conversation.
*/
type WidgetConversation struct {
	ID             string             `json:"id"`
	Object         string             `json:"object"`
	Number         int64              `json:"number"`
	Status         string             `json:"status"`
	Subject        *string            `json:"subject"`
	LastMessage    *WidgetLastMessage `json:"last_message"`
	UnreadCount    int64              `json:"unread_count"`
	AIState        string             `json:"ai_state"`
	CustomerStatus string             `json:"customer_status"`
	WidgetID       *string            `json:"widget_id"`
	Imported       bool               `json:"imported,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	LastMessageAt  *time.Time         `json:"last_message_at"`
	SeenByTeamAt   *time.Time         `json:"seen_by_team_at,omitempty"`
	Ticket         *WidgetTicketBlock `json:"ticket,omitempty"`
}

/*
NewWidgetConversation returns the conversation as a visitor sees it.
*/
func NewWidgetConversation(v widget.VisibleConversation) WidgetConversation {
	c := v.Conversation
	status := c.Status

	if status == string(enums.ConversationStatusSnoozed) {
		status = string(enums.ConversationStatusOpen)
	}

	out := WidgetConversation{
		ID: models.PublicID(models.PrefixConversation, c.ID), Object: "conversation", Number: c.Number,
		Status: status, Subject: c.Subject, UnreadCount: v.UnreadCount, AIState: c.AiState,
		CustomerStatus: CustomerStatus(c, v.Last), WidgetID: SourceWidgetID(c.Source),
		Imported: v.ChannelType == "import", CreatedAt: c.CreatedAt.UTC(), LastMessageAt: c.LastMessageAt,
		SeenByTeamAt: v.SeenAt,
	}

	if v.Last != nil {
		short := ""
		if v.Last.Message.DeletedAt == nil {
			short = excerpt(&v.Last.Message)
		}

		out.LastMessage = &WidgetLastMessage{
			ID: models.PublicID(models.PrefixMessage, v.Last.Message.ID), Author: v.Last.Author, Excerpt: short,
			CreatedAt: v.Last.Message.CreatedAt.UTC(),
		}
	}

	return out
}

/*
CustomerStatus is what the customer is told of a conversation: closed, or
open and waiting on them once the team spoke last, or else with the team.
*/
func CustomerStatus(c models.Conversation, last *widget.VisibleMessage) string {
	if c.Status == string(enums.ConversationStatusClosed) {
		return CustomerStatusClosed
	}

	if last == nil {
		return CustomerStatusWithTeam
	}

	if last.Author.Type == string(enums.AuthorTypeMember) || last.Author.Type == string(enums.AuthorTypeAgent) {
		return CustomerStatusWaitingOnYou
	}

	return CustomerStatusWithTeam
}

/*
SourceWidgetID returns the widget a conversation started on, nil for one of
another channel.
*/
func SourceWidgetID(source json.RawMessage) *string {
	var s struct {
		WidgetID string `json:"widget_id"`
	}

	if json.Unmarshal(source, &s) != nil || s.WidgetID == "" {
		return nil
	}

	return &s.WidgetID
}

/*
NewWidgetMessage returns the message as a visitor sees it.
*/
func NewWidgetMessage(v widget.VisibleMessage) WidgetMessage {
	m := v.Message
	out := WidgetMessage{
		ID: models.PublicID(models.PrefixMessage, m.ID), Object: "message",
		ConversationID: models.PublicID(models.PrefixConversation, m.ConversationID), ClientID: clientID(m.Metadata),
		Author: v.Author, Body: m.Body, Attachments: []WidgetAttachment{}, CreatedAt: m.CreatedAt.UTC(),
		ReadAt: m.ReadAt, AI: customerAIOf(m.Metadata), Broadcast: broadcastOf(m.Metadata),
	}

	if m.DeletedAt != nil {
		out.Body, out.Deleted, out.Attachments, out.AI = nil, true, []WidgetAttachment{}, nil

		return out
	}

	for _, file := range v.Attachments {
		out.Attachments = append(out.Attachments, NewWidgetAttachment(file))
	}

	return out
}

/*
NewWidgetAttachment presents a file as a visitor sees it.
*/
func NewWidgetAttachment(file widget.VisibleAttachment) WidgetAttachment {
	return WidgetAttachment{
		ID: models.PublicID(models.PrefixAttachment, file.Row.ID), Object: "attachment",
		Filename: file.Row.Filename, ContentType: file.Row.ContentType, SizeBytes: file.Row.SizeBytes,
		URL: file.URL, Blocked: file.Blocked, Width: file.Row.Width, Height: file.Row.Height,
	}
}

/*
broadcastOf reads how a broadcast's message shows in the Messenger, nil
for any other message.
*/
func broadcastOf(metadata json.RawMessage) *WidgetBroadcast {
	var m struct {
		Broadcast *struct {
			Display string `json:"display"`
			Subject string `json:"subject"`
		} `json:"broadcast"`
	}

	if json.Unmarshal(metadata, &m) != nil || m.Broadcast == nil {
		return nil
	}

	return &WidgetBroadcast{Display: m.Broadcast.Display, Title: m.Broadcast.Subject}
}

/*
clientID returns the id the visitor's browser gave the message.
*/
func clientID(metadata json.RawMessage) *string {
	var m struct {
		Client struct {
			ClientID string `json:"client_id"`
		} `json:"client"`
	}

	if json.Unmarshal(metadata, &m) != nil || m.Client.ClientID == "" {
		return nil
	}

	return &m.Client.ClientID
}
