/*
|--------------------------------------------------------------------------
| Setup
|--------------------------------------------------------------------------
|
| A workspace's types, fields and states, read once per transaction. A
| workspace that has none yet gets the defaults the first time anything
| reads them: the five groups' starting states, the states the product
| moves tickets to on its own (close the loop, being worked on, the
| incident stages), and one type of each kind, so a new workspace can
| convert, escalate, track and declare an incident from the start.
|
*/

package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The state groups, in the order work moves through them.
*/
const (
	GroupSubmitted       = "submitted"
	GroupInProgress      = "in_progress"
	GroupWaitingCustomer = "waiting_customer"
	GroupWaitingTeam     = "waiting_team"
	GroupResolved        = "resolved"
)

/*
Groups returns every state group in order.
*/
func Groups() []string {
	return []string{GroupSubmitted, GroupInProgress, GroupWaitingCustomer, GroupWaitingTeam, GroupResolved}
}

/*
The ticket kinds.
*/
const (
	KindCustomer   = "customer"
	KindBackOffice = "back_office"
	KindTracker    = "tracker"
	KindIncident   = "incident"
)

/*
Kinds returns every kind.
*/
func Kinds() []string {
	return []string{KindCustomer, KindBackOffice, KindTracker, KindIncident}
}

/*
The purposes of the states the product moves tickets to by itself.
*/
const (
	PurposeCloseLoop     = "close_loop"
	PurposeBeingWorked   = "being_worked_on"
	PurposeWontFix       = "wont_fix"
	PurposeIdentified    = "incident_identified"
	PurposeMonitoring    = "incident_monitoring"
	PurposeInvestigating = "investigating"
)

/*
Setup is a workspace's ticket configuration.
*/
type Setup struct {
	States     []models.TicketState
	Types      []models.TicketType
	Fields     []models.TicketField
	TypeStates []models.TicketTypeState
}

/*
loadSetup reads the workspace's configuration, adding the defaults to a
workspace that has none.
*/
func loadSetup(ctx context.Context, q *models.Queries, ws ids.UUID, now time.Time) (Setup, error) {
	sd := &seeder{q: q, ws: ws, now: now, states: map[string]ids.UUID{}}
	if err := sd.seed(ctx); err != nil {
		return Setup{}, err
	}

	var (
		s   Setup
		err error
	)

	if s.States, err = q.ListTicketStates(ctx, ws); err != nil {
		return s, fmt.Errorf("list ticket states: %w", err)
	}

	if s.Types, err = q.ListTicketTypes(ctx, ws); err != nil {
		return s, fmt.Errorf("list ticket types: %w", err)
	}

	if s.Fields, err = q.ListTicketFields(ctx, ws); err != nil {
		return s, fmt.Errorf("list ticket fields: %w", err)
	}

	if s.TypeStates, err = q.ListTypeStates(ctx, ws); err != nil {
		return s, fmt.Errorf("list ticket type states: %w", err)
	}

	return s, nil
}

/*
State returns the state with the id.
*/
func (s Setup) State(id ids.UUID) (models.TicketState, bool) {
	i := slices.IndexFunc(s.States, func(st models.TicketState) bool { return st.ID == id })
	if i < 0 {
		return models.TicketState{}, false
	}

	return s.States[i], true
}

/*
StateByPurpose returns the live state with the purpose.
*/
func (s Setup) StateByPurpose(role string) (models.TicketState, bool) {
	i := slices.IndexFunc(s.States, func(st models.TicketState) bool {
		return st.Purpose != nil && *st.Purpose == role && st.ArchivedAt == nil
	})
	if i < 0 {
		return models.TicketState{}, false
	}

	return s.States[i], true
}

/*
Type returns the type with the id.
*/
func (s Setup) Type(id ids.UUID) (models.TicketType, bool) {
	i := slices.IndexFunc(s.Types, func(t models.TicketType) bool { return t.ID == id })
	if i < 0 {
		return models.TicketType{}, false
	}

	return s.Types[i], true
}

/*
FirstType returns the first live type of the kind.
*/
func (s Setup) FirstType(kind string) (models.TicketType, bool) {
	i := slices.IndexFunc(s.Types, func(t models.TicketType) bool { return t.Kind == kind && t.ArchivedAt == nil })
	if i < 0 {
		return models.TicketType{}, false
	}

	return s.Types[i], true
}

/*
FieldsOf returns a type's fields in order, archived ones included.
*/
func (s Setup) FieldsOf(typeID ids.UUID) []models.TicketField {
	var out []models.TicketField
	for _, f := range s.Fields {
		if f.TypeID == typeID {
			out = append(out, f)
		}
	}

	return out
}

