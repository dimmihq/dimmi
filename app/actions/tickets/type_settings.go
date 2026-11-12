/*
|--------------------------------------------------------------------------
| Type Settings
|--------------------------------------------------------------------------
|
| A workspace's types: added with a kind, changed with the states they
| may use and their defaults, and archived, which keeps them on old
| tickets.
|
*/

package tickets

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TypeInput is a type's settings; nil leaves a field. States lists the
states it may use, the default first in each; DefaultResolved names the
state closing the conversation moves it to.
*/
type TypeInput struct {
	Kind            *string
	Name            *string
	Icon            *string
	Description     *string
	InboxID         **ids.UUID
	TeamID          **ids.UUID
	CustomerVisible *bool
	CompanySharing  *bool
	ReopenDays      *int32
	AutoResolveDays **int32
	AutoResolveText **string
	AIAutoFill      *bool
	States          []ids.UUID
	DefaultState    *ids.UUID
	DefaultResolved *ids.UUID
}

/*
iconShape is what an icon name looks like.
*/
var iconShape = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

/*
CreateType adds a type after the others.
*/
func (t *Tickets) CreateType(ctx context.Context, m authz.Membership, in TypeInput) (models.TicketType, error) {
	var out models.TicketType

	err := t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		if in.Kind == nil || !slices.Contains(Kinds(), *in.Kind) {
			return ErrKindMismatch.OnField("kind")
		}

		cur := models.TicketType{
			Kind: *in.Kind, Icon: "ticket", ReopenDays: defaultReopenDays,
			CustomerVisible: *in.Kind == KindCustomer || *in.Kind == KindIncident,
		}

		next, err := applyType(cur, in)
		if err != nil {
			return err
		}

		out, err = w.Q.CreateTicketType(ctx, models.CreateTicketTypeParams{
			WorkspaceID: m.Workspace.ID, Kind: next.Kind, Name: next.Name, Icon: next.Icon,
			Description: next.Description, InboxID: next.InboxID, TeamID: next.TeamID,
			CustomerVisible: next.CustomerVisible, CompanySharing: next.CompanySharing, ReopenDays: next.ReopenDays,
			AutoResolveDays: next.AutoResolveDays, AutoResolveMessage: next.AutoResolveMessage,
			AiAutoFill: next.AiAutoFill,
			Now:        w.Now,
		})
		if err != nil {
			return fmt.Errorf("create ticket type: %w", err)
		}

		return w.saveTypeStates(ctx, out, in)
	})

	return out, err
}

/*
UpdateType changes a type's settings and the states it may use.
*/
func (t *Tickets) UpdateType(
	ctx context.Context, m authz.Membership, id ids.UUID, in TypeInput,
) (models.TicketType, error) {
	var out models.TicketType

	err := t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		cur, ok := w.Setup.Type(id)
		if !ok || cur.ArchivedAt != nil {
			return ErrTypeNotFound
		}

		in.Kind = nil

		next, err := applyType(cur, in)
		if err != nil {
			return err
		}

		out, err = w.Q.UpdateTicketType(ctx, models.UpdateTicketTypeParams{
			Name: next.Name, Icon: next.Icon, Description: next.Description, InboxID: next.InboxID,
			TeamID: next.TeamID, CustomerVisible: next.CustomerVisible, CompanySharing: next.CompanySharing,
			ReopenDays: next.ReopenDays, AutoResolveDays: next.AutoResolveDays,
			AutoResolveMessage: next.AutoResolveMessage, AiAutoFill: next.AiAutoFill, Now: w.Now,
			WorkspaceID: m.Workspace.ID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("update ticket type: %w", err)
		}

		return w.saveTypeStates(ctx, out, in)
	})

	return out, err
}

/*
applyType applies the input to a type's settings and checks them.
*/
func applyType(cur models.TicketType, in TypeInput) (models.TicketType, error) {
	setText(&cur.Name, in.Name)
	setText(&cur.Icon, in.Icon)
	setText(&cur.Description, in.Description)
	setOpt(&cur.InboxID, in.InboxID)
	setOpt(&cur.TeamID, in.TeamID)
	setValue(&cur.CustomerVisible, in.CustomerVisible)
	setValue(&cur.CompanySharing, in.CompanySharing)
	setValue(&cur.ReopenDays, in.ReopenDays)
	setOpt(&cur.AutoResolveDays, in.AutoResolveDays)
	setOpt(&cur.AutoResolveMessage, in.AutoResolveText)
	setValue(&cur.AiAutoFill, in.AIAutoFill)

	return cur, checkType(cur)
}

/*
checkType refuses settings a type may not have.
*/
func checkType(t models.TicketType) error {
	switch {
	case len([]rune(t.Name)) < 1 || len([]rune(t.Name)) > 60:
		return ErrNameInvalid
	case !iconShape.MatchString(t.Icon):
		return ErrNameInvalid.OnField("icon")
	case len([]rune(t.Description)) > 500:
		return ErrNameInvalid.OnField("description")
	case t.ReopenDays < 0 || t.ReopenDays > 90:
		return ErrNameInvalid.OnField("reopen_days")
	case t.AutoResolveDays != nil && (*t.AutoResolveDays < 1 || *t.AutoResolveDays > 90):
		return ErrNameInvalid.OnField("auto_resolve_days")
	}

	return nil
}

/*
saveTypeStates stores the states a type may use, when the input names
them.
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

	if err := w.Q.DeleteTypeStates(ctx, models.DeleteTypeStatesParams{
		WorkspaceID: ty.WorkspaceID, TypeID: ty.ID,
	}); err != nil {
		return fmt.Errorf("clear type states: %w", err)
	}

	for _, id := range in.States {
		err := w.Q.AddTypeState(ctx, models.AddTypeStateParams{
			TypeID: ty.ID, StateID: id, WorkspaceID: ty.WorkspaceID,
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
ArchiveType hides a type from new tickets.
*/
func (t *Tickets) ArchiveType(ctx context.Context, m authz.Membership, id ids.UUID) error {
	return t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		if _, ok := w.Setup.Type(id); !ok {
			return ErrTypeNotFound
		}

		if err := w.Q.ArchiveTicketType(ctx, models.ArchiveTicketTypeParams{
			Now: &w.Now, WorkspaceID: m.Workspace.ID, ID: id,
		}); err != nil {
			return fmt.Errorf("archive ticket type: %w", err)
		}

		return nil
	})
}
