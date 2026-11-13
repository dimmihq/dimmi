/*
|--------------------------------------------------------------------------
| A Type's States
|--------------------------------------------------------------------------
|
| A type lists the states its tickets move through, in its own order,
| so reordering one type's steps leaves the others alone. Taking a state
| off a type that still holds some of its tickets needs a place for
| them: one of the states the type keeps, which they move to in the same
| step, resolved or reopened as that state's group says.
|
*/

package tickets

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
saveTypeStates stores the states a type may use, in order, when the
input names them, after moving the tickets held in states it drops.
*/
func (w *Writer) saveTypeStates(ctx context.Context, ty models.TicketType, in TypeInput) error {
	if in.States == nil {
		return nil
	}

	for _, id := range in.States {
		if st, ok := w.Setup.State(id); !ok || st.ArchivedAt != nil {
			return ErrStateNotFound.OnField("states")
		}
	}

	if err := w.moveDropped(ctx, ty, in); err != nil {
		return err
	}

	if err := w.Q.DeleteTypeStates(ctx, models.DeleteTypeStatesParams{
		WorkspaceID: ty.WorkspaceID, TypeID: ty.ID,
	}); err != nil {
		return fmt.Errorf("clear type states: %w", err)
	}

	for i, id := range in.States {
		err := w.Q.AddTypeState(ctx, models.AddTypeStateParams{
			TypeID: ty.ID, StateID: id, WorkspaceID: ty.WorkspaceID, Position: int32(i),
			IsDefault:         in.DefaultState != nil && *in.DefaultState == id,
			IsDefaultResolved: in.DefaultResolved != nil && *in.DefaultResolved == id,
		})
		if err != nil {
			return fmt.Errorf("save type states: %w", err)
		}
	}

	return nil
}

/*
moveDropped moves the type's tickets out of each state it no longer
uses, to the state the input names for it. A dropped state that holds
tickets and has no place named is refused.
*/
func (w *Writer) moveDropped(ctx context.Context, ty models.TicketType, in TypeInput) error {
	held, err := w.heldByState(ctx, ty)
	if err != nil {
		return err
	}

	for _, st := range w.Setup.Allowed(ty.ID) {
		if slices.Contains(in.States, st.ID) || held[st.ID] == 0 {
			continue
		}

		target, ok := w.Setup.State(in.MoveTickets[st.ID])
		if !ok || !slices.Contains(in.States, target.ID) {
			return ErrTicketsNeedState.With("state_id", models.PublicID(models.PrefixTicketState, st.ID))
		}

		if _, err := w.Q.MoveTypeTicketsToState(ctx, models.MoveTypeTicketsToStateParams{
			Target: target.ID, TargetGroup: target.StateGroup, Now: w.Now,
			WorkspaceID: ty.WorkspaceID, TypeID: ty.ID, Source: st.ID,
		}); err != nil {
			return fmt.Errorf("move tickets off state: %w", err)
		}
	}

	return nil
}

/*
heldByState counts the type's tickets, trashed ones included, in each
state.
*/
func (w *Writer) heldByState(ctx context.Context, ty models.TicketType) (map[ids.UUID]int64, error) {
	rows, err := w.Q.CountTicketsByType(ctx, ty.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("count tickets by state: %w", err)
	}

	out := map[ids.UUID]int64{}

	for _, r := range rows {
		if r.TypeID == ty.ID {
			out[r.StateID] = r.Held
		}
	}

	return out, nil
}
