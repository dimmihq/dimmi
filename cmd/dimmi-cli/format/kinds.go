package format

import (
	"encoding/json"
	"time"
)

/*
Author is who wrote an entry or made a change: a teammate or a contact
by source id, an AI agent with the label it showed, or the system.
*/
type Author struct {
	Type  string `json:"type"`
	Ref   string `json:"ref,omitempty"`
	Label string `json:"label,omitempty"`
}

/*
The kinds of author.
*/
const (
	AuthorTeammate = "teammate"
	AuthorContact  = "contact"
	AuthorAIAgent  = "ai_agent"
	AuthorSystem   = "system"
)

/*
Teammate is a person who worked in the source tool. Teammates arrive
disabled; an admin enables them. Current says whether the source still
lists them.
*/
type Teammate struct {
	Base
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Role      string `json:"role,omitempty"`
	Current   *bool  `json:"current,omitempty"`
}

/*
Team is a group of teammates.
*/
type Team struct {
	Base
	Name       string   `json:"name"`
	MemberRefs []string `json:"member_refs,omitempty"`
}

/*
Tag is a label put on contacts, companies and conversations.
*/
type Tag struct {
	Base
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

/*
The objects an attribute describes.
*/
const (
	ObjectContact      = "contact"
	ObjectCompany      = "company"
	ObjectConversation = "conversation"
)

/*
The attribute types.
*/
const (
	TypeText     = "text"
	TypeNumber   = "number"
	TypeDecimal  = "decimal"
	TypeBoolean  = "boolean"
	TypeDate     = "date"
	TypeList     = "list"
	TypeFiles    = "files"
	TypeRelation = "relationship"
)

/*
AttributeDefinition is a custom field of contacts, companies or
conversations.
*/
type AttributeDefinition struct {
	Base
	Object      string   `json:"object"`
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
	Archived    bool     `json:"archived,omitempty"`
	Multiline   bool     `json:"multiline,omitempty"`
	Required    bool     `json:"required_to_close,omitempty"`
}

/*
Company is an organisation contacts belong to.
*/
type Company struct {
	Base
	ExternalID       string         `json:"external_id,omitempty"`
	Name             string         `json:"name"`
	Domain           string         `json:"domain,omitempty"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
	TagRefs          []string       `json:"tag_refs,omitempty"`
	CreatedAt        *time.Time     `json:"created_at,omitempty"`
}

/*
CompanyLink is one company a contact belongs to; one link is the
contact's default.
*/
type CompanyLink struct {
	Ref     string `json:"ref"`
	Default bool   `json:"default,omitempty"`
}

/*
Location is where a contact was last seen.
*/
type Location struct {
	City    string `json:"city,omitempty"`
	Region  string `json:"region,omitempty"`
	Country string `json:"country,omitempty"`
}

/*
Contact is a customer: a signed-up user or an anonymous lead.
*/
type Contact struct {
	Base
	Kind              string         `json:"kind,omitempty"`
	ExternalID        string         `json:"external_id,omitempty"`
	Email             string         `json:"email,omitempty"`
	Name              string         `json:"name,omitempty"`
	Phone             string         `json:"phone,omitempty"`
	AvatarURL         string         `json:"avatar_url,omitempty"`
	Locale            string         `json:"locale,omitempty"`
	Timezone          string         `json:"timezone,omitempty"`
	Location          *Location      `json:"location,omitempty"`
	FirstSeenAt       *time.Time     `json:"first_seen_at,omitempty"`
	LastSeenAt        *time.Time     `json:"last_seen_at,omitempty"`
	CreatedAt         *time.Time     `json:"created_at,omitempty"`
	CustomAttributes  map[string]any `json:"custom_attributes,omitempty"`
	TagRefs           []string       `json:"tag_refs,omitempty"`
	Companies         []CompanyLink  `json:"companies,omitempty"`
	EmailUnsubscribed bool           `json:"email_unsubscribed,omitempty"`
}

/*
ContactNote is a teammate's note about a contact or a company.
*/
type ContactNote struct {
	Base
	ContactRef string    `json:"contact_ref,omitempty"`
	CompanyRef string    `json:"company_ref,omitempty"`
	Author     Author    `json:"author"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

/*
The ticket type kinds.
*/
const (
	TicketCustomer   = "customer"
	TicketBackOffice = "back_office"
	TicketTracker    = "tracker"
)

/*
TicketType is a kind of ticket with its fields and the states it moves
through, the first state its starting one.
*/
type TicketType struct {
	Base
	Kind        string        `json:"kind"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Fields      []TicketField `json:"fields,omitempty"`
	States      []TicketState `json:"states"`
}

/*
TicketField is one field of a ticket type.
*/
type TicketField struct {
	SourceID        string   `json:"source_id"`
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Type            string   `json:"type"`
	Options         []string `json:"options,omitempty"`
	CustomerVisible bool     `json:"customer_visible,omitempty"`
	Required        bool     `json:"required,omitempty"`
	Multiline       bool     `json:"multiline,omitempty"`
}

/*
TicketState is a state of a ticket type, in one of the groups
submitted, in_progress, waiting_on_customer or resolved.
*/
type TicketState struct {
	SourceID      string `json:"source_id"`
	Group         string `json:"group"`
	Name          string `json:"name"`
	CustomerLabel string `json:"customer_label,omitempty"`
}

/*
Ticket is one ticket, with the states it went through.
*/
type Ticket struct {
	Base
	TicketTypeRef   string         `json:"ticket_type_ref"`
	StateRef        string         `json:"state_ref"`
	Title           string         `json:"title"`
	Description     string         `json:"description,omitempty"`
	Values          map[string]any `json:"values,omitempty"`
	Priority        string         `json:"priority,omitempty"`
	ContactRef      string         `json:"contact_ref,omitempty"`
	CompanyRef      string         `json:"company_ref,omitempty"`
	AssigneeRef     string         `json:"assignee_ref,omitempty"`
	TeamRef         string         `json:"team_ref,omitempty"`
	ConversationRef string         `json:"conversation_ref,omitempty"`
	TagRefs         []string       `json:"tag_refs,omitempty"`
	Author          *Author        `json:"author,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       *time.Time     `json:"updated_at,omitempty"`
	StateHistory    []StateMove    `json:"state_history,omitempty"`
	Entries         []Entry        `json:"entries,omitempty"`
	EntriesCapped   bool           `json:"entries_capped,omitempty"`
}

/*
StateMove is a ticket entering a state at a time.
*/
type StateMove struct {
	StateRef string    `json:"state_ref"`
	At       time.Time `json:"at"`
}

/*
HelpCenter is a help center with its default language and the others it
is translated into.
*/
type HelpCenter struct {
	Base
	Name          string   `json:"name"`
	DefaultLocale string   `json:"default_locale,omitempty"`
	Locales       []string `json:"locales,omitempty"`
	URL           string   `json:"url,omitempty"`
}

/*
Translation is a collection's or an article's text in another language.
*/
type Translation struct {
	Locale      string     `json:"locale"`
	Name        string     `json:"name,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Body        string     `json:"body,omitempty"`
	State       string     `json:"state,omitempty"`
	Author      *Author    `json:"author,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	Redirects   []string   `json:"redirects,omitempty"`
}

/*
Collection is a group of articles, nested under another or at the top.
*/
type Collection struct {
	Base
	HelpCenterRef string        `json:"help_center_ref"`
	ParentRef     string        `json:"parent_ref,omitempty"`
	Name          string        `json:"name"`
	Description   string        `json:"description,omitempty"`
	Icon          string        `json:"icon,omitempty"`
	Order         int           `json:"order,omitempty"`
	Translations  []Translation `json:"translations,omitempty"`
}

/*
The article visibilities.
*/
const (
	VisibilityPublic    = "public"
	VisibilityAgentOnly = "agent_only"
)

/*
Article is a help center article. Visibility is public or agent_only;
Redirects are the old paths that should lead to it. Blocks lists the
kinds of rich block its body uses.
*/
type Article struct {
	Base
	HelpCenterRef string        `json:"help_center_ref"`
	CollectionRef string        `json:"collection_ref,omitempty"`
	Title         string        `json:"title"`
	Description   string        `json:"description,omitempty"`
	Body          string        `json:"body"`
	State         string        `json:"state,omitempty"`
	Visibility    string        `json:"visibility,omitempty"`
	Audience      string        `json:"audience,omitempty"`
	Author        *Author       `json:"author,omitempty"`
	CreatedAt     *time.Time    `json:"created_at,omitempty"`
	UpdatedAt     *time.Time    `json:"updated_at,omitempty"`
	Redirects     []string      `json:"redirects,omitempty"`
	Translations  []Translation `json:"translations,omitempty"`
	Blocks        []string      `json:"blocks,omitempty"`
	TagRefs       []string      `json:"tag_refs,omitempty"`
}

/*
Snippet is a piece of text teammates and the AI agent reuse; it becomes
an article only teammates see.
*/
type Snippet struct {
	Base
	HelpCenterRef string     `json:"help_center_ref,omitempty"`
	Title         string     `json:"title"`
	Body          string     `json:"body"`
	Locale        string     `json:"locale,omitempty"`
	Author        *Author    `json:"author,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
}

/*
SavedReply is a canned answer: a Liquid body and what sending it does.
TeamRefs limits it to teams; Unknown lists placeholders the source used
that have no dimmi equivalent, left in the body as written.
*/
type SavedReply struct {
	Base
	Title    string        `json:"title"`
	Body     string        `json:"body"`
	Shortcut string        `json:"shortcut,omitempty"`
	Shared   *bool         `json:"shared,omitempty"`
	OwnerRef string        `json:"owner_ref,omitempty"`
	TeamRefs []string      `json:"team_refs,omitempty"`
	Actions  []ReplyAction `json:"actions,omitempty"`
	Unknown  []string      `json:"unknown_placeholders,omitempty"`
}

/*
ReplyAction is what a saved reply does when sent: add a tag, assign, or
change the status.
*/
type ReplyAction struct {
	Type        string `json:"type"`
	TagRef      string `json:"tag_ref,omitempty"`
	AssigneeRef string `json:"assignee_ref,omitempty"`
	TeamRef     string `json:"team_ref,omitempty"`
	Status      string `json:"status,omitempty"`
}

/*
Schedule is office hours in a time zone with the days off. Exceptions
are days with other hours, which Holidays can't hold.
*/
type Schedule struct {
	Base
	Name       string      `json:"name"`
	Timezone   string      `json:"timezone"`
	Weekly     []Hours     `json:"weekly"`
	Holidays   []Holiday   `json:"holidays,omitempty"`
	Exceptions []Exception `json:"exceptions,omitempty"`
	Default    bool        `json:"default,omitempty"`
}

/*
Hours is one stretch of a weekday, its times as HH:MM.
*/
type Hours struct {
	Day   string `json:"day"`
	Start string `json:"start"`
	End   string `json:"end"`
}

/*
Holiday is a day off, YYYY-MM-DD; Yearly repeats it every year.
*/
type Holiday struct {
	Date   string `json:"date"`
	Name   string `json:"name,omitempty"`
	Yearly bool   `json:"yearly,omitempty"`
}

/*
Exception is a day that differs from the week: closed, or open from
Start to End, HH:MM.
*/
type Exception struct {
	Date   string `json:"date"`
	Name   string `json:"name,omitempty"`
	Closed bool   `json:"closed,omitempty"`
	Start  string `json:"start,omitempty"`
	End    string `json:"end,omitempty"`
	Yearly bool   `json:"yearly,omitempty"`
}

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

/*
Event is something that happened to a conversation: an assignment, a
team, a status, a priority, tags, an attribute, or anything else the
source tool recorded, kept with its own type and text.
*/
type Event struct {
	Type          string     `json:"type"`
	AssigneeRef   *string    `json:"assignee_ref,omitempty"`
	TeamRef       *string    `json:"team_ref,omitempty"`
	Status        string     `json:"status,omitempty"`
	Until         *time.Time `json:"until,omitempty"`
	Priority      string     `json:"priority,omitempty"`
	AddedTagRefs  []string   `json:"added_tag_refs,omitempty"`
	RemovedTagRef []string   `json:"removed_tag_refs,omitempty"`
	Attribute     string     `json:"attribute,omitempty"`
	Value         any        `json:"value,omitempty"`
	RawType       string     `json:"raw_type,omitempty"`
	Text          string     `json:"text,omitempty"`
}

/*
The kinds of event.
*/
const (
	EventAssignment     = "assignment"
	EventTeamAssignment = "team_assignment"
	EventStatus         = "status"
	EventPriority       = "priority"
	EventTag            = "tag"
	EventAttribute      = "attribute"
	EventCustom         = "custom"
)

/*
Workflow is a source automation, imported disabled. Definition is the
source's own export; Unsupported lists the step types dimmi can't run
yet, so the person rebuilding it knows what to look at.
*/
type Workflow struct {
	Base
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Trigger     string          `json:"trigger,omitempty"`
	Channels    []string        `json:"channels,omitempty"`
	Enabled     bool            `json:"enabled"`
	Steps       []WorkflowStep  `json:"steps"`
	Unsupported []string        `json:"unsupported,omitempty"`
	Definition  json.RawMessage `json:"definition,omitempty"`
}

/*
WorkflowStep is one step of a workflow, in the source's terms, with the
dimmi step it becomes when there is one.
*/
type WorkflowStep struct {
	SourceType string `json:"source_type"`
	Type       string `json:"type,omitempty"`
	Supported  bool   `json:"supported"`
}
