package format

import "time"

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
