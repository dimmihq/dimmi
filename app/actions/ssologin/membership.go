/*
|--------------------------------------------------------------------------
| Membership Through Single Sign-On
|--------------------------------------------------------------------------
|
| A person signing in through their company's identity provider must be
| an active member of the company's workspace. A connection that admits
| newcomers adds them with its default role, custom role and teams; it
| never brings back a member someone deactivated. When the identity
| provider reports groups, the connection's group mapping decides the
| member's role: the highest role any of their groups gives wins, and an
| owner is never touched.
|
*/

package ssologin

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
membership returns the user's active membership of the workspace, adding
it when the connection admits newcomers.
*/
func (a *CompleteSSOLogin) membership(ctx context.Context, j joining) (models.WorkspaceMember, error) {
	q, workspace, user := models.New(j.tx), j.workspace, j.user

	member, err := q.FindWorkspaceMemberForUser(ctx, models.FindWorkspaceMemberForUserParams{
		WorkspaceID: workspace.ID, UserID: user.ID,
	})
	if err == nil {
		if member.Status != string(enums.MemberStatusActive) {
			return member, exceptions.ErrMembershipDeactivated
		}

		return member, nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return member, fmt.Errorf("find membership: %w", err)
	}

	if !j.in.Connection.Connection.JitEnabled {
		return member, exceptions.ErrSSONoAccess
	}

	return a.join(ctx, j)
}

/*
joining is a person entering the company's workspace through a
connection: the transaction, the sign-in, the workspace and the user.
*/
type joining struct {
	tx        *database.Tx
	in        CompleteSSOLoginInput
	workspace models.Workspace
	user      models.User
}

/*
join adds the user to the workspace with the connection's defaults.
*/
func (a *CompleteSSOLogin) join(ctx context.Context, j joining) (models.WorkspaceMember, error) {
	tx, conn, workspace, user := j.tx, j.in.Connection.Connection, j.workspace, j.user
	q := models.New(tx)
	now := a.Clock.Now()
	via := appauth.MethodSSO

	member, err := q.CreateWorkspaceMember(ctx, models.CreateWorkspaceMemberParams{
		WorkspaceID: workspace.ID, UserID: user.ID, Role: conn.DefaultRole, JoinedVia: &via, Now: now,
	})
	if err != nil {
		return member, fmt.Errorf("add member through sso: %w", err)
	}

	if conn.DefaultCustomRoleID != nil {
		start := sso.GroupRole{Role: conn.DefaultRole, CustomRoleID: conn.DefaultCustomRoleID}
		if member, err = a.setRole(ctx, q, member, start); err != nil {
			return member, err
		}
	}

	for _, team := range conn.DefaultTeamIds {
		if _, err := q.AddTeamMember(ctx, models.AddTeamMemberParams{
			WorkspaceMemberID: member.ID, Now: now, WorkspaceID: workspace.ID, TeamID: team,
		}); err != nil {
			return member, fmt.Errorf("add member to default team: %w", err)
		}
	}

	return member, a.Events.Dispatch(ctx, tx, appevents.MemberJoined{
		WorkspaceID: workspace.ID, MemberID: member.ID, UserID: user.ID, JoinedVia: via,
	})
}

/*
applyGroups gives the member the role their groups map to, when the
identity provider reported groups and one of them is mapped. Owners keep
their role.
*/
func (a *CompleteSSOLogin) applyGroups(
	ctx context.Context, tx *database.Tx, in CompleteSSOLoginInput, member models.WorkspaceMember,
) error {
	if member.Role == string(enums.RoleOwner) || len(in.Identity.Groups) == 0 {
		return nil
	}

	mapped, ok := highest(in.Connection.GroupRoleMap, in.Identity.Groups)
	if !ok || (mapped.Role == member.Role && sameRole(mapped.CustomRoleID, member.CustomRoleID)) {
		return nil
	}

	updated, err := a.setRole(ctx, models.New(tx), member, mapped)
	if err != nil {
		return err
	}

	if updated.Role == member.Role {
		return nil
	}

	return a.Events.Dispatch(ctx, tx, appevents.MemberRoleChanged{
		WorkspaceID: member.WorkspaceID, MemberID: member.ID, UserID: member.UserID, From: member.Role,
		To: updated.Role, ChangedBy: ids.Nil(),
	})
}

/*
setRole gives the member the built-in role and the custom role the
target names, nil for none.
*/
func (a *CompleteSSOLogin) setRole(
	ctx context.Context, q *models.Queries, member models.WorkspaceMember, target sso.GroupRole,
) (models.WorkspaceMember, error) {
	updated, err := q.SetMemberCustomRole(ctx, models.SetMemberCustomRoleParams{
		Role: target.Role, CustomRoleID: target.CustomRoleID, Now: a.Clock.Now(), WorkspaceID: member.WorkspaceID,
		ID: member.ID,
	})
	if err != nil {
		return member, fmt.Errorf("set member role through sso: %w", err)
	}

	return updated, nil
}

/*
highest returns the mapping of the person's groups that gives the most:
admin over member, and among equals the first listed. A mapping without
a role gives member.
*/
func highest(mappings []sso.GroupRole, groups []string) (sso.GroupRole, bool) {
	var (
		best  sso.GroupRole
		found bool
	)

	for _, m := range mappings {
		if !slices.Contains(groups, m.Group) {
			continue
		}

		if m.Role == "" {
			m.Role = string(enums.RoleMember)
		}

		if !found || (m.Role == string(enums.RoleAdmin) && best.Role != string(enums.RoleAdmin)) {
			best, found = m, true
		}
	}

	return best, found
}

/*
sameRole reports whether two custom roles, nil for none, are the same.
*/
func sameRole(a, b *ids.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
