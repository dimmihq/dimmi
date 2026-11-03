package workspaces_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

/*
ownerWith returns the test case owner's membership with the
permissions given.
*/
func ownerWith(tc *tests.TestCase, permissions ...string) authz.Membership {
	return authz.NewMembership(tc.Workspace.Workspace, tc.Workspace.Owner, permissions)
}

func TestTeamWriteFailures(t *testing.T) {
	t.Parallel()

	t.Run("answers a server error and changes nothing when a team write fails", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support")
		billing := team(t, tc, "Billing", member)
		owner := in(tc, tc.Owner)
		tc.FailWrites("teams", "INSERT", "UPDATE")
		tc.FailWrites("team_members", "INSERT", "DELETE")

		owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Sales"}).AssertStatus(500)
		owner.PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": "Care"}).AssertStatus(500)
		owner.Delete("/api/v1/teams/" + teamID(support)).AssertStatus(500)
		owner.PostJSON("/api/v1/teams/"+teamID(support)+"/members", testkit.JSON{
			"member_ids": []string{memberID(member)},
		}).AssertStatus(500)
		owner.Delete("/api/v1/teams/" + teamID(billing) + "/members/" + memberID(member)).AssertStatus(500)
		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).
			AssertStatus(500)

		tc.Table("teams").AssertHas(testkit.Row{"id": support.ID, "name": "Support"})
		tc.Table("team_members").AssertCount(1, testkit.Row{"workspace_member_id": member.ID})
	})

	t.Run("rolls back when a listener refuses the change", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		_, other := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support", member)
		refuse := errors.New("refused")
		d := tc.App.Events
		events.Listen(d, events.InTx, func(context.Context, appevents.TeamCreated) error { return refuse })
		events.Listen(d, events.InTx, func(context.Context, appevents.TeamUpdated) error { return refuse })
		events.Listen(d, events.InTx, func(context.Context, appevents.TeamDeleted) error { return refuse })
		events.Listen(d, events.InTx, func(context.Context, appevents.TeamMemberAdded) error { return refuse })
		events.Listen(d, events.InTx, func(context.Context, appevents.TeamMemberRemoved) error { return refuse })
		owner := in(tc, tc.Owner)

		owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Sales"}).AssertStatus(500)
		owner.PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": "Care"}).AssertStatus(500)
		owner.Delete("/api/v1/teams/" + teamID(support)).AssertStatus(500)
		owner.PostJSON("/api/v1/teams/"+teamID(support)+"/members", testkit.JSON{
			"member_ids": []string{memberID(other)},
		}).AssertStatus(500)
		owner.Delete("/api/v1/teams/" + teamID(support) + "/members/" + memberID(member)).AssertStatus(500)
		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).
			AssertStatus(500)

		owner.GetJSON("/api/v1/teams?include=members").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.name", "Support").
			AssertJSONPath("data.0.member_count", 1)
	})

	t.Run("reports failures of lookups to the caller", func(t *testing.T) {
		t.Parallel()

		for _, table := range []string{"teams", "team_members", "users", "workspace_members"} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				support := team(t, tc, "Support")
				ctx := t.Context()
				m := ownerWith(tc, "members:read", "members:write")
				acts := tc.App.Actions
				tc.BreakTable(table)

				_, listErr := acts.ListTeams.Handle(ctx, teams.ListTeamsInput{
					Membership: m, WithMembers: true, Limit: 5,
				})
				_, showErr := acts.ShowTeam.Handle(ctx, teams.ShowTeamInput{
					Membership: m, TeamID: support.ID, WithMembers: true,
				})
				_, addErr := acts.AddTeamMembers.Handle(ctx, teams.AddTeamMembersInput{
					Membership: m, TeamID: support.ID, MemberIDs: []ids.UUID{tc.Workspace.Owner.ID},
				})
				_, teamsErr := acts.ListMemberTeams.Handle(ctx, teams.ListMemberTeamsInput{
					Membership: m, MemberIDs: []ids.UUID{tc.Workspace.Owner.ID},
				})
				_, purgeErr := acts.PurgeTeam.Handle(ctx, teams.PurgeTeamInput{
					WorkspaceID: tc.Workspace.Workspace.ID, TeamID: support.ID,
				})

				require.Error(t, errors.Join(listErr, showErr, addErr, teamsErr, purgeErr))
			})
		}
	})

	t.Run("refuses team reads to a membership without the permission", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		m := ownerWith(tc)
		acts := tc.App.Actions

		_, err := acts.ListTeams.Handle(t.Context(), teams.ListTeamsInput{Membership: m})
		require.ErrorContains(t, err, "unauthorized")

		_, err = acts.ShowTeam.Handle(t.Context(), teams.ShowTeamInput{Membership: m})
		require.ErrorContains(t, err, "unauthorized")

		_, err = acts.ListMemberTeams.Handle(t.Context(), teams.ListMemberTeamsInput{Membership: m})
		require.ErrorContains(t, err, "unauthorized")
	})
}

func TestPurgeTeam(t *testing.T) {
	t.Parallel()

	t.Run("removes a deleted team with its memberships and leaves live teams alone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		gone := team(t, tc, "Support", member)
		live := team(t, tc, "Billing", member)
		in(tc, tc.Owner).Delete("/api/v1/teams/" + teamID(gone)).AssertNoContent()
		ctx := tenancy.With(t.Context(), tc.Workspace.Workspace.ID.String())
		purge := func(id ids.UUID) bool {
			purged, err := tc.App.Actions.PurgeTeam.Handle(ctx, teams.PurgeTeamInput{
				WorkspaceID: tc.Workspace.Workspace.ID, TeamID: id,
			})
			require.NoError(t, err)

			return purged
		}

		assert.True(t, purge(gone.ID))
		assert.False(t, purge(gone.ID))
		assert.False(t, purge(live.ID))
		tc.Table("teams").AssertMissing(testkit.Row{"id": gone.ID}).AssertHas(testkit.Row{"id": live.ID})
		tc.Table("team_members").AssertCount(1, testkit.Row{"workspace_member_id": member.ID})
	})
}
