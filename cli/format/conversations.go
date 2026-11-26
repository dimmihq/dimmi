package format

import "time"

/*
Conversation is a conversation with every entry it had, oldest first,
and the state it ended in.
*/
type Conversation struct {
	Base
	ContactRef      string             `json:"contact_ref"`
	ParticipantRefs []string           `json:"participant_refs,omitempty"`
	CompanyRef      string             `json:"company_ref,omitempty"`
	Channel         string             `json:"channel,omitempty"`
	Subject         string             `json:"subject,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	Source          ConversationSource `json:"source,omitzero"`
	Status          string             `json:"status,omitempty"`
	SnoozedUntil    *time.Time         `json:"snoozed_until,omitempty"`
	Priority        string             `json:"priority,omitempty"`
	AssigneeRef     string             `json:"assignee_ref,omitempty"`
	TeamRef         string             `json:"team_ref,omitempty"`
	TagRefs         []string           `json:"tag_refs,omitempty"`
	Attributes      map[string]any     `json:"custom_attributes,omitempty"`
	Rating          *Rating            `json:"rating,omitempty"`
	Entries         []Entry            `json:"entries"`
}

/*
ConversationSource is where the conversation came from in the source
tool: its own name for the channel, and a reference to the anonymous
visitor who started it.
*/
type ConversationSource struct {
	Type       string `json:"type,omitempty"`
	VisitorRef string `json:"visitor_ref,omitempty"`
	URL        string `json:"url,omitempty"`
}

/*
Rating is how the customer rated the conversation once it closed.
*/
type Rating struct {
	Score       int       `json:"score"`
	Remark      string    `json:"remark,omitempty"`
	TeammateRef string    `json:"teammate_ref,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

/*
Entry is one entry of a conversation: a customer's message, a reply, an
internal note, or an event. Attachments name uploaded files by source
id.
*/
type Entry struct {
	SourceID    string     `json:"source_id"`
	Type        string     `json:"type"`
	CreatedAt   time.Time  `json:"created_at"`
	Author      Author     `json:"author"`
	Body        string     `json:"body,omitempty"`
	Attachments []string   `json:"attachments,omitempty"`
	Email       *EmailRefs `json:"email,omitempty"`
	Redacted    bool       `json:"redacted,omitempty"`
	Event       *Event     `json:"event,omitempty"`
}

/*
The kinds of entry.
*/
const (
	EntryMessage = "message"
	EntryReply   = "reply"
	EntryNote    = "note"
	EntryEvent   = "event"
)

/*
EmailRefs are the headers an email entry had, so a customer replying to
an old thread lands in this conversation.
*/
type EmailRefs struct {
	MessageID  string   `json:"message_id,omitempty"`
	InReplyTo  string   `json:"in_reply_to,omitempty"`
	References []string `json:"references,omitempty"`
}
