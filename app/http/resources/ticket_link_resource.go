/*
|--------------------------------------------------------------------------
| Ticket Link Resource
|--------------------------------------------------------------------------
|
| A link from or to a ticket: back-office work, a tracker's report, a
| follow-up or a related ticket, with the ticket on its other end.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketLink is a link from or to a ticket, with the ticket on its other
end when it joins two tickets.
*/
type TicketLink struct {
	Object         string        `json:"object"`
	ID             string        `json:"id"`
	Kind           string        `json:"kind"`
	FromTicketID   string        `json:"from_ticket_id"`
	ToTicketID     *string       `json:"to_ticket_id"`
	ConversationID *string       `json:"conversation_id"`
	Ticket         *LinkedTicket `json:"ticket"`
	CreatedAt      time.Time     `json:"created_at"`
}

/*
LinkedTicket is the ticket on a link's other end.
*/
type LinkedTicket struct {
	ID         string         `json:"id"`
	Reference  string         `json:"reference"`
	Title      string         `json:"title"`
	Kind       string         `json:"kind"`
	TypeName   string         `json:"type_name"`
	State      TicketStateRef `json:"state"`
	AssigneeID *string        `json:"assignee_id"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

/*
NewTicketLinks shapes a ticket's links.
*/
func NewTicketLinks(list []tickets.LinkView) []TicketLink {
	out := make([]TicketLink, len(list))
	for i, v := range list {
		out[i] = NewTicketLink(v)
	}

	return out
}

/*
NewTicketLink shapes one link.
*/
func NewTicketLink(v tickets.LinkView) TicketLink {
	l := v.Link
	out := TicketLink{
		Object: "ticket_link", ID: models.PublicID(models.PrefixTicketLink, l.ID), Kind: l.Kind,
		FromTicketID:   models.PublicID(models.PrefixTicket, l.FromTicketID),
		ToTicketID:     optionalID(models.PrefixTicket, l.ToTicketID),
		ConversationID: optionalID(models.PrefixConversation, l.ConversationID), CreatedAt: l.CreatedAt,
	}

	if v.Other != nil && v.State != nil && v.Type != nil {
		out.Ticket = &LinkedTicket{
			ID: models.PublicID(models.PrefixTicket, v.Other.ID), Reference: tickets.Reference(v.Other.Number),
			Title: v.Other.Title, Kind: v.Other.Kind, TypeName: v.Type.Name, State: NewTicketStateRef(*v.State),
			AssigneeID: optionalID(models.PrefixMember, v.Other.AssigneeMemberID), UpdatedAt: v.Other.UpdatedAt,
		}
	}

	return out
}
