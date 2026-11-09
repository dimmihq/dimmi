/*
|--------------------------------------------------------------------------
| Operator Memories
|--------------------------------------------------------------------------
|
| What a teammate asked Operator to remember is theirs alone: they can
| read all of it and make Operator forget any of it.
|
*/

package operatorchat

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
memoriesListed is the most memories one list shows.
*/
const memoriesListed = 200

/*
Memories lists and forgets a teammate's memories.

Rules:
  - The caller needs conversations:read and sees only their own.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type Memories struct {
	DB *database.DB
}

/*
List returns the teammate's memories, newest first.
*/
func (a *Memories) List(ctx context.Context, m authz.Membership) ([]models.OperatorMemory, error) {
	if err := conversations.CanRead(m); err != nil {
		return nil, err
	}
	var out []models.OperatorMemory
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListOperatorMemories(ctx, models.ListOperatorMemoriesParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, RowLimit: memoriesListed,
		})
		if err != nil {
			return fmt.Errorf("list operator memories: %w", err)
		}
		return nil
	})
	return out, err
}

/*
Delete forgets one of the teammate's memories.
*/
func (a *Memories) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := conversations.CanRead(m); err != nil {
		return err
	}
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		n, err := models.New(tx).DeleteOperatorMemory(ctx, models.DeleteOperatorMemoryParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("forget operator memory: %w", err)
		}
		if n == 0 {
			return exceptions.ErrNotFound
		}
		return nil
	})
}
