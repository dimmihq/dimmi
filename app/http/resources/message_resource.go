/*
|--------------------------------------------------------------------------
| Message Resources
|--------------------------------------------------------------------------
|
| An entry of a conversation's timeline: a customer message, a reply, a
| note or an event, with who it is from and, for a reply, how its
| delivery went. A deleted entry keeps its place with its text gone. An
| author the workspace no longer has still shows, as a former teammate.
| An event carries what happened, for the timeline to say in words.
|
*/

package resources

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
excerptLength is how many characters of a message its short form
shows.
*/
const excerptLength = 140

/*
TimelineEntry is one entry of a conversation's timeline. ScheduledSendAt
is set while a reply waits to leave and its writer may still undo it.
*/
type TimelineEntry struct {
	ID              string          `json:"id"`
	Object          string          `json:"object"`
	ConversationID  string          `json:"conversation_id"`
	Type            string          `json:"type"`
	Author          Author          `json:"author"`
	Body            *string         `json:"body"`
	DeliveryStatus  *string         `json:"delivery_status"`
	DeliveryError   *string         `json:"delivery_error"`
	Substantive     bool            `json:"substantive"`
	Event           json.RawMessage `json:"event"`
	SentAt          *time.Time      `json:"sent_at"`
	ScheduledSendAt *time.Time      `json:"scheduled_send_at"`
	EditedAt        *time.Time      `json:"edited_at"`
	DeletedAt       *time.Time      `json:"deleted_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

/*
Authors are the members who wrote a page of entries, by id.
*/
type Authors = map[ids.UUID]conversations.Member

/*
Author is who wrote an entry: their type, public id and name. The
system and former teammates have no id.
*/
type Author struct {
	Type      string  `json:"type"`
	ID        *string `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

/*
NewTimelineEntry shapes the entry, its author found among the conversation's
contact and the members given.
*/
func NewTimelineEntry(m models.Message, contact models.Contact, members Authors) TimelineEntry {
	var by *conversations.Member

	if m.AuthorID != nil {
		if member, ok := members[*m.AuthorID]; ok {
			by = &member
		}
	}

	out := TimelineEntry{
		ID: models.PublicID(models.PrefixMessage, m.ID), Object: "message",
		ConversationID: models.PublicID(models.PrefixConversation, m.ConversationID), Type: m.Type,
		Author: author(m, contact, by), Body: m.Body, DeliveryStatus: m.DeliveryStatus,
		DeliveryError: m.DeliveryError, Substantive: m.Substantive, Event: m.Event, SentAt: m.SentAt,
		ScheduledSendAt: m.ScheduledSendAt, EditedAt: m.EditedAt, DeletedAt: m.DeletedAt, CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}

	if m.DeletedAt != nil {
		out.Body = nil
	}

	if len(out.Event) == 0 {
		out.Event = json.RawMessage(`null`)
	}

	return out
}

/*
NewTimelineEntries shapes a page of entries, in the order given.
*/
func NewTimelineEntries(rows []models.Message, contact models.Contact, members Authors) []TimelineEntry {
	out := make([]TimelineEntry, 0, len(rows))
	for _, m := range rows {
		out = append(out, NewTimelineEntry(m, contact, members))
	}

	return out
}

/*
author names who wrote the entry: the contact, the member when known, a
former teammate when not, or the system.
*/
func author(m models.Message, contact models.Contact, member *conversations.Member) Author {
	switch m.AuthorType {
	case string(enums.AuthorTypeContact):
		name := "Customer"
		if contact.Name != nil {
			name = *contact.Name
		} else if contact.Email != nil {
			name = *contact.Email
		}

		return Author{Type: m.AuthorType, ID: new(models.PublicID(models.PrefixContact, contact.ID)), Name: name}
	case string(enums.AuthorTypeMember):
		if member == nil {
			return Author{Type: m.AuthorType, Name: "Former teammate"}
		}

		return Author{
			Type: m.AuthorType, ID: new(models.PublicID(models.PrefixMember, member.Member.ID)), Name: member.User.Name,
		}
	}

	return Author{Type: m.AuthorType, Name: "dimmi"}
}

/*
excerpt returns the start of a message as plain text, its Markdown
taken out, cut short where it is long.
*/
func excerpt(body *string) string {
	if body == nil {
		return ""
	}

	plain := strings.Join(strings.Fields(markdown.PlainText([]byte(*body))), " ")

	runes := []rune(plain)
	if len(runes) <= excerptLength {
		return plain
	}

	return string(runes[:excerptLength]) + "…"
}
