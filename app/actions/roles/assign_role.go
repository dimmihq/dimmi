/*
|--------------------------------------------------------------------------
| Assign Role
|--------------------------------------------------------------------------
|
| Giving a member a custom role makes them a holder of its base role that
| does exactly what the custom role lists. Taking it away leaves them the
| whole base role, which is a widening, so it follows the same rule as
| handing out a role: the caller must hold everything the member ends up
| with. Owners never hold a custom role, and nobody changes their own, so
| nobody locks themselves out or lifts themselves up by accident.
|
*/

package roles

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AssignRoleInput is who assigns, through their membership, the member and
the custom role they get, nil to take theirs away.
*/
type AssignRoleInput struct {
	Membership authz.Membership
	MemberID   ids.UUID
	CustomRole *ids.UUID
}

/*
AssignRole gives a member of the caller's workspace a custom role, or
takes it away.

Rules:
  - The caller needs roles:manage.
  - Members and roles of other workspaces are not found.
  - Owners never hold a custom role, and nobody assigns their own.
  - The caller must hold everything the member ends up holding.
  - The member's built-in role becomes the custom role's base role.
  - The role the member already holds changes nothing.

Events: [appevents.MemberRoleChanged] when the built-in role changed,
then [appevents.MemberCustomRoleChanged].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInvalidTransition], [exceptions.ErrPermissionNotHeld], or
a wrapped database error.
*/
type AssignRole struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle assigns the role and returns the member as they now are.
*/
func (a *AssignRole) Handle(ctx context.Context, in AssignRoleInput) (models.FindWorkspaceMemberWithUserRow, error) {
	if !in.Membership.Can(managePermission) {
		return models.FindWorkspaceMemberWithUserRow{}, exceptions.ErrForbidden
	}

	var out models.FindWorkspaceMemberWithUserRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		workspaceID := in.Membership.Workspace.ID

		target, err := lockMember(ctx, q, workspaceID, in.MemberID)
		if err != nil {
			return err
		}

		if err := a.assign(ctx, tx, target, in); err != nil {
			return err
		}

		out, err = q.FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
			WorkspaceID: workspaceID, ID: target.ID,
		})
		if err != nil {
			return fmt.Errorf("find assigned member: %w", err)
		}

		return nil
	})

	return out, err
}

/*
assign checks the change against the member and the caller, then writes
and announces it.
*/
func (a *AssignRole) assign(
	ctx context.Context, tx *database.Tx, target models.WorkspaceMember, in AssignRoleInput,
) error {
	if appauth.IsOwnerRole(enums.Role(target.Role)) || target.ID == in.Membership.Member.ID {
		return exceptions.ErrInvalidTransition.OnField("custom_role_id").With("from", target.Role)
	}

	if sameRole(target.CustomRoleID, in.CustomRole) {
		return nil
	}

	role, granted, err := a.grant(ctx, models.New(tx), target, in)
	if err != nil {
		return err
	}

	if bad := outside(granted, in.Membership.Can); len(bad) > 0 {
		return exceptions.ErrPermissionNotHeld.With("permissions", bad)
	}

	updated, err := models.New(tx).SetMemberCustomRole(ctx, models.SetMemberCustomRoleParams{
		WorkspaceID: target.WorkspaceID, ID: target.ID, Role: role, CustomRoleID: in.CustomRole, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("assign role: %w", err)
	}

	return a.announce(ctx, tx, change{before: target, after: updated, by: in.Membership.Member.ID})
}

/*
change is an assignment written: the member before and after, and the
member who made it.
*/
type change struct {
	before, after models.WorkspaceMember
	by            ids.UUID
}

/*
grant returns the built-in role the member ends up with and everything
they then hold: the custom role's base role and list, or their own role
as a whole when the custom role is taken away.
*/
func (a *AssignRole) grant(
	ctx context.Context, q *models.Queries, target models.WorkspaceMember, in AssignRoleInput,
) (string, []string, error) {
	if in.CustomRole == nil {
		return target.Role, appauth.RolePermissions(enums.Role(target.Role)), nil
	}

	role, err := lockRole(ctx, q, target.WorkspaceID, *in.CustomRole)
	if err != nil {
		return "", nil, err
	}

	return role.BaseRole, authz.HeldPermissions(enums.Role(role.BaseRole), role.Permissions), nil
}

/*
announce dispatches what the assignment changed: the built-in role, if
it moved, then the custom role.
*/
func (a *AssignRole) announce(ctx context.Context, tx *database.Tx, c change) error {
	before, after, by := c.before, c.after, c.by

	if appauth.RoleChanged(before.Role, after.Role) {
		err := a.Events.Dispatch(ctx, tx, appevents.MemberRoleChanged{
			WorkspaceID: after.WorkspaceID, MemberID: after.ID, UserID: after.UserID, From: before.Role, To: after.Role,
			ChangedBy: by,
		})
		if err != nil {
			return err
		}
	}

	return a.Events.Dispatch(ctx, tx, appevents.MemberCustomRoleChanged{
		WorkspaceID: after.WorkspaceID, MemberID: after.ID, UserID: after.UserID, From: before.CustomRoleID,
		To: after.CustomRoleID, ChangedBy: by,
	})
}

/*
sameRole reports whether two custom roles are the same, none included.
*/
func sameRole(a, b *ids.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

/*
lockMember returns the member and holds the row until the transaction
ends, or exceptions.ErrNotFound.
*/
func lockMember(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.WorkspaceMember, error) {
	member, err := q.LockWorkspaceMember(ctx, models.LockWorkspaceMemberParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return member, exceptions.ErrNotFound
	}

	if err != nil {
		return member, fmt.Errorf("lock member: %w", err)
	}

	return member, nil
}
