package workspaces_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestWorkspaceStorageFailures(t *testing.T) {
	t.Parallel()

	t.Run("reports a workspace table that cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		owner := ownerMembership(tc)
		name := "Gone"
		tc.BreakTable("workspaces")
		acts := tc.App.Actions

		_, err := acts.UpdateMember.Handle(t.Context(), members.UpdateMemberInput{
			Principal: &appauth.Principal{Kind: appauth.KindSession}, Membership: owner, MemberID: member.ID,
		})
		require.ErrorContains(t, err, "lock workspace")

		_, err = acts.UpdateWorkspace.Handle(t.Context(), workspaces.UpdateWorkspaceInput{
			Membership: owner, Name: &name,
		})
		require.ErrorContains(t, err, "lock workspace")

		_, err = acts.RepairWorkspaces.Handle(t.Context(), "")
		require.ErrorContains(t, err, "list workspaces")

		_, err = acts.RepairWorkspaces.Handle(t.Context(), "acme")
		require.ErrorContains(t, err, "find workspace acme")

		_, err = acts.ResolveWorkspace.Handle(t.Context(), workspaces.ResolveWorkspaceInput{
			Principal: &appauth.Principal{User: tc.Owner, Kind: appauth.KindSession},
		})
		require.ErrorContains(t, err, "list workspaces of user")
	})

	t.Run("reports member rows that cannot be locked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		owner := ownerMembership(tc)
		exec(t, tc, "ALTER TABLE workspace_members DROP COLUMN deactivated_by_member_id")

		_, err := tc.App.Actions.UpdateMember.Handle(t.Context(), members.UpdateMemberInput{
			Principal: &appauth.Principal{Kind: appauth.KindSession}, Membership: owner, MemberID: member.ID,
		})
		require.ErrorContains(t, err, "lock member")

		err = tc.App.Actions.LeaveWorkspace.Handle(t.Context(), members.LeaveWorkspaceInput{
			UserID: member.UserID, MemberID: member.ID,
		})
		require.ErrorContains(t, err, "lock own membership")
	})

	t.Run("reports a member access that cannot be recorded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		tc.FailWrites("workspace_members", "UPDATE")

		owner.GetJSON("/api/v1/workspace").AssertStatus(500)
	})

	t.Run("reports workspace settings it cannot read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)

		exec(t, tc, `UPDATE workspaces SET settings = '{"previous_slugs": "broken"}' WHERE id = $1`,
			tc.Workspace.Workspace.ID)
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": "fresh-start"}).AssertStatus(500)

		exec(t, tc, `UPDATE workspaces SET settings = '[]' WHERE id = $1`, tc.Workspace.Workspace.ID)
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": "fresh-start"}).AssertStatus(500)
		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "owner"}).AssertOK()
	})
}

/*
ownerMembership returns the test case's owner's membership of the
workspace with every permission of the owner role.
*/
func ownerMembership(tc *tests.TestCase) authz.Membership {
	return authz.NewMembership(tc.Workspace.Workspace, tc.Workspace.Owner,
		appauth.RolePermissions(enums.RoleOwner))
}
