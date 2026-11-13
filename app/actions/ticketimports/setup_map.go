/*
|--------------------------------------------------------------------------
| Mapping The Setup
|--------------------------------------------------------------------------
|
| States, types and fields come in before any ticket. Each is matched
| to what the workspace already has, a state by name within its group,
| a type by name within its kind, a field by key within its type, so a
| second run or a workspace set up by hand reuses them. Anything new is
| added and kept on the import, so an undo can archive it. A type uses
| every state the other tool let it use; a new type starts in its first
| submitted state and resolves to its first resolved one. Imported
| fields are never required, so history replays whatever it held.
|
*/

package ticketimports

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
reopenDays is the reopen window of a type an import adds.
*/
const reopenDays = 7

/*
mapping is the other tool's setup in the workspace's terms.
*/
type mapping struct {
	types  map[string]models.TicketType
	states map[string]ids.UUID
}

/*
setupStep is the setup being mapped in one transaction.
*/
type setupStep struct {
	run   *run
	q     *models.Queries
	setup tickets.Setup
	added counts
}

/*
mapSetup maps the catalog in one transaction.
*/
func (r *run) mapSetup(ctx context.Context, catalog Catalog) error {
	r.mapped = mapping{types: map[string]models.TicketType{}, states: map[string]ids.UUID{}}

	var added counts

	err := r.importer.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := r.writer(ctx, tx, r.importer.Clock.Now())
		if err != nil {
			return err
		}

		s := &setupStep{run: r, q: w.Q, setup: w.Setup}
		if err := s.states(ctx, catalog.States); err != nil {
			return err
		}

		for _, t := range catalog.Types {
			if err := s.kind(ctx, t, catalog.States); err != nil {
				return err
			}
		}

		added = s.added

		return nil
	})
	if err != nil {
		return err
	}

	return r.progress(ctx, added)
}

/*
states maps each state, adding the ones the workspace lacks.
*/
func (s *setupStep) states(ctx context.Context, states []State) error {
	for _, st := range states {
		name := cut(strings.TrimSpace(st.Name), 60)
		if name == "" {
			name = st.Group
		}

		i := slices.IndexFunc(s.setup.States, func(have models.TicketState) bool {
			return have.ArchivedAt == nil && have.StateGroup == st.Group && strings.EqualFold(have.Name, name)
		})
		if i >= 0 {
			s.run.mapped.states[st.ExternalID] = s.setup.States[i].ID
			if err := s.run.record(ctx, s.q, touched{
				kind: "state", external: st.ExternalID,
				id: s.setup.States[i].ID,
			}); err != nil {
				return err
			}

			continue
		}

		if err := s.addState(ctx, st, name); err != nil {
			return err
		}
	}

	return nil
}

