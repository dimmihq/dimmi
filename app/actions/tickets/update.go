/*
|--------------------------------------------------------------------------
| Changing Tickets
|--------------------------------------------------------------------------
|
| A change names only what it changes. Given the version it was read
| at, it is refused when the ticket moved on since, so two people never
| overwrite each other. Field values merge into those stored; a null
| removes one. Moving to another type keeps the values whose keys the
| new type also has and logs the rest. Accepting AI suggestions turns
| them into values. The state moves last, so a change that fills the
| fields a resolve needs and resolves goes through in one step.
|
*/

package tickets

import (
	"context"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Change is a change to a ticket; nil leaves a field. Version, when set,
must be the ticket's current one. ClearAssignee and ClearTeam unassign.
Accept lists suggested field keys to keep as values.
*/
type Change struct {
	Version       *int32
	StateID       *ids.UUID
	TypeID        *ids.UUID
	Title         *string
	Description   *string
	Fields        map[string]any
	Priority      *string
	Assignee      *ids.UUID
	ClearAssignee bool
	Team          *ids.UUID
	ClearTeam     bool
	SLAPolicyID   *ids.UUID
	Accept        []string
}

/*
Apply makes the change to the locked ticket.
*/
func (w *Writer) Apply(ctx context.Context, t models.Ticket, c Change) (models.Ticket, error) {
	if c.Version != nil && *c.Version != t.Version {
		return t, ErrVersionClash.With("version", t.Version)
	}

	t, err := w.retype(ctx, t, c.TypeID)
	if err != nil {
		return t, err
	}

	if t, err = w.edit(ctx, t, c); err != nil {
		return t, err
	}

	if t, err = w.assign(ctx, t, c); err != nil {
		return t, err
	}

	if c.StateID == nil {
		return t, nil
	}

	to, ok := w.Setup.State(*c.StateID)
	if !ok || to.ArchivedAt != nil {
		return t, ErrStateNotFound.OnField("state_id")
	}

	moved, err := w.move(ctx, t, to)

	return moved.ticket, err
}

/*
retype moves the ticket to another type of its kind, keeping the
values the new type has fields for and a state it allows.
*/
func (w *Writer) retype(ctx context.Context, t models.Ticket, typeID *ids.UUID) (models.Ticket, error) {
	if typeID == nil || *typeID == t.TypeID {
		return t, nil
	}

	ty, ok := w.Setup.Type(*typeID)
	if !ok {
		return t, ErrTypeNotFound.OnField("type_id")
	}

	if ty.Kind != t.Kind {
		return t, ErrKindMismatch
	}

	kept, dropped := keepFields(w.Setup.FieldsOf(ty.ID), objectOf(t.Fields))
	from := t.TypeID
	t.TypeID, t.Fields = ty.ID, rawObject(kept)

	if !w.Setup.Allows(ty.ID, t.StateID) {
		if st, ok := w.Setup.FirstOf(ty.ID, t.StateGroup); ok {
			t.StateID = st.ID
		} else if st, ok := w.Setup.DefaultState(ty.ID); ok {
			t.StateID, t.StateGroup = st.ID, st.StateGroup
		}
	}

	saved, err := w.save(ctx, t)
	if err != nil {
		return t, err
	}

	if err := w.log(ctx, saved, "type", map[string]any{
		"from": from.String(), "to": ty.ID.String(), "to_name": ty.Name, "dropped_fields": dropped,
	}); err != nil {
		return saved, err
	}

	changes := map[string]appevents.TicketChange{
		"type": change(from, ty.ID), "dropped_fields": change(dropped, nil),
	}

	return saved, w.emit(ctx, appevents.TicketTypeChanged{TicketChanged: w.changed(saved, changes)})
}

/*
keepFields splits values into those the fields have keys for and the
keys dropped.
*/
func keepFields(fields []models.TicketField, values map[string]any) (map[string]any, []string) {
	kept, dropped := map[string]any{}, []string{}

	for k, v := range values {
		if _, ok := fieldByKey(fields, k); ok {
			kept[k] = v
		} else {
			dropped = append(dropped, k)
		}
	}

	slices.Sort(dropped)

	return kept, dropped
}

/*
edit changes the title, description, values, priority and policy.
*/
func (w *Writer) edit(ctx context.Context, t models.Ticket, c Change) (models.Ticket, error) {
	before := t
	changes := map[string]appevents.TicketChange{}

	if err := editText(&t, c, changes); err != nil {
		return t, err
	}

	if err := w.editFields(&t, c, changes); err != nil {
		return t, err
	}

	editSettings(&t, c, changes)

	if len(changes) == 0 && slices.Equal(before.Suggestions, t.Suggestions) {
		return t, nil
	}

	saved, err := w.save(ctx, t)
	if err != nil || len(changes) == 0 {
		return saved, err
	}

	if err := w.log(ctx, saved, "update", changeData(changes)); err != nil {
		return saved, err
	}

	return saved, w.emit(ctx, appevents.TicketUpdated{TicketChanged: w.changed(saved, changes)})
}

/*
editText changes the title and description.
*/
func editText(t *models.Ticket, c Change, changes map[string]appevents.TicketChange) error {
	if c.Title != nil {
		title := strings.TrimSpace(*c.Title)
		if n := len([]rune(title)); n < 1 || n > 200 {
			return ErrTitleInvalid
		}

		if title != t.Title {
			changes["title"] = change(t.Title, title)
			t.Title = title
		}
	}

	if c.Description != nil && *c.Description != t.Description {
		changes["description"] = change(nil, nil)
		t.Description = *c.Description
	}

	return nil
}

/*
editFields merges values and accepted suggestions, checking each.
*/
func (w *Writer) editFields(t *models.Ticket, c Change, changes map[string]appevents.TicketChange) error {
	stored := objectOf(t.Fields)
	suggested := objectOf(t.Suggestions)
	asked := map[string]any{}

	for _, key := range c.Accept {
		if v, ok := suggested[key]; ok {
			asked[key] = v
			delete(suggested, key)
		}
	}

	for k, v := range c.Fields {
		asked[k] = v
		delete(suggested, k)
	}

	merged, problems := merge(w.Setup.FieldsOf(t.TypeID), stored, asked)
	if len(problems) > 0 {
		return invalidError(problems)
	}

	for k := range asked {
		if !sameValue(stored[k], merged[k]) {
			changes["fields."+k] = change(stored[k], merged[k])
		}
	}

	t.Fields, t.Suggestions = rawObject(merged), rawObject(suggested)

	return nil
}

/*
sameValue compares two stored values.
*/
func sameValue(a, b any) bool {
	return string(rawObject(map[string]any{"v": a})) == string(rawObject(map[string]any{"v": b}))
}

/*
editSettings changes the priority and service level policy.
*/
func editSettings(t *models.Ticket, c Change, changes map[string]appevents.TicketChange) {
	if c.Priority != nil && *c.Priority != t.Priority && slices.Contains(Priorities(), *c.Priority) {
		changes["priority"] = change(t.Priority, *c.Priority)
		t.Priority = *c.Priority
	}

	if c.SLAPolicyID != nil && (t.SlaPolicyID == nil || *t.SlaPolicyID != *c.SLAPolicyID) {
		changes["sla_policy_id"] = change(t.SlaPolicyID, *c.SLAPolicyID)
		t.SlaPolicyID = c.SLAPolicyID
	}
}

/*
changeData shapes changes for the log.
*/
func changeData(changes map[string]appevents.TicketChange) map[string]any {
	out := map[string]any{}
	for k, v := range changes {
		out[k] = map[string]any{"from": v.From, "to": v.To}
	}

	return map[string]any{"changes": out}
}

/*
assign changes the teammate and team.
*/
func (w *Writer) assign(ctx context.Context, t models.Ticket, c Change) (models.Ticket, error) {
	member, team := t.AssigneeMemberID, t.AssigneeTeamID

	switch {
	case c.ClearAssignee:
		member = nil
	case c.Assignee != nil:
		member = c.Assignee
	}

	switch {
	case c.ClearTeam:
		team = nil
	case c.Team != nil:
		team = c.Team
	}

	if samePtr(member, t.AssigneeMemberID) && samePtr(team, t.AssigneeTeamID) {
		return t, nil
	}

	changes := map[string]appevents.TicketChange{
		"assignee_member_id": change(t.AssigneeMemberID, member), "assignee_team_id": change(t.AssigneeTeamID, team),
	}
	t.AssigneeMemberID, t.AssigneeTeamID = member, team

	saved, err := w.save(ctx, t)
	if err != nil {
		return saved, err
	}

	if err := w.log(ctx, saved, "assign", changeData(changes)); err != nil {
		return saved, err
	}

	return saved, w.emit(ctx, appevents.TicketAssigned{TicketChanged: w.changed(saved, changes)})
}

/*
samePtr compares two optional ids.
*/
func samePtr(a, b *ids.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
