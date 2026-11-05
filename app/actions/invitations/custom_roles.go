/*
|--------------------------------------------------------------------------
| Invitations With Custom Roles
|--------------------------------------------------------------------------
|
| An invitation may name the custom role its invitee starts with. Only
| someone who manages roles may offer one, and only a role holding
| nothing they lack, so an invitation never hands out more than its
| sender could. The invitee joins in the role's base role and the custom
| role at once; if the role was deleted in the meantime, the invitation's
| built-in role applies.
|
*/

package invitations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
offeredRole returns the built-in role the invitee is offered: the
custom role's base role when one is named, after checking the caller
may offer it.
*/
func offeredRole(
	ctx context.Context, q *models.Queries, m authz.Membership, invitee Invitee,
) (enums.Role, error) {
	if invitee.CustomRoleID == nil {
		return invitee.Role, nil
	}

	if !m.Can("roles:manage") {
		return "", exceptions.ErrForbidden
	}

	role, err := q.LockRole(ctx, models.LockRoleParams{WorkspaceID: m.Workspace.ID, ID: *invitee.CustomRoleID})
	if errors.Is(err, database.ErrNotFound) {
		return "", exceptions.ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf("lock offered role: %w", err)
	}

	var lacking []string

	for _, permission := range authz.HeldPermissions(enums.Role(role.BaseRole), role.Permissions) {
		if !m.Can(permission) {
			lacking = append(lacking, permission)
		}
	}

	if len(lacking) > 0 {
		return "", exceptions.ErrPermissionNotHeld.With("permissions", lacking)
	}

	return enums.Role(role.BaseRole), nil
}

/*
startInCustomRole gives the new member the custom role the invitation
names, when it still exists, and returns the member as they now are.
*/
func (a *AcceptInvitation) startInCustomRole(
	ctx context.Context, tx *database.Tx, inv models.Invitation, member models.WorkspaceMember,
) (models.WorkspaceMember, error) {
	if inv.CustomRoleID == nil {
		return member, nil
	}

	q := models.New(tx)

	role, err := q.LockRole(ctx, models.LockRoleParams{WorkspaceID: inv.WorkspaceID, ID: *inv.CustomRoleID})
	if errors.Is(err, database.ErrNotFound) {
		return member, nil
	}

	if err != nil {
		return member, fmt.Errorf("lock invited role: %w", err)
	}

	updated, err := q.SetMemberCustomRole(ctx, models.SetMemberCustomRoleParams{
		WorkspaceID: inv.WorkspaceID, ID: member.ID, Role: role.BaseRole, CustomRoleID: &role.ID,
		Now: a.Clock.Now(),
	})
	if err != nil {
		return member, fmt.Errorf("start in custom role: %w", err)
	}

	return updated, a.Events.Dispatch(ctx, tx, appevents.MemberCustomRoleChanged{
		WorkspaceID: inv.WorkspaceID, MemberID: updated.ID, UserID: updated.UserID, From: member.CustomRoleID,
		To: updated.CustomRoleID, ChangedBy: invitedBy(inv),
	})
}
