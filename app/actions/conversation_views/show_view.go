/*
|--------------------------------------------------------------------------
| Showing a View
|--------------------------------------------------------------------------
|
| One saved view with its warnings. Another member's personal view does
| not exist for the caller, so it is not found rather than forbidden.
|
*/

package conversationviews

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ShowInput is who asks about which view.
*/
type ShowInput struct {
	Membership authz.Membership
	ID         ids.UUID
}

/*
ShowView returns one view.

Rules:
  - The caller needs conversations:read.
  - Another member's personal view is not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowView struct {
	DB  *database.DB
	IDs *ids.Registry
}

/*
Handle returns the view.
*/
func (a *ShowView) Handle(ctx context.Context, in ShowInput) (View, error) {
	if err := conversations.CanRead(in.Membership); err != nil {
		return View{}, err
	}

	var out View

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		v, err := conversations.FindView(ctx, q, in.Membership, in.ID)
		if err != nil {
			return err
		}

		views, err := warner{q: q, ids: a.IDs}.views(ctx, in.Membership, []models.ConversationView{v})
		if err == nil {
			out = views[0]
		}

		return err
	})

	return out, err
}
