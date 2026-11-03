/*
|--------------------------------------------------------------------------
| List Memberships
|--------------------------------------------------------------------------
|
| A user's own memberships, across every workspace, the one used most
| recently first: the first one is where an app opens. Workspaces in the
| trash stay hidden unless asked for, and only their owners ever see
| them, since only an owner can bring one back.
|
*/

package workspaces

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
The ways trashed workspaces can be asked for: left out, listed with the
rest, or listed alone.
*/
const (
	TrashedWithout = ""
	TrashedWith    = "with"
	TrashedOnly    = "only"
)

/*
ListMembershipsInput is the user and how to treat workspaces in the
trash.
*/
type ListMembershipsInput struct {
	UserID  ids.UUID
	Trashed string
}

/*
ListMemberships lists the user's active memberships.

Rules:
  - Workspaces in the trash are listed only when asked for, and only to
    their owners.

Errors: a wrapped database error.
*/
type ListMemberships struct {
	DB *database.DB
}

/*
Handle returns the memberships.
*/
func (a *ListMemberships) Handle(
	ctx context.Context, in ListMembershipsInput,
) ([]models.ListUserMembershipsRow, error) {
	rows, err := models.New(a.DB).ListUserMemberships(ctx, models.ListUserMembershipsParams{
		UserID: in.UserID, Trashed: in.Trashed,
	})
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}

	return rows, nil
}
