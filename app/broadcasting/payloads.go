/*
|--------------------------------------------------------------------------
| Broadcast Payloads
|--------------------------------------------------------------------------
|
| A broadcast shows the subject of an event as it is now, read when the
| broadcast goes out, never a copy the event carried: two changes in a
| row then both show the latest state, whichever arrives last. Lists get
| the compact row they render, the open conversation gets the whole
| record, and a timeline gets the message.
|
| The shapes are the API's, so a screen patches what it fetched with
| what it hears without translating. A subject that no longer exists has
| nothing to show and is not broadcast.
|
*/

package broadcasting

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Payloads renders the subjects of broadcasts. Each method reports false
when the subject no longer exists.
*/
type Payloads interface {
	/*
		ConversationRow renders the conversation as lists show it.
	*/
	ConversationRow(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error)

	/*
		Conversation renders the whole conversation.
	*/
	Conversation(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error)

	/*
		Message renders the message.
	*/
	Message(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error)
}

/*
Reference is what a broadcast carries in place of a subject too large to
broadcast; clients fetch the rest.
*/
type Reference struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	UpdatedAt time.Time `json:"updated_at"`
}

/*
Ref is a related record by its public id.
*/
type Ref struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

/*
ConversationPayload is a conversation as broadcasts show it.
*/
type ConversationPayload struct {
	ID            string     `json:"id"`
	Object        string     `json:"object"`
	Number        int64      `json:"number"`
	Subject       *string    `json:"subject"`
	Status        string     `json:"status"`
	Spam          bool       `json:"spam"`
	Priority      bool       `json:"priority"`
	AIState       string     `json:"ai_state"`
	Inbox         Ref        `json:"inbox"`
	Channel       Ref        `json:"channel"`
	Contact       Ref        `json:"contact"`
	Assignee      *Ref       `json:"assignee"`
	Team          *Ref       `json:"team"`
	MessageCount  int32      `json:"message_count"`
	LastMessageAt *time.Time `json:"last_message_at"`
	SnoozedUntil  *time.Time `json:"snoozed_until"`
	ClosedAt      *time.Time `json:"closed_at"`
	DeletedAt     *time.Time `json:"deleted_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

/*
MessagePayload is a timeline entry as broadcasts show it. A deleted
entry keeps its place but loses its body.
*/
type MessagePayload struct {
	ID             string     `json:"id"`
	Object         string     `json:"object"`
	ConversationID string     `json:"conversation_id"`
	Type           string     `json:"type"`
	Author         AuthorRef  `json:"author"`
	Body           *string    `json:"body"`
	BodyHTML       *string    `json:"body_html"`
	DeliveryStatus *string    `json:"delivery_status"`
	EditedAt       *time.Time `json:"edited_at"`
	DeletedAt      *time.Time `json:"deleted_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

/*
AuthorRef is who wrote a timeline entry: its type and, for a member or
contact, their public id.
*/
type AuthorRef struct {
	Type string  `json:"type"`
	ID   *string `json:"id"`
}

/*
NewConversationPayload shapes a conversation for broadcasts.
*/
func NewConversationPayload(c models.Conversation) ConversationPayload {
	return ConversationPayload{
		ID:            models.PublicID(models.PrefixConversation, c.ID),
		Object:        "conversation",
		Number:        c.Number,
		Subject:       c.Subject,
		Status:        c.Status,
		Spam:          c.Spam,
		Priority:      c.Priority,
		AIState:       c.AiState,
		Inbox:         ref(models.PrefixInbox, "inbox", c.InboxID),
		Channel:       ref(models.PrefixChannelConnection, "channel_connection", c.ChannelConnectionID),
		Contact:       ref(models.PrefixContact, "contact", c.ContactID),
		Assignee:      optionalRef(models.PrefixMember, "member", c.AssigneeMemberID),
		Team:          optionalRef(models.PrefixTeam, "team", c.AssigneeTeamID),
		MessageCount:  c.MessageCount,
		LastMessageAt: c.LastMessageAt,
		SnoozedUntil:  c.SnoozedUntil,
		ClosedAt:      c.ClosedAt,
		DeletedAt:     c.DeletedAt,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

/*
NewMessagePayload shapes a timeline entry for broadcasts.
*/
func NewMessagePayload(m models.Message) MessagePayload {
	payload := MessagePayload{
		ID:             models.PublicID(models.PrefixMessage, m.ID),
		Object:         "message",
		ConversationID: models.PublicID(models.PrefixConversation, m.ConversationID),
		Type:           m.Type,
		Author:         authorRef(m.AuthorType, m.AuthorID),
		Body:           m.Body,
		BodyHTML:       m.BodyHtml,
		DeliveryStatus: m.DeliveryStatus,
		EditedAt:       m.EditedAt,
		DeletedAt:      m.DeletedAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}

	if m.DeletedAt != nil {
		payload.Body, payload.BodyHTML = nil, nil
	}

	return payload
}

/*
authorRef names the author of an entry. Only members and contacts have
ids clients can look up.
*/
func authorRef(authorType string, id *ids.UUID) AuthorRef {
	author := AuthorRef{Type: authorType}
	if id == nil {
		return author
	}

	var prefix models.Prefix

	switch enums.AuthorType(authorType) {
	case enums.AuthorTypeMember:
		prefix = models.PrefixMember
	case enums.AuthorTypeContact:
		prefix = models.PrefixContact
	case enums.AuthorTypeAgent, enums.AuthorTypeSystem, enums.AuthorTypeIntegration:
		return author
	}

	public := models.PublicID(prefix, *id)
	author.ID = &public

	return author
}

func ref(prefix models.Prefix, object string, id ids.UUID) Ref {
	return Ref{ID: models.PublicID(prefix, id), Object: object}
}

func optionalRef(prefix models.Prefix, object string, id *ids.UUID) *Ref {
	if id == nil {
		return nil
	}

	r := ref(prefix, object, *id)

	return &r
}
