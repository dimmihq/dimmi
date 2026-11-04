/*
|--------------------------------------------------------------------------
| Changing Views
|--------------------------------------------------------------------------
|
| A view's owner changes their personal view; shared views, their order
| and their deletion take views:manage_shared. Deleting moves a view to
| the trash, so a mistake can be undone by hand, and frees its name.
|
*/

package conversationviews

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateInput is who changes which view, and how.
*/
type UpdateInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Fields     Fields
}

/*
UpdateView changes a view.

Rules:
  - The owner changes a personal view; views:manage_shared a shared
    one. Another member's personal view is not found.
  - The same checks as creating apply to what changes.

Events: [appevents.ConversationViewChanged].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInvalidDefinition], [exceptions.ErrViewNameTaken], or a
wrapped database error.
*/
type UpdateView struct {
	Saver
}

/*
Handle changes the view and returns it as it now is.
*/
func (a *UpdateView) Handle(ctx context.Context, in UpdateInput) (View, error) {
	name, err := a.validate(in.Membership, in.Fields)
	if err != nil {
		return View{}, err
	}

	var out View

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		v, err := conversations.FindView(ctx, q, in.Membership, in.ID)
		if err != nil {
			return err
		}

		if err := canChange(in.Membership, v.Scope, v.OwnerMemberID); err != nil {
			return err
		}

		row, err := q.UpdateConversationView(ctx, updateParams(v, in.Fields, name, a.Clock.Now()))
		if database.IsUniqueViolation(err, "") {
			return exceptions.ErrViewNameTaken
		}

		if err != nil {
			return fmt.Errorf("update conversation view: %w", err)
		}

		out, err = a.finish(ctx, tx, saved{membership: in.Membership, row: row, action: "updated"})

		return err
	})

	return out, err
}

/*
updateParams keeps what the fields leave out.
*/
func updateParams(v models.ConversationView, f Fields, name string, now time.Time) models.UpdateConversationViewParams {
	p := models.UpdateConversationViewParams{
		WorkspaceID: v.WorkspaceID, ID: v.ID, Name: v.Name, Icon: v.Icon, Definition: v.Definition, Sort: v.Sort,
		StatusFilter: v.StatusFilter, Now: now,
	}

	if f.Name != nil {
		p.Name = name
	}

	if f.Icon != nil {
		p.Icon = blankNil(f.Icon)
	}

	if f.Definition != nil {
		p.Definition = f.Definition
	}

	if f.Sort != nil {
		p.Sort = blankNil(f.Sort)
	}

	if f.StatusFilter != nil {
		p.StatusFilter = blankNil(f.StatusFilter)
	}

	return p
}

/*
DeleteView moves a view to the trash.

Rules:
  - As for UpdateView.

Events: [appevents.ConversationViewChanged].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type DeleteView struct {
	Saver
}

/*
Handle deletes the view.
*/
func (a *DeleteView) Handle(ctx context.Context, in ShowInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		v, err := conversations.FindView(ctx, q, in.Membership, in.ID)
		if err != nil {
			return err
		}

		if err := canChange(in.Membership, v.Scope, v.OwnerMemberID); err != nil {
			return err
		}

		err = q.DeleteConversationView(ctx, models.DeleteConversationViewParams{
			WorkspaceID: v.WorkspaceID, ID: v.ID, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("delete conversation view: %w", err)
		}

		_, err = a.finish(ctx, tx, saved{membership: in.Membership, row: v, action: "deleted"})

		return err
	})
}

/*
ReorderInput is who orders the shared views, and the order: view ids,
first first. Views left out keep their place after those named.
*/
type ReorderInput struct {
	Membership authz.Membership
	Order      []ids.UUID
}

/*
ReorderViews puts the shared views in the order given.

Rules:
  - The caller needs conversations:write and views:manage_shared.
  - Every id names a shared view of the workspace.

Events: [appevents.ConversationViewChanged] for each view moved.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ReorderViews struct {
	Saver
}

/*
Handle orders the views and returns every view the caller sees.
*/
func (a *ReorderViews) Handle(ctx context.Context, in ReorderInput) ([]View, error) {
	if err := canChange(in.Membership, ScopeShared, nil); err != nil {
		return nil, err
	}

	var out []View

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if err := a.move(ctx, tx, in); err != nil {
			return err
		}

		rows, err := q.ListConversationViews(ctx, models.ListConversationViewsParams{
			WorkspaceID: in.Membership.Workspace.ID, MemberID: in.Membership.Member.ID,
		})
		if err != nil {
			return fmt.Errorf("list conversation views: %w", err)
		}

		out, err = warner{q: q, ids: a.IDs}.views(ctx, in.Membership, rows)

		return err
	})

	return out, err
}

/*
move puts each named shared view at its place in the order.
*/
func (a *ReorderViews) move(ctx context.Context, tx *database.Tx, in ReorderInput) error {
	q := models.New(tx)

	for position, id := range in.Order {
		v, err := conversations.FindView(ctx, q, in.Membership, id)
		if err != nil {
			return err
		}

		if v.Scope != ScopeShared {
			return exceptions.ErrNotFound
		}

		err = q.MoveSharedConversationView(ctx, models.MoveSharedConversationViewParams{
			WorkspaceID: v.WorkspaceID, ID: v.ID, Position: int32(position), Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("move conversation view: %w", err)
		}

		if _, err := a.finish(ctx, tx, saved{membership: in.Membership, row: v, action: "updated"}); err != nil {
			return err
		}
	}

	return nil
}