/*
addState adds a state the workspace lacks.
*/
func (s *setupStep) addState(ctx context.Context, st State, name string) error {
	label := cut(strings.TrimSpace(st.CustomerLabel), 80)
	if label == "" {
		label = name
	}

	row, err := s.q.CreateTicketState(ctx, models.CreateTicketStateParams{
		WorkspaceID: s.run.row.WorkspaceID, StateGroup: st.Group, Name: name, CustomerLabel: label,
		NotifyCustomer: true, Now: s.run.importer.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("add imported state: %w", err)
	}

	s.run.mapped.states[st.ExternalID] = row.ID
	s.setup.States = append(s.setup.States, row)
	s.added.states++

	return s.run.record(ctx, s.q, touched{kind: "state", external: st.ExternalID, id: row.ID, created: true})
}

/*
kind maps a type with its states and fields, adding it when the
workspace lacks it.
*/
func (s *setupStep) kind(ctx context.Context, t Type, states []State) error {
	name := cut(strings.TrimSpace(t.Name), 60)

	i := slices.IndexFunc(s.setup.Types, func(have models.TicketType) bool {
		return have.ArchivedAt == nil && have.Kind == t.Kind && strings.EqualFold(have.Name, name)
	})

	var (
		ty      models.TicketType
		created bool
	)

	if i >= 0 {
		ty = s.setup.Types[i]
	} else {
		var err error
		if ty, err = s.addType(ctx, t, name); err != nil {
			return err
		}

		created = true
	}

	s.run.mapped.types[t.ExternalID] = ty

	if err := s.run.record(ctx, s.q, touched{
		kind: "type", external: t.ExternalID, id: ty.ID,
		created: created,
	}); err != nil {
		return err
	}

	if err := s.allow(ctx, ty, ordered(t.States, states), created); err != nil {
		return err
	}

	return s.fields(ctx, ty, t.Fields)
}

/*
addType adds a type the workspace lacks.
*/
func (s *setupStep) addType(ctx context.Context, t Type, name string) (models.TicketType, error) {
	if name == "" {
		name = "Imported tickets"
	}

	row, err := s.q.CreateTicketType(ctx, models.CreateTicketTypeParams{
		WorkspaceID: s.run.row.WorkspaceID, Kind: t.Kind, Name: name, Icon: "ticket",
		Description: cut(t.Description, 500), CustomerVisible: t.Kind == tickets.KindCustomer,
		ReopenDays: reopenDays, Now: s.run.importer.Clock.Now(),
	})
	if err != nil {
		return row, fmt.Errorf("add imported type: %w", err)
	}

	s.added.types++

	return row, nil
}

/*
ordered returns the external ids of the type's states in group order,
so the first is a submitted one.
*/
func ordered(wanted []string, states []State) []State {
	var out []State

	for _, group := range tickets.Groups() {
		for _, st := range states {
			if st.Group == group && slices.Contains(wanted, st.ExternalID) {
				out = append(out, st)
			}
		}
	}

	return out
}

/*
allow lets the type use the states it does not yet, a new type's first
state its default and first resolved state its resolution.
*/
func (s *setupStep) allow(ctx context.Context, ty models.TicketType, states []State, created bool) error {
	first, resolved := created, false

	for _, st := range states {
		id, ok := s.run.mapped.states[st.ExternalID]
		if !ok || s.setup.Allows(ty.ID, id) {
			continue
		}

		last := created && st.Group == tickets.GroupResolved && !resolved
		resolved = resolved || last

		err := s.q.AddTypeState(ctx, models.AddTypeStateParams{
			TypeID: ty.ID, StateID: id, WorkspaceID: ty.WorkspaceID, IsDefault: first,
			IsDefaultResolved: last, Position: s.nextPosition(ty.ID),
		})
		first = false
		if err != nil {
			return fmt.Errorf("allow imported state: %w", err)
		}

		s.setup.TypeStates = append(s.setup.TypeStates, models.TicketTypeState{
			TypeID: ty.ID, StateID: id, WorkspaceID: ty.WorkspaceID, Position: s.nextPosition(ty.ID),
		})
	}

	return nil
}

/*
fields adds the type's fields it lacks.
*/
func (s *setupStep) fields(ctx context.Context, ty models.TicketType, fields []Field) error {
	have := s.setup.FieldsOf(ty.ID)

	for _, f := range fields {
		if slices.ContainsFunc(have, func(h models.TicketField) bool { return h.Key == f.Key }) {
			continue
		}

		row, err := s.q.CreateTicketField(ctx, models.CreateTicketFieldParams{
			WorkspaceID: ty.WorkspaceID, TypeID: ty.ID, Key: f.Key, Label: cut(f.Label, 80), DataType: f.DataType,
			Options: options(f.Options), VisibleCustomer: f.Customer, Now: s.run.importer.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			continue
		}

		if err != nil {
			return fmt.Errorf("add imported field: %w", err)
		}

		have = append(have, row)
		s.added.fields++

		err = s.run.record(ctx, s.q, touched{
			kind: "field", external: ty.ID.String() + ":" + f.ExternalID,
			id: row.ID, created: true,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
options returns at most 200 distinct options, never nil.
*/
func options(in []string) []string {
	out := []string{}

	for _, o := range in {
		if o = strings.TrimSpace(o); o != "" && !slices.Contains(out, o) && len(out) < 200 {
			out = append(out, o)
		}
	}

	return out
}

/*
nextPosition is the place after the type's last state.
*/
func (s *setupStep) nextPosition(typeID ids.UUID) int32 {
	var n int32

	for _, ts := range s.setup.TypeStates {
		if ts.TypeID == typeID && ts.Position >= n {
			n = ts.Position + 1
		}
	}

	return n
}
