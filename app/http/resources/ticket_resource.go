/*
|--------------------------------------------------------------------------
| Ticket Resource
|--------------------------------------------------------------------------
|
| A ticket as the API shows it: its reference, kind, type and state with
| the label its customer sees, field values and AI suggestions, who
| works it, the conversation, customer and company it belongs to, its
| version for safe edits, and the issues behind it. One ticket read on
| its own also carries its links, running service level clocks and,
| for trackers and incidents, who they reach.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketStateRef is a state as a ticket names it.
*/
type TicketStateRef struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	CustomerLabel string `json:"customer_label"`
	Group         string `json:"group"`
}

/*
TicketTypeRef is a type as a ticket names it.
*/
type TicketTypeRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Kind string `json:"kind"`
}

/*
Ticket is one ticket. Links, SLAClocks and Impact come only when the
ticket is read on its own.
*/
type Ticket struct {
	Object              string         `json:"object"`
	ID                  string         `json:"id"`
	Reference           string         `json:"reference"`
	Number              int64          `json:"number"`
	Kind                string         `json:"kind"`
	TypeID              string         `json:"type_id"`
	Type                TicketTypeRef  `json:"type"`
	StateID             string         `json:"state_id"`
	State               TicketStateRef `json:"state"`
	Group               string         `json:"group"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	Fields              map[string]any `json:"fields"`
	Suggestions         map[string]any `json:"suggestions"`
	Priority            string         `json:"priority"`
	AssigneeID          *string        `json:"assignee_id"`
	TeamID              *string        `json:"team_id"`
	InboxID             string         `json:"inbox_id"`
	ConversationID      *string        `json:"conversation_id"`
	ContactID           *string        `json:"contact_id"`
	CompanyID           *string        `json:"company_id"`
	ExternalID          *string        `json:"external_id"`
	SLAPolicyID         *string        `json:"sla_policy_id"`
	CloseLoop           bool           `json:"close_loop"`
	CustomerVisible     bool           `json:"customer_visible"`
	Version             int32          `json:"version"`
	LinkedConversations int64          `json:"linked_conversations"`
	Issues              []TicketIssue  `json:"issues"`
	CreatedByType       string         `json:"created_by_type"`
	StateSince          time.Time      `json:"state_since"`
	ResolvedAt          *time.Time     `json:"resolved_at"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           *time.Time     `json:"deleted_at"`
	Links               []TicketLink   `json:"links,omitempty"`
	SLAClocks           []SLAClock     `json:"sla_clocks,omitempty"`
	Impact              *TicketImpact  `json:"impact,omitempty"`
}

/*
TicketImpact is who a tracker or incident reaches.
*/
type TicketImpact struct {
	Conversations int64   `json:"conversations"`
	Customers     int64   `json:"customers"`
	Companies     int64   `json:"companies"`
	Revenue       float64 `json:"revenue"`
}

/*
NewTicket shapes a ticket from a list.
*/
func NewTicket(s tickets.Summary) Ticket {
	t := s.Ticket

	out := Ticket{
		Object: "ticket", ID: models.PublicID(models.PrefixTicket, t.ID), Reference: tickets.Reference(t.Number),
		Number: t.Number, Kind: t.Kind, TypeID: models.PublicID(models.PrefixTicketType, t.TypeID),
		Type: TicketTypeRef{
			ID: models.PublicID(models.PrefixTicketType, s.Type.ID), Name: s.Type.Name, Icon: s.Type.Icon, Kind: t.Kind,
		},
		StateID: models.PublicID(models.PrefixTicketState, t.StateID), State: NewTicketStateRef(s.State),
		Group: t.StateGroup, Title: t.Title, Description: t.Description, Fields: object(t.Fields),
		Suggestions: object(t.Suggestions), Priority: t.Priority,
		AssigneeID:     optionalID(models.PrefixMember, t.AssigneeMemberID),
		TeamID:         optionalID(models.PrefixTeam, t.AssigneeTeamID),
		InboxID:        models.PublicID(models.PrefixInbox, t.InboxID),
		ConversationID: optionalID(models.PrefixConversation, t.ConversationID),
		ContactID:      optionalID(models.PrefixContact, t.ContactID),
		CompanyID:      optionalID(models.PrefixCompany, t.CompanyID), ExternalID: t.ExternalID,
		SLAPolicyID: optionalID(models.PrefixSLAPolicy, t.SlaPolicyID), CloseLoop: t.CloseLoop,
		CustomerVisible: t.CustomerVisible, Version: t.Version, LinkedConversations: s.Linked,
		Issues: NewTicketIssues(s.Issues), CreatedByType: t.CreatedByType, StateSince: t.StateSince,
		ResolvedAt: t.ResolvedAt, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, DeletedAt: t.DeletedAt,
	}

	return out
}

/*
NewTickets shapes a page of tickets.
*/
func NewTickets(list []tickets.Summary) []Ticket {
	out := make([]Ticket, len(list))
	for i, s := range list {
		out[i] = NewTicket(s)
	}

	return out
}

/*
NewTicketDetail shapes one ticket with its links, clocks and impact.
*/
func NewTicketDetail(d tickets.Detail) Ticket {
	out := NewTicket(d.Summary)
	out.Links = NewTicketLinks(d.Links)
	out.SLAClocks = NewSLAClocks(d.Clocks)

	if d.Impact != nil {
		out.Impact = &TicketImpact{
			Conversations: d.Impact.Conversations, Customers: d.Impact.Customers, Companies: d.Impact.Companies,
			Revenue: d.Impact.Revenue,
		}
	}

	return out
}

/*
NewTicketStateRef names a state.
*/
func NewTicketStateRef(s models.TicketState) TicketStateRef {
	return TicketStateRef{
		ID: models.PublicID(models.PrefixTicketState, s.ID), Name: s.Name, CustomerLabel: s.CustomerLabel,
		Group: s.StateGroup,
	}
}

/*
object reads a stored JSON object.
*/
func object(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}

	if out == nil {
		out = map[string]any{}
	}

	return out
}
