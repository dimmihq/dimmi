package workspaces_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestMemberWriteFailures(t *testing.T) {
	t.Parallel()

	t.Run("answers a server error and changes nothing when a write fails", func(t *testing.T) {
		t.Parallel()

		for table, kinds := range map[string][]string{
			"workspace_members":      {"UPDATE"},
			"team_members":           {"DELETE"},
			"personal_access_tokens": {"UPDATE"},
		} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				_, member := join(t, tc, enums.RoleMember)
				_, gone := join(t, tc, enums.RoleMember)
				require.NoError(t, factories.Deactivate(t.Context(), tc.App, gone))

				token := in(tc, tc.Owner)
				if table == "workspace_members" {
					token = tc.ActingAsToken(tc.Workspace.Owner, "members:write")
				}

				token.GetJSON("/api/v1/workspace").AssertOK()
				tc.FailWrites(table, kinds...)

				token.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).
					AssertStatus(500)
				token.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"})
				token.PatchJSON("/api/v1/members/"+memberID(gone), testkit.JSON{"status": "active"})
				tc.Table("workspace_members").AssertHas(testkit.Row{"id": member.ID, "status": "active"})
			})
		}
	})

	t.Run("rolls back when a listener refuses the change", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		_, gone := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, gone))
		refuse := errors.New("refused")
		d := tc.App.Events
		events.Listen(d, events.InTx, func(context.Context, appevents.MemberRoleChanged) error { return refuse })
		events.Listen(d, events.InTx, func(context.Context, appevents.MemberReactivated) error { return refuse })
		events.Listen(d, events.InTx, func(context.Context, appevents.WorkspaceUpdated) error { return refuse })
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:write", "settings:write")

		token.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"}).AssertStatus(500)
		token.PatchJSON("/api/v1/members/"+memberID(gone), testkit.JSON{"status": "active"}).AssertStatus(500)
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Refused"}).AssertStatus(500)
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": member.ID, "role": "member"})
	})

	t.Run("answers a server error when the workspace or its organization cannot be written", func(t *testing.T) {
		t.Parallel()

		for _, table := range []string{"workspaces", "organizations"} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				token := tc.ActingAsToken(tc.Workspace.Owner, "settings:write")
				tc.FailWrites(table, "UPDATE")

				token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Unsaved", "slug": "unsaved-slug"}).
					AssertStatus(500)
			})
		}
	})

	t.Run("reports failures of lookups to the caller", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.Workspace.Owner
		membership := authz.NewMembership(tc.Workspace.Workspace, owner, []string{"members:read", "settings:write"})
		tc.BreakTable("workspace_members")

		_, err := tc.App.Actions.ListMembers.Handle(t.Context(), members.ListMembersInput{
			Membership: membership, Limit: 10,
		})
		require.Error(t, err)

		_, err = tc.App.Actions.ShowMember.Handle(t.Context(), members.ShowMemberInput{
			Membership: membership, MemberID: owner.ID,
		})
		require.Error(t, err)

		_, err = tc.App.Actions.ListMemberships.Handle(t.Context(), workspaces.ListMembershipsInput{
			UserID: owner.UserID,
		})
		require.Error(t, err)

		err = tc.App.Actions.LeaveWorkspace.Handle(t.Context(), members.LeaveWorkspaceInput{
			UserID: owner.UserID, MemberID: owner.ID,
		})
		require.Error(t, err)
	})

	t.Run("refuses member lists to a membership without the permission", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		membership := authz.NewMembership(tc.Workspace.Workspace, tc.Workspace.Owner, nil)

		_, err := tc.App.Actions.ListMembers.Handle(t.Context(), members.ListMembersInput{Membership: membership})
		require.ErrorContains(t, err, "unauthorized")

		_, err = tc.App.Actions.ShowMember.Handle(t.Context(), members.ShowMemberInput{Membership: membership})
		require.ErrorContains(t, err, "unauthorized")
	})
}
