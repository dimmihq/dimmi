/*
|--------------------------------------------------------------------------
| State Settings
|--------------------------------------------------------------------------
|
| A workspace's states: added to a group, renamed or relabelled, put in
| order, and removed with their tickets moved to a state of the same
| group, which the types that used it now use.
|
*/

package tickets

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
StateInput is a state's settings; nil leaves a field.
*/
type StateInput struct {
	Group          *string
	Name           *string
	CustomerLabel  *string
	NotifyCustomer *bool
}

/*
CreateState adds a state at the end of its group.
*/
func (t *Tickets) CreateState(ctx context.Context, m authz.Membership, in StateInput) (models.TicketState, error) {
	var out models.TicketState

	err := t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		if in.Group == nil || !slices.Contains(Groups(), *in.Group) {
			return ErrGroupInvalid
		}

		name, label, err := stateText(in, "", "")
		if err != nil {
			return err
		}

		out, err = w.Q.CreateTicketState(ctx, models.CreateTicketStateParams{
			WorkspaceID: m.Workspace.ID, StateGroup: *in.Group, Name: name, CustomerLabel: label,
			NotifyCustomer: in.NotifyCustomer == nil || *in.NotifyCustomer, Now: w.Now,
		})
		if err != nil {
			return fmt.Errorf("create ticket state: %w", err)
		}

		return nil
	})

	return out, err
}

/*
stateText checks a state's name and customer label, keeping the current
ones where the input leaves them.
*/
func stateText(in StateInput, name, label string) (string, string, error) {
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}

	if in.CustomerLabel != nil {
		label = strings.TrimSpace(*in.CustomerLabel)
	}

	if label == "" {
		label = name
	}

	if n := len([]rune(name)); n < 1 || n > 60 {
		return "", "", ErrNameInvalid
	}

	if n := len([]rune(label)); n > 80 {
		return "", "", ErrNameInvalid.OnField("customer_label")
	}

	return name, label, nil
}

/*
UpdateState renames a state, changes its label or notice; its group
stays.
*/
func (t *Tickets) UpdateState(
	ctx context.Context, m authz.Membership, id ids.UUID, in StateInput,
) (models.TicketState, error) {
	var out models.TicketState

	err := t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		cur, ok := w.Setup.State(id)
		if !ok || cur.ArchivedAt != nil {
			return ErrStateNotFound
		}

		name, label, err := stateText(in, cur.Name, cur.CustomerLabel)
		if err != nil {
			return err
		}

		notify := cur.NotifyCustomer
		if in.NotifyCustomer != nil {
			notify = *in.NotifyCustomer
		}

		out, err = w.Q.UpdateTicketState(ctx, models.UpdateTicketStateParams{
			Name: name, CustomerLabel: label, NotifyCustomer: notify, Now: w.Now, WorkspaceID: m.Workspace.ID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("update ticket state: %w", err)
		}

		return nil
	})

	return out, err
}

/*
ReorderStates saves the order of the live states; each keeps its group.
*/
func (t *Tickets) ReorderStates(
	ctx context.Context, m authz.Membership, order []ids.UUID,
) ([]models.TicketState, error) {
	var out []models.TicketState

	err := t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		var live []ids.UUID
		for _, s := range w.Setup.States {
			if s.ArchivedAt == nil {
				live = append(live, s.ID)
			}
		}

		if !sameSet(live, order) {
			return ErrOrderInvalid
		}

		if err := w.Q.SaveTicketStateOrder(ctx, models.SaveTicketStateOrderParams{
			Now: w.Now, Ids: order, WorkspaceID: m.Workspace.ID,
		}); err != nil {
			return fmt.Errorf("order ticket states: %w", err)
		}

		var err error
		out, err = w.Q.ListTicketStates(ctx, m.Workspace.ID)

		return err
	})

	return out, err
}

/*
DeleteState archives a state, moving its tickets to another live state
of its group, which the types that used it now use.
*/
func (t *Tickets) DeleteState(ctx context.Context, m authz.Membership, id ids.UUID, moveTo *ids.UUID) error {
	return t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		cur, ok := w.Setup.State(id)
		if !ok || cur.ArchivedAt != nil {
			return ErrStateNotFound
		}

		if moveTo == nil || *moveTo == id {
			return ErrMoveToRequired
		}

		target, ok := w.Setup.State(*moveTo)
		if !ok || target.ArchivedAt != nil || target.StateGroup != cur.StateGroup {
			return ErrMoveToRequired
		}

		return w.removeState(ctx, cur, target)
	})
}

/*
removeState moves the state's tickets and types to the target and
archives it.
*/
func (w *Writer) removeState(ctx context.Context, cur, target models.TicketState) error {
	ws := cur.WorkspaceID

	if _, err := w.Q.MoveTicketsToState(ctx, models.MoveTicketsToStateParams{
		Target: target.ID, Now: w.Now, WorkspaceID: ws, Source: cur.ID,
	}); err != nil {
		return fmt.Errorf("move tickets off state: %w", err)
	}

	if err := w.Q.ReplaceTypeState(ctx, models.ReplaceTypeStateParams{
		Target: target.ID, WorkspaceID: ws, Source: cur.ID,
	}); err != nil {
		return fmt.Errorf("move types off state: %w", err)
	}

	if err := w.Q.DeleteTypeStatesOfState(ctx, models.DeleteTypeStatesOfStateParams{
		WorkspaceID: ws, StateID: cur.ID,
	}); err != nil {
		return fmt.Errorf("take state off types: %w", err)
	}

	if err := w.Q.ArchiveTicketState(ctx, models.ArchiveTicketStateParams{
		Now: &w.Now, WorkspaceID: ws, ID: cur.ID,
	}); err != nil {
		return fmt.Errorf("archive ticket state: %w", err)
	}

	return nil
}
