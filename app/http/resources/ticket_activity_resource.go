/*
|--------------------------------------------------------------------------
| Ticket Activity Resources
|--------------------------------------------------------------------------
|
| What happened on a ticket: its event log entries, and the
| conversations a tracker reaches with their customer, company, plan
| and revenue.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketEvent is one entry of a ticket's history.
*/
type TicketEvent struct {
	Object    string         `json:"object"`
	ID        string         `json:"id"`
	TicketID  string         `json:"ticket_id"`
	Reference *string        `json:"reference,omitempty"`
	Kind      string         `json:"kind"`
	ActorType string         `json:"actor_type"`
	ActorID   *string        `json:"actor_id"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"created_at"`
}

/*
NewTicketEvent shapes a history entry.
*/
func NewTicketEvent(e models.TicketEvent) TicketEvent {
	var actor *string
	if e.ActorID != nil {
		switch e.ActorType {
		case "member":
			actor = optionalID(models.PrefixMember, e.ActorID)
		case "contact":
			actor = optionalID(models.PrefixContact, e.ActorID)
		}
	}

	return TicketEvent{
		Object: "ticket_event", ID: models.PublicID(models.PrefixTicketEvent, e.ID),
		TicketID: models.PublicID(models.PrefixTicket, e.TicketID), Kind: e.Kind, ActorType: e.ActorType,
		ActorID: actor, Data: object(e.Data), CreatedAt: e.CreatedAt,
	}
}

/*
NewTicketEvents shapes history entries.
*/
func NewTicketEvents(list []models.TicketEvent) []TicketEvent {
	out := make([]TicketEvent, len(list))
	for i, e := range list {
		out[i] = NewTicketEvent(e)
	}

	return out
}

/*
NewExportEvents shapes the workspace's log with each ticket's
reference.
*/
func NewExportEvents(list []tickets.ExportEvent) []TicketEvent {
	out := make([]TicketEvent, len(list))
	for i, e := range list {
		out[i] = NewTicketEvent(e.TicketEvent)
		out[i].Reference = new(tickets.Reference(e.Number))
	}

	return out
}

/*
TrackerConversation is a conversation a tracker reaches.
*/
type TrackerConversation struct {
	Object             string     `json:"object"`
	LinkID             string     `json:"link_id"`
	ConversationID     string     `json:"conversation_id"`
	ConversationNumber int64      `json:"conversation_number"`
	Subject            *string    `json:"subject"`
	Status             string     `json:"status"`
	ContactID          string     `json:"contact_id"`
	ContactName        *string    `json:"contact_name"`
	ContactEmail       *string    `json:"contact_email"`
	CompanyID          *string    `json:"company_id"`
	CompanyName        *string    `json:"company_name"`
	CompanyPlan        *string    `json:"company_plan"`
	CompanyRevenue     float64    `json:"company_revenue"`
	TicketID           *string    `json:"ticket_id"`
	TicketReference    *string    `json:"ticket_reference"`
	TicketStateID      *string    `json:"ticket_state_id"`
	LastMessageAt      *time.Time `json:"last_message_at"`
	LinkedAt           time.Time  `json:"linked_at"`
}

/*
NewTrackerConversations shapes a tracker's conversations.
*/
func NewTrackerConversations(list []tickets.LinkedConversation) []TrackerConversation {
	out := make([]TrackerConversation, len(list))
	for i, r := range list {
		out[i] = TrackerConversation{
			Object: "tracker_conversation", LinkID: models.PublicID(models.PrefixTicketLink, r.LinkID),
			ConversationID:     models.PublicID(models.PrefixConversation, r.ConversationID),
			ConversationNumber: r.ConversationNumber, Subject: r.Subject, Status: r.Status,
			ContactID: models.PublicID(models.PrefixContact, r.ContactID), ContactName: r.ContactName,
			ContactEmail: r.ContactEmail, CompanyID: optionalID(models.PrefixCompany, r.CompanyID),
			CompanyName: r.CompanyName, CompanyPlan: planOf(r.CompanyPlan), CompanyRevenue: r.CompanyRevenue,
			TicketID:      optionalID(models.PrefixTicket, r.TicketID),
			TicketStateID: optionalID(models.PrefixTicketState, r.TicketStateID),
			LastMessageAt: r.LastMessageAt, LinkedAt: r.LinkedAt,
		}

		if r.TicketNumber != nil {
			out[i].TicketReference = new(tickets.Reference(*r.TicketNumber))
		}
	}

	return out
}

/*
planOf returns a company's plan, nil when it has none.
*/
func planOf(plan string) *string {
	if plan == "" {
		return nil
	}

	return &plan
}
