/*
|--------------------------------------------------------------------------
| Saved Views
|--------------------------------------------------------------------------
|
| A saved view is a named list of conversations a team shares, or one
| member keeps for themselves. Everyone sees the shared views and their
| own; another member's personal views do not exist for them. A view
| whose conditions name a tag, team or inbox that was since deleted
| still loads: the condition matches nothing, and the view says which
| condition it is so the editor can flag it.
|
*/

package conversationviews

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
Warning flags a condition of a view: its path and why, such as
reference_missing for a deleted row.
*/
type Warning struct {
	Path string
	Code string
}

/*
View is a saved view with the warnings about its conditions.
*/
type View struct {
	View     models.ConversationView
	Warnings []Warning
}

/*
ListInput is who asks, and the text a view's name must hold, empty for
every view.
*/
type ListInput struct {
	Membership authz.Membership
	Search     string
}

/*
ListViews returns the shared views in their order, then the caller's
own.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListViews struct {
	DB  *database.DB
	IDs *ids.Registry
}

/*
Handle returns the views.
*/
func (a *ListViews) Handle(ctx context.Context, in ListInput) ([]View, error) {
	if err := conversations.CanRead(in.Membership); err != nil {
		return nil, err
	}

	var out []View

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListConversationViews(ctx, models.ListConversationViewsParams{
			WorkspaceID: in.Membership.Workspace.ID, MemberID: in.Membership.Member.ID, Search: in.Search,
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
warner finds the conditions of views naming rows that are gone.
*/
type warner struct {
	q   *models.Queries
	ids *ids.Registry
}

/*
views adds to each view the conditions naming rows that are gone.
*/
func (w warner) views(ctx context.Context, m authz.Membership, rows []models.ConversationView) ([]View, error) {
	q := w.q
	compiler := conversations.NewCompiler(m, w.ids, m.Workspace.CreatedAt)
	out := make([]View, len(rows))
	refs := make([][]querybuilder.Reference, len(rows))

	var wanted []ids.UUID

	for i, row := range rows {
		out[i].View = row

		d, err := conversations.ReadDefinition(row.Definition)
		if err != nil {
			continue
		}

		compiled, err := compiler.Compile(d)
		if err != nil {
			continue
		}

		refs[i] = compiled.References
		for _, r := range compiled.References {
			wanted = append(wanted, r.ID)
		}
	}

	if len(wanted) == 0 {
		return out, nil
	}

	live, err := q.LiveReferences(ctx, models.LiveReferencesParams{WorkspaceID: m.Workspace.ID, Ids: wanted})
	if err != nil {
		return nil, fmt.Errorf("find rows views name: %w", err)
	}

	for i := range out {
		for _, r := range refs[i] {
			if !slices.Contains(live, r.ID) {
				out[i].Warnings = append(out[i].Warnings, Warning{Path: r.Path, Code: "reference_missing"})
			}
		}
	}

	return out, nil
}
