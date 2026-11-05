/*
|--------------------------------------------------------------------------
| Role Names
|--------------------------------------------------------------------------
|
| A member listed beside the custom role they hold shows its name, read
| for the whole page at once.
|
*/

package roles

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
RoleNames names the custom roles members hold, for showing them beside
each member.

Errors: a wrapped database error.
*/
type RoleNames struct {
	DB *database.DB
}

/*
Handle returns the name of each of the custom roles of the workspace,
by id. Ids of other workspaces' roles are left out.
*/
func (a *RoleNames) Handle(ctx context.Context, workspaceID ids.UUID, roleIDs []ids.UUID) (map[ids.UUID]string, error) {
	names := map[ids.UUID]string{}
	if len(roleIDs) == 0 {
		return names, nil
	}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListRolesByIDs(ctx, models.ListRolesByIDsParams{
			WorkspaceID: workspaceID, Ids: roleIDs,
		})
		for _, row := range rows {
			names[row.ID] = row.Name
		}

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("name roles: %w", err)
	}

	return names, nil
}
