/*
|--------------------------------------------------------------------------
| Conversation Neighbors
|--------------------------------------------------------------------------
|
| Moving to the next or previous conversation of a list without going
| back to it. The neighbors are found from where the conversation sits
| in the list's order, even when it no longer belongs to the list, such
| as right after closing it, so the next one is always the one that was
| below it.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
NeighborsInput is who asks, the conversation, and the list it is
moved through.
*/
type NeighborsInput struct {
	Membership authz.Membership
	Ref        Ref
	Filters    Filters
	Sort       querybuilder.Sort
}

/*
Neighbors are the conversations just before and after one in a list,
nil at either end.
*/
type Neighbors struct {
	Prev *models.Conversation
	Next *models.Conversation
}

/*
ShowNeighbors finds the conversations around one in a list.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowNeighbors struct {
	DB *database.DB
}

/*
Handle returns the neighbors.
*/
func (a *ShowNeighbors) Handle(ctx context.Context, in NeighborsInput) (Neighbors, error) {
	if err := CanRead(in.Membership); err != nil {
		return Neighbors{}, err
	}

	var out Neighbors

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		c, err := Find(ctx, models.New(tx), in.Membership, in.Ref)
		if err != nil {
			return err
		}

		list, ok := listOf(in.Membership, in.Filters, in.Sort)
		if !ok {
			return nil
		}

		list.After, list.Limit = new(positionOf(c, in.Sort)), 1

		if out.Next, err = first(ctx, tx, list); err != nil {
			return err
		}

		list.Reverse = true
		out.Prev, err = first(ctx, tx, list)

		return err
	})

	return out, err
}

/*
first returns the first conversation of the list, nil when it is
empty.
*/
func first(ctx context.Context, tx *database.Tx, list querybuilder.Conversations) (*models.Conversation, error) {
	rows, err := selectConversations(ctx, tx, list)
	if err != nil {
		return nil, fmt.Errorf("find neighbor: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	return &rows[0], nil
}