/*
Allowed returns the states a type may use, in the workspace's order.
A type that names none may use every live state.
*/
func (s Setup) Allowed(typeID ids.UUID) []models.TicketState {
	var named []ids.UUID
	for _, ts := range s.TypeStates {
		if ts.TypeID == typeID {
			named = append(named, ts.StateID)
		}
	}

	var out []models.TicketState
	for _, st := range s.States {
		if st.ArchivedAt == nil && (len(named) == 0 || slices.Contains(named, st.ID)) {
			out = append(out, st)
		}
	}

	return out
}

/*
Allows reports whether the type may use the state.
*/
func (s Setup) Allows(typeID, stateID ids.UUID) bool {
	return slices.ContainsFunc(s.Allowed(typeID), func(st models.TicketState) bool { return st.ID == stateID })
}

/*
DefaultState is where a new ticket of the type starts: the state it
marks as default, or its first submitted state, or its first state.
*/
func (s Setup) DefaultState(typeID ids.UUID) (models.TicketState, bool) {
	return s.defaultOf(typeID, GroupSubmitted, func(ts models.TicketTypeState) bool { return ts.IsDefault })
}

/*
DefaultResolved is where closing the conversation moves a ticket of the
type: the resolved state it marks, or its first resolved state.
*/
func (s Setup) DefaultResolved(typeID ids.UUID) (models.TicketState, bool) {
	return s.defaultOf(typeID, GroupResolved, func(ts models.TicketTypeState) bool { return ts.IsDefaultResolved })
}

/*
FirstOf returns the type's first allowed state of the group.
*/
func (s Setup) FirstOf(typeID ids.UUID, group string) (models.TicketState, bool) {
	for _, st := range s.Allowed(typeID) {
		if st.StateGroup == group {
			return st, true
		}
	}

	return models.TicketState{}, false
}

/*
defaultOf finds the marked state of a type, else its first of the group,
else its first state.
*/
func (s Setup) defaultOf(
	typeID ids.UUID, group string, marked func(models.TicketTypeState) bool,
) (models.TicketState, bool) {
	allowed := s.Allowed(typeID)
	for _, ts := range s.TypeStates {
		if ts.TypeID != typeID || !marked(ts) {
			continue
		}

		if st, ok := s.State(ts.StateID); ok && st.ArchivedAt == nil && st.StateGroup == group {
			return st, true
		}
	}

	if st, ok := s.FirstOf(typeID, group); ok {
		return st, true
	}

	if len(allowed) == 0 {
		return models.TicketState{}, false
	}

	return allowed[0], true
}

/*
defaultState is one of the states a workspace starts with.
*/
type defaultState struct {
	group, name, label, role string
	notify                   bool
}

/*
defaultStates lists the states a workspace starts with, in order.
*/
func defaultStates() []defaultState {
	return []defaultState{
		{group: GroupSubmitted, name: "New", label: "Submitted", notify: true},
		{group: GroupInProgress, name: "Investigating", label: "We're on it", role: PurposeInvestigating, notify: true},
		{group: GroupInProgress, name: "Close the loop", label: "We're on it", role: PurposeCloseLoop},
		{
			group: GroupInProgress, name: "Identified", label: "We found the cause and are fixing it",
			role:   PurposeIdentified,
			notify: true,
		},
		{group: GroupWaitingCustomer, name: "Needs info", label: "Waiting for your reply", notify: true},
		{group: GroupWaitingTeam, name: "With another team", label: "With our specialist team", notify: true},
		{
			group: GroupWaitingTeam, name: "Being worked on", label: "Being worked on", role: PurposeBeingWorked,
			notify: true,
		},
		{
			group: GroupWaitingTeam, name: "Monitoring", label: "A fix is out and we're watching it",
			role:   PurposeMonitoring,
			notify: true,
		},
		{group: GroupResolved, name: "Resolved", label: "Resolved", notify: true},
		{group: GroupResolved, name: "Won't fix", label: "Closed", role: PurposeWontFix, notify: true},
	}
}

/*
seeder adds the defaults to one workspace.
*/
type seeder struct {
	q      *models.Queries
	ws     ids.UUID
	now    time.Time
	states map[string]ids.UUID
}

/*
seed adds the default states and types to a workspace that has no type
yet.
*/
func (sd *seeder) seed(ctx context.Context) error {
	count, err := sd.q.CountTicketTypes(ctx, sd.ws)
	if err != nil {
		return fmt.Errorf("count ticket types: %w", err)
	}

	if count > 0 {
		return nil
	}

	for _, d := range defaultStates() {
		row, err := sd.q.CreateTicketState(ctx, models.CreateTicketStateParams{
			WorkspaceID: sd.ws, StateGroup: d.group, Name: d.name, CustomerLabel: d.label, NotifyCustomer: d.notify,
			Purpose: nonEmpty(d.role), Now: sd.now,
		})
		if err != nil {
			return fmt.Errorf("seed ticket states: %w", err)
		}

		sd.states[d.name] = row.ID
	}

	return sd.types(ctx)
}

