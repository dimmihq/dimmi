/*
|--------------------------------------------------------------------------
| Member Custom Roles
|--------------------------------------------------------------------------
|
| A member holding a custom role does what that role lists and nothing
| more of their built-in role. The list is read fresh on every request,
| so editing a role binds its holders on their very next one.
|
*/

package models

import (
	"context"
	"fmt"
)

/*
CustomPermissions returns what the member's custom role lists, or nil
when they hold none.
*/
func (q *Queries) CustomPermissions(ctx context.Context, m WorkspaceMember) ([]string, error) {
	if m.CustomRoleID == nil {
		return nil, nil
	}

	permissions, err := q.FindRolePermissions(ctx, FindRolePermissionsParams{
		WorkspaceID: m.WorkspaceID, ID: *m.CustomRoleID,
	})
	if err != nil {
		return nil, fmt.Errorf("find custom role permissions: %w", err)
	}

	if permissions == nil {
		permissions = []string{}
	}

	return permissions, nil
}
