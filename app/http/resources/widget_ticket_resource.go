/*
|--------------------------------------------------------------------------
| Widget Ticket Resources
|--------------------------------------------------------------------------
|
| Requests as their customer sees them: the types they may open with
| only the fields a customer sees, and their requests with the customer
| label of each state, never the team's name for it, the visible field
| values and the states the request went through.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WidgetTicketState is a state as the customer reads it: its label and
the group it belongs to, which orders the progress strip.
*/
type WidgetTicketState struct {
	Label string     `json:"label"`
	Group string     `json:"group"`
	At    *time.Time `json:"at,omitempty"`
}

/*
WidgetTicketField is a field of the customer's form, or of their
request with its value.
*/
type WidgetTicketField struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Description string            `json:"description,omitempty"`
	DataType    string            `json:"data_type"`
	Options     []string          `json:"options"`
	Currency    *string           `json:"currency,omitempty"`
	Required    bool              `json:"required"`
	Editable    bool              `json:"editable"`
	ShowWhen    *tickets.ShowWhen `json:"show_when"`
	Value       any               `json:"value,omitempty"`
}

/*
WidgetTicketType is a type the customer may open.
*/
type WidgetTicketType struct {
	ID             string              `json:"id"`
	Object         string              `json:"object"`
	Name           string              `json:"name"`
	Icon           string              `json:"icon"`
	Description    string              `json:"description"`
	CompanySharing bool                `json:"company_sharing"`
	Fields         []WidgetTicketField `json:"fields"`
	States         []WidgetTicketState `json:"states"`
}

/*
NewWidgetTicketType shapes a type the customer may open.
*/
func NewWidgetTicketType(t tickets.CustomerType) WidgetTicketType {
	out := WidgetTicketType{
		ID: models.PublicID(models.PrefixTicketType, t.Type.ID), Object: "ticket_type", Name: t.Type.Name,
		Icon: t.Type.Icon, Description: t.Type.Description, CompanySharing: t.Type.CompanySharing,
		Fields: widgetFields(t.Fields, nil), States: []WidgetTicketState{},
	}

	for _, s := range t.States {
		out.States = append(out.States, WidgetTicketState{Label: s.CustomerLabel, Group: s.StateGroup})
	}

	return out
}

/*
widgetFields shapes the fields the customer sees, with their values
when the request has them.
*/
func widgetFields(fields []models.TicketField, values map[string]any) []WidgetTicketField {
	out := []WidgetTicketField{}

	for _, f := range fields {
		var sw *tickets.ShowWhen
		if len(f.ShowWhen) > 0 && string(f.ShowWhen) != "null" {
			_ = json.Unmarshal(f.ShowWhen, &sw)
		}

		options := f.Options
		if options == nil {
			options = []string{}
		}

		out = append(out, WidgetTicketField{
			Key: f.Key, Label: f.Label, Description: f.Description, DataType: f.DataType, Options: options,
			Currency: f.Currency, Required: f.RequiredCreateCustomer, Editable: f.EditableCustomer, ShowWhen: sw,
			Value: values[f.Key],
		})
	}

	return out
}

/*
WidgetTicketBlock is the request a conversation carries, as its list
row and thread show it.
*/
type WidgetTicketBlock struct {
	ID        string            `json:"id"`
	Reference string            `json:"reference"`
	Title     string            `json:"title"`
	State     WidgetTicketState `json:"state"`
}

/*
NewWidgetTicketBlock shapes the request a conversation carries.
*/
func NewWidgetTicketBlock(b tickets.Badge) *WidgetTicketBlock {
	return &WidgetTicketBlock{
		ID: models.PublicID(models.PrefixTicket, b.Ticket.ID), Reference: tickets.Reference(b.Ticket.Number),
		Title: b.Ticket.Title, State: WidgetTicketState{Label: b.State.CustomerLabel, Group: b.State.StateGroup},
	}
}

/*
WidgetTicket is one of the customer's requests. Mine is false for a
colleague's request the company shares.
*/
type WidgetTicket struct {
	WidgetTicketBlock

	Object         string              `json:"object"`
	Description    string              `json:"description"`
	Type           WidgetTicketType    `json:"type"`
	ConversationID *string             `json:"conversation_id"`
	Mine           bool                `json:"mine"`
	Fields         []WidgetTicketField `json:"fields"`
	History        []WidgetTicketState `json:"history"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
	ResolvedAt     *time.Time          `json:"resolved_at"`
}

/*
NewWidgetTicket shapes one of the customer's requests for the contact.
*/
func NewWidgetTicket(t tickets.CustomerTicket, contact ids.UUID) WidgetTicket {
	r := t.Ticket
	values := map[string]any{}
	_ = json.Unmarshal(r.Fields, &values)

	out := WidgetTicket{
		WidgetTicketBlock: *NewWidgetTicketBlock(tickets.Badge{Ticket: r, State: t.State}),
		Object:            "ticket", Description: r.Description,
		Type: WidgetTicketType{
			ID: models.PublicID(models.PrefixTicketType, t.Type.ID), Object: "ticket_type", Name: t.Type.Name,
			Icon: t.Type.Icon, Description: t.Type.Description, CompanySharing: t.Type.CompanySharing,
			Fields: []WidgetTicketField{}, States: []WidgetTicketState{},
		},
		Mine:   r.ContactID != nil && *r.ContactID == contact,
		Fields: widgetFields(t.Fields, values), History: []WidgetTicketState{},
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(), ResolvedAt: r.ResolvedAt,
	}

	if r.ConversationID != nil {
		id := models.PublicID(models.PrefixConversation, *r.ConversationID)
		out.ConversationID = &id
	}

	for _, s := range t.History {
		at := s.At.UTC()
		out.History = append(out.History, WidgetTicketState{Label: s.Label, Group: s.Group, At: &at})
	}

	return out
}