/*
defaultType is one of the types a workspace starts with and the states
it uses, the first being its default.
*/
type defaultType struct {
	kind, name, icon, description string
	customer                      bool
	states                        []string
	fields                        []defaultField
}

/*
defaultField is one field of a default type.
*/
type defaultField struct {
	key, label, dataType string
	options              []string
	requiredResolve      bool
	customer             bool
	hint                 string
}

/*
defaultTypes lists the types a workspace starts with.
*/
func defaultTypes() []defaultType {
	everyday := []string{
		"New", "Investigating", "Close the loop", "Needs info", "With another team",
		"Being worked on", "Resolved", "Won't fix",
	}

	return []defaultType{
		{
			kind: KindCustomer, name: "Question", icon: "question", customer: true, states: everyday,
			description: "A request the team needs time to answer.",
		},
		{
			kind: KindCustomer, name: "Bug report", icon: "bug", customer: true, states: everyday,
			description: "Something in the product is not working.",
			fields: []defaultField{
				{
					key: "steps", label: "Steps to reproduce", dataType: "long_text", customer: true,
					hint: "How the customer got to the problem, step by step.",
				},
				{
					key: "severity", label: "Severity", dataType: "single_choice",
					options: []string{"low", "medium", "high"},
					hint:    "high when the customer cannot work at all, low for cosmetic problems.",
				},
			},
		},
		{
			kind: KindBackOffice, name: "Back-office task", icon: "briefcase", states: everyday,
			description: "Work another team does for a customer request.",
		},
		{
			kind: KindTracker, name: "Tracker", icon: "stack", states: everyday,
			description: "One problem many customers report.",
		},
		{
			kind: KindIncident, name: "Incident", icon: "alarm-warning",
			states:      []string{"Investigating", "Identified", "Monitoring", "Resolved"},
			description: "An outage customers are affected by.",
			fields: []defaultField{
				{
					key: "severity", label: "Severity", dataType: "single_choice",
					options: []string{"minor", "major", "critical"}, customer: true,
				},
				{
					key: "areas", label: "Affected areas", dataType: "multiple_choice",
					options: []string{"Sign-in", "Payments", "API", "Messenger", "Email"}, customer: true,
				},
			},
		},
	}
}

/*
types adds the default types with their states and fields.
*/
func (sd *seeder) types(ctx context.Context) error {
	for _, d := range defaultTypes() {
		row, err := sd.q.CreateTicketType(ctx, models.CreateTicketTypeParams{
			WorkspaceID: sd.ws, Kind: d.kind, Name: d.name, Icon: d.icon, Description: d.description,
			CustomerVisible: d.customer || d.kind == KindIncident, ReopenDays: defaultReopenDays,
			Now: sd.now,
		})
		if err != nil {
			return fmt.Errorf("seed ticket types: %w", err)
		}

		if err := sd.typeStates(ctx, row, d.states); err != nil {
			return err
		}

		if err := sd.fields(ctx, row, d.fields); err != nil {
			return err
		}
	}

	return nil
}

/*
typeStates lets a default type use its states.
*/
func (sd *seeder) typeStates(ctx context.Context, t models.TicketType, names []string) error {
	for i, name := range names {
		err := sd.q.AddTypeState(ctx, models.AddTypeStateParams{
			TypeID: t.ID, StateID: sd.states[name], WorkspaceID: t.WorkspaceID, IsDefault: i == 0,
			IsDefaultResolved: name == "Resolved",
		})
		if err != nil {
			return fmt.Errorf("seed ticket type states: %w", err)
		}
	}

	return nil
}

/*
fields adds a default type's fields.
*/
func (sd *seeder) fields(ctx context.Context, t models.TicketType, fields []defaultField) error {
	for _, f := range fields {
		_, err := sd.q.CreateTicketField(ctx, models.CreateTicketFieldParams{
			WorkspaceID: t.WorkspaceID, TypeID: t.ID, Key: f.key, Label: f.label, DataType: f.dataType,
			Options: nonNil(f.options), RequiredResolve: f.requiredResolve, VisibleCustomer: f.customer,
			EditableCustomer: f.customer, AiHint: nonEmpty(f.hint), Now: sd.now,
		})
		if err != nil {
			return fmt.Errorf("seed ticket fields: %w", err)
		}
	}

	return nil
}

/*
nonEmpty returns a pointer to the text, nil when it is empty.
*/
func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

/*
nonNil returns the list, empty rather than nil.
*/
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}

/*
rawObject encodes a map as a JSON object.
*/
func rawObject(m map[string]any) json.RawMessage {
	if m == nil {
		return json.RawMessage(`{}`)
	}

	b, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}

	return b
}

/*
objectOf reads a JSON object, empty when it is not one.
*/
func objectOf(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}

	if out == nil {
		out = map[string]any{}
	}

	return out
}
