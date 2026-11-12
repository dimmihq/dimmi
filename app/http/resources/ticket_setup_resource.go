/*
|--------------------------------------------------------------------------
| Ticket Setup Resources
|--------------------------------------------------------------------------
|
| How a workspace sets tickets up: its states, each in a fixed group
| with the label the customer sees; its types with their settings,
| fields, the states they use and how many tickets they hold; and the
| fields, typed and saying when each is required and what the customer
| sees of it.
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
TicketState is one state.
*/
type TicketState struct {
	Object         string     `json:"object"`
	ID             string     `json:"id"`
	Group          string     `json:"group"`
	Name           string     `json:"name"`
	CustomerLabel  string     `json:"customer_label"`
	NotifyCustomer bool       `json:"notify_customer"`
	Purpose        *string    `json:"purpose"`
	Position       int32      `json:"position"`
	ArchivedAt     *time.Time `json:"archived_at"`
}

/*
NewTicketState shapes a state.
*/
func NewTicketState(s models.TicketState) TicketState {
	return TicketState{
		Object: "ticket_state", ID: models.PublicID(models.PrefixTicketState, s.ID), Group: s.StateGroup, Name: s.Name,
		CustomerLabel: s.CustomerLabel, NotifyCustomer: s.NotifyCustomer, Purpose: s.Purpose, Position: s.Position,
		ArchivedAt: s.ArchivedAt,
	}
}

/*
NewTicketStates shapes states.
*/
func NewTicketStates(list []models.TicketState) []TicketState {
	out := make([]TicketState, len(list))
	for i, s := range list {
		out[i] = NewTicketState(s)
	}

	return out
}

/*
TicketField is one field of a type.
*/
type TicketField struct {
	Object                 string            `json:"object"`
	ID                     string            `json:"id"`
	TypeID                 string            `json:"type_id"`
	Key                    string            `json:"key"`
	Label                  string            `json:"label"`
	Description            string            `json:"description"`
	DataType               string            `json:"data_type"`
	Options                []string          `json:"options"`
	Currency               *string           `json:"currency"`
	RequiredCreate         bool              `json:"required_create"`
	RequiredCreateCustomer bool              `json:"required_create_customer"`
	RequiredResolve        bool              `json:"required_resolve"`
	VisibleCustomer        bool              `json:"visible_customer"`
	EditableCustomer       bool              `json:"editable_customer"`
	ShowWhen               *tickets.ShowWhen `json:"show_when"`
	AIHint                 *string           `json:"ai_hint"`
	AIAuto                 bool              `json:"ai_auto"`
	Position               int32             `json:"position"`
	ArchivedAt             *time.Time        `json:"archived_at"`
}

/*
NewTicketField shapes a field.
*/
func NewTicketField(f models.TicketField) TicketField {
	var sw *tickets.ShowWhen
	if len(f.ShowWhen) > 0 && string(f.ShowWhen) != "null" {
		var parsed tickets.ShowWhen
		if json.Unmarshal(f.ShowWhen, &parsed) == nil && parsed.Field != "" {
			sw = &parsed
		}
	}

	options := f.Options
	if options == nil {
		options = []string{}
	}

	return TicketField{
		Object: "ticket_field", ID: models.PublicID(models.PrefixTicketField, f.ID),
		TypeID: models.PublicID(models.PrefixTicketType, f.TypeID), Key: f.Key, Label: f.Label,
		Description: f.Description, DataType: f.DataType, Options: options, Currency: f.Currency,
		RequiredCreate: f.RequiredCreate, RequiredCreateCustomer: f.RequiredCreateCustomer,
		RequiredResolve: f.RequiredResolve, VisibleCustomer: f.VisibleCustomer, EditableCustomer: f.EditableCustomer,
		ShowWhen: sw, AIHint: f.AiHint, AIAuto: f.AiAuto, Position: f.Position, ArchivedAt: f.ArchivedAt,
	}
}

/*
NewTicketFields shapes fields.
*/
func NewTicketFields(list []models.TicketField) []TicketField {
	out := make([]TicketField, len(list))
	for i, f := range list {
		out[i] = NewTicketField(f)
	}

	return out
}

/*
TicketType is one type with its fields, the states it uses and its
ticket counts.
*/
type TicketType struct {
	Object             string        `json:"object"`
	ID                 string        `json:"id"`
	Kind               string        `json:"kind"`
	Name               string        `json:"name"`
	Icon               string        `json:"icon"`
	Description        string        `json:"description"`
	InboxID            *string       `json:"inbox_id"`
	TeamID             *string       `json:"team_id"`
	CustomerVisible    bool          `json:"customer_visible"`
	CompanySharing     bool          `json:"company_sharing"`
	ReopenDays         int32         `json:"reopen_days"`
	AutoResolveDays    *int32        `json:"auto_resolve_days"`
	AutoResolveMessage *string       `json:"auto_resolve_message"`
	AIAutoFill         bool          `json:"ai_auto_fill"`
	StateIDs           []string      `json:"state_ids"`
	DefaultStateID     *string       `json:"default_state_id"`
	DefaultResolvedID  *string       `json:"default_resolved_state_id"`
	Fields             []TicketField `json:"fields"`
	OpenTickets        int64         `json:"open_tickets"`
	TotalTickets       int64         `json:"total_tickets"`
	Position           int32         `json:"position"`
	ArchivedAt         *time.Time    `json:"archived_at"`
}

/*
NewTicketType shapes a type within its workspace's setup.
*/
func NewTicketType(t models.TicketType, setup tickets.Setup, counts []models.CountTicketsByTypeRow) TicketType {
	out := TicketType{
		Object: "ticket_type", ID: models.PublicID(models.PrefixTicketType, t.ID), Kind: t.Kind, Name: t.Name,
		Icon: t.Icon, Description: t.Description, InboxID: optionalID(models.PrefixInbox, t.InboxID),
		TeamID: optionalID(models.PrefixTeam, t.TeamID), CustomerVisible: t.CustomerVisible,
		CompanySharing: t.CompanySharing, ReopenDays: t.ReopenDays, AutoResolveDays: t.AutoResolveDays,
		AutoResolveMessage: t.AutoResolveMessage,
		AIAutoFill:         t.AiAutoFill, StateIDs: []string{}, Fields: NewTicketFields(setup.FieldsOf(t.ID)),
		Position: t.Position, ArchivedAt: t.ArchivedAt,
	}

	for _, s := range setup.Allowed(t.ID) {
		out.StateIDs = append(out.StateIDs, models.PublicID(models.PrefixTicketState, s.ID))
	}

	if s, ok := setup.DefaultState(t.ID); ok {
		out.DefaultStateID = new(models.PublicID(models.PrefixTicketState, s.ID))
	}

	if s, ok := setup.DefaultResolved(t.ID); ok {
		out.DefaultResolvedID = new(models.PublicID(models.PrefixTicketState, s.ID))
	}

	for _, c := range counts {
		if c.TypeID == t.ID {
			out.OpenTickets, out.TotalTickets = c.Open, c.Total
		}
	}

	return out
}

/*
NewTicketTypes shapes every type of the setup.
*/
func NewTicketTypes(setup tickets.Setup, counts []models.CountTicketsByTypeRow) []TicketType {
	out := make([]TicketType, len(setup.Types))
	for i, t := range setup.Types {
		out[i] = NewTicketType(t, setup, counts)
	}

	return out
}
