package workspaces_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
teamID returns the public id of the team.
*/
func teamID(team models.Team) string {
	return models.PublicID(models.PrefixTeam, team.ID)
}

/*
team creates a team in the test case's workspace with the members.
*/
func team(t *testing.T, tc *tests.TestCase, name string, members ...models.WorkspaceMember) models.Team {
	t.Helper()

	created, err := factories.Team(t.Context(), tc.App, factories.TeamOf{
		Workspace: tc.Workspace.Workspace, Name: name, Members: members,
	})
	require.NoError(t, err)

	return created
}

func TestTeamsIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists live teams by name with member counts to any member", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		team(t, tc, "support", member, tc.Workspace.Owner)
		team(t, tc, "Billing")
		gone := team(t, tc, "Archive")
		in(tc, tc.Owner).Delete("/api/v1/teams/" + teamID(gone)).AssertNoContent()

		in(tc, user).GetJSON("/api/v1/teams").AssertOK().
			AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.object", "team").
			AssertJSONPath("data.0.name", "Billing").
			AssertJSONPath("data.0.member_count", 0).
			AssertJSONPath("data.1.name", "support").
			AssertJSONPath("data.1.member_count", 2).
			AssertJSONMissing("data.1.members").
			AssertJSONPath("meta.next_cursor", nil)
	})

	t.Run("includes each team's members by name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		team(t, tc, "Support", member)
		team(t, tc, "Empty")

		res := in(tc, tc.Owner).GetJSON("/api/v1/teams?include=members").AssertOK().
			AssertJSONPath("data.0.members.#", 0).
			AssertJSONPath("data.1.members.#", 1).
			AssertJSONPath("data.1.members.0.object", "member")
		assert.Equal(t, memberID(member), res.String("data.1.members.0.id"))
	})

	t.Run("pages with a cursor and refuses unknown includes and forged cursors", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		for _, name := range []string{"A", "B", "C"} {
			team(t, tc, name)
		}
		owner := in(tc, tc.Owner)

		first := owner.GetJSON("/api/v1/teams?limit=2").AssertOK().AssertJSONPath("data.#", 2)
		owner.GetJSON("/api/v1/teams?limit=2&cursor="+first.String("meta.next_cursor")).AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.name", "C")
		owner.GetJSON("/api/v1/teams?include=owners").AssertStatus(400)
		owner.GetJSON("/api/v1/teams?cursor=forged").AssertStatus(400)
	})

	t.Run("lets a token with members:read list and refuses one without", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		team(t, tc, "Support")

		tc.ActingAsToken(tc.Workspace.Owner, "members:read").GetJSON("/api/v1/teams").AssertOK().
			AssertJSONPath("data.#", 1)
		tc.ActingAsToken(tc.Workspace.Owner, "settings:read").GetJSON("/api/v1/teams").AssertForbidden()
	})
}

func TestTeamsShow(t *testing.T) {
	t.Parallel()

	t.Run("shows a team with its members and nothing of another workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support", member)
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		foreign, err := factories.Team(t.Context(), tc.App, factories.TeamOf{
			Workspace: other.Workspace, Name: "Support",
		})
		require.NoError(t, err)
		client := in(tc, user)

		client.GetJSON("/api/v1/teams/"+teamID(support)).AssertOK().
			AssertJSONPath("data.id", teamID(support)).
			AssertJSONPath("data.member_count", 1).
			AssertJSONMissing("data.members")
		client.GetJSON("/api/v1/teams/"+teamID(support)+"?include=members").AssertOK().
			AssertJSONPath("data.members.0.id", memberID(member))
		client.GetJSON("/api/v1/teams/" + teamID(support) + "?include=owners").AssertStatus(400)

		for _, id := range []string{teamID(foreign), "team_nope", memberID(member)} {
			client.GetJSON("/api/v1/teams/"+id).AssertNotFound().AssertJSONPath("code", "not_found")
		}
	})
}

func TestTeamsStore(t *testing.T) {
	t.Parallel()

	t.Run("lets an admin create a team, cleaned, and announces it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, adminMember := join(t, tc, enums.RoleAdmin)
		created := record[appevents.TeamCreated](tc)

		res := in(tc, admin).PostJSON("/api/v1/teams", testkit.JSON{"name": "  Support\u200b "}).AssertCreated().
			AssertJSONPath("data.object", "team").
			AssertJSONPath("data.name", "Support").
			AssertJSONPath("data.member_count", 0)

		require.Len(t, created.all(), 1)
		assert.Equal(t, adminMember.ID, created.all()[0].CreatedBy)
		assert.Equal(t, res.String("data.id"), models.PublicID(models.PrefixTeam, created.all()[0].TeamID))
	})

	t.Run("refuses a name another live team uses in any case or Unicode form", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		team(t, tc, "Caf\u00e9")
		owner := in(tc, tc.Owner)

		for _, name := range []string{"CAF\u00c9", "cafe\u0301"} {
			owner.PostJSON("/api/v1/teams", testkit.JSON{"name": name}).AssertStatus(422).
				AssertJSONPath("code", "team_name_taken")
		}
	})

	t.Run("lets a deleted team's name be used again at once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		gone := team(t, tc, "Support")
		owner := in(tc, tc.Owner)

		owner.Delete("/api/v1/teams/" + teamID(gone)).AssertNoContent()
		owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "support"}).AssertCreated()
		tc.Table("teams").AssertCount(2, testkit.Row{"workspace_id": tc.Workspace.Workspace.ID})
	})

	t.Run("refuses a missing, invisible or overlong name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		for _, body := range []testkit.JSON{{}, {"name": "\u200b "}, {"name": string(make([]byte, 81))}} {
			owner.PostJSON("/api/v1/teams", body).AssertStatus(422)
		}
	})

	t.Run("keeps members and narrow tokens from creating teams", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)

		in(tc, member).PostJSON("/api/v1/teams", testkit.JSON{"name": "Support"}).AssertForbidden().
			AssertJSONPath("code", "forbidden")
		tc.ActingAsToken(tc.Workspace.Owner, "members:read").
			PostJSON("/api/v1/teams", testkit.JSON{"name": "Support"}).AssertForbidden()
		tc.Table("teams").AssertCount(0, testkit.Row{"workspace_id": tc.Workspace.Workspace.ID})
	})
}

func TestTeamsUpdate(t *testing.T) {
	t.Parallel()

	t.Run("renames a team and announces the change", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support", member)
		updated := record[appevents.TeamUpdated](tc)

		in(tc, tc.Owner).PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": "Customer care"}).
			AssertOK().
			AssertJSONPath("data.name", "Customer care").
			AssertJSONPath("data.member_count", 1)

		require.Len(t, updated.all(), 1)
		assert.Equal(t, "Support", updated.all()[0].From)
		assert.Equal(t, "Customer care", updated.all()[0].To)
	})

	t.Run("answers the current name without an event and changes only the case", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		support := team(t, tc, "Support")
		updated := record[appevents.TeamUpdated](tc)
		owner := in(tc, tc.Owner)

		owner.PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": "Support "}).AssertOK()
		assert.Empty(t, updated.all())

		owner.PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": "SUPPORT"}).AssertOK().
			AssertJSONPath("data.name", "SUPPORT")
		assert.Len(t, updated.all(), 1)
	})

	t.Run("refuses another team's name, other workspaces' teams and members", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support")
		team(t, tc, "Billing")
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		foreign, err := factories.Team(t.Context(), tc.App, factories.TeamOf{Workspace: other.Workspace, Name: "Sales"})
		require.NoError(t, err)
		owner := in(tc, tc.Owner)

		owner.PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": "billing"}).AssertStatus(422).
			AssertJSONPath("code", "team_name_taken")
		owner.PatchJSON("/api/v1/teams/"+teamID(foreign), testkit.JSON{"name": "Mine"}).AssertNotFound()
		owner.PatchJSON("/api/v1/teams/team_nope", testkit.JSON{"name": "Mine"}).AssertNotFound()
		owner.PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": ""}).AssertStatus(422)
		in(tc, member).PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": "Mine"}).AssertForbidden()
		tc.Table("teams").AssertHas(testkit.Row{"id": foreign.ID, "name": "Sales"})
	})
}

func TestTeamsDestroy(t *testing.T) {
	t.Parallel()

	t.Run("hides the team at once and announces it, keeping the row for the sweep", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support", member)
		deleted := record[appevents.TeamDeleted](tc)
		owner := in(tc, tc.Owner)

		owner.Delete("/api/v1/teams/" + teamID(support)).AssertNoContent()

		require.Len(t, deleted.all(), 1)
		assert.Equal(t, support.ID, deleted.all()[0].TeamID)
		owner.GetJSON("/api/v1/teams/" + teamID(support)).AssertNotFound()
		owner.Delete("/api/v1/teams/" + teamID(support)).AssertNotFound()
		owner.Delete("/api/v1/teams/team_nope").AssertNotFound()
		owner.PostJSON("/api/v1/teams/"+teamID(support)+"/members", testkit.JSON{
			"member_ids": []string{memberID(member)},
		}).AssertNotFound()
		owner.PatchJSON("/api/v1/teams/"+teamID(support), testkit.JSON{"name": "Back"}).AssertNotFound()
		tc.Table("teams").AssertCount(1, testkit.Row{"id": support.ID})
		tc.Table("team_members").AssertCount(1, testkit.Row{"team_id": support.ID})
	})

	t.Run("takes the team's conversations away from it first, each on its timeline", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		support := team(t, tc, "Support")
		assigned := record[appevents.ConversationAssigned](tc)
		owner := in(tc, tc.Owner)
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		conversation := owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": models.PublicID(models.PrefixInbox, inbox.Inbox.ID), "author": "contact",
			"body": "Hello", "contact": testkit.JSON{"email": "emily@example.com"},
		}).AssertCreated().String("data.id")
		owner.PatchJSON("/api/v1/conversations/"+conversation, testkit.JSON{"team_id": teamID(support)}).AssertOK()

		owner.Delete("/api/v1/teams/" + teamID(support)).AssertNoContent()

		owner.GetJSON("/api/v1/conversations/"+conversation).AssertOK().AssertJSONPath("data.team", nil)
		require.Len(t, assigned.all(), 2)
		assert.Equal(t, support.ID, *assigned.all()[1].FromTeamID)
		assert.Nil(t, assigned.all()[1].ToTeamID)
	})

	t.Run("keeps members from deleting teams", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support")

		in(tc, member).Delete("/api/v1/teams/" + teamID(support)).AssertForbidden()
		in(tc, member).GetJSON("/api/v1/teams/" + teamID(support)).AssertOK()
	})
}

func TestTeamMembersStore(t *testing.T) {
	t.Parallel()

	t.Run("adds members one result each, skipping the inactive and unknown", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, first := join(t, tc, enums.RoleMember)
		_, already := join(t, tc, enums.RoleAdmin)
		_, gone := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, gone))
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		support := team(t, tc, "Support", already)
		added := record[appevents.TeamMemberAdded](tc)
		ids := []string{
			memberID(first), memberID(already), memberID(gone), memberID(other.Owner), "mem_nope", memberID(first),
		}

		in(tc, tc.Owner).PostJSON("/api/v1/teams/"+teamID(support)+"/members", testkit.JSON{"member_ids": ids}).
			AssertOK().
			AssertJSONPath("data.#", 6).
			AssertJSONPath("data.0.member_id", memberID(first)).
			AssertJSONPath("data.0.result", "added").
			AssertJSONPath("data.1.result", "already_member").
			AssertJSONPath("data.2.result", "inactive").
			AssertJSONPath("data.3.result", "not_found").
			AssertJSONPath("data.4.member_id", "mem_nope").
			AssertJSONPath("data.4.result", "not_found").
			AssertJSONPath("data.5.result", "already_member")

		require.Len(t, added.all(), 1)
		assert.Equal(t, first.ID, added.all()[0].MemberID)
		tc.Table("team_members").
			AssertHas(testkit.Row{
				"team_id": support.ID, "workspace_member_id": first.ID, "workspace_id": tc.Workspace.Workspace.ID,
			}).
			AssertMissing(testkit.Row{"team_id": support.ID, "workspace_member_id": gone.ID})
		in(tc, tc.Owner).GetJSON("/api/v1/teams/"+teamID(support)).AssertJSONPath("data.member_count", 2)
	})

	t.Run("refuses an empty or oversized list, other workspaces' teams and members", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, someone := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support")
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		foreign, err := factories.Team(t.Context(), tc.App, factories.TeamOf{
			Workspace: other.Workspace, Name: "Support",
		})
		require.NoError(t, err)
		owner := in(tc, tc.Owner)
		path := "/api/v1/teams/" + teamID(support) + "/members"

		owner.PostJSON(path, testkit.JSON{"member_ids": []string{}}).AssertStatus(422)
		owner.PostJSON(path, testkit.JSON{"member_ids": make([]string, 101)}).AssertStatus(422)
		owner.PostJSON("/api/v1/teams/"+teamID(foreign)+"/members", testkit.JSON{
			"member_ids": []string{memberID(someone)},
		}).AssertNotFound()
		in(tc, member).PostJSON(path, testkit.JSON{"member_ids": []string{memberID(someone)}}).AssertForbidden()
		tc.Table("team_members").AssertCount(0, testkit.Row{"workspace_member_id": someone.ID})
	})
}

func TestTeamMembersDestroy(t *testing.T) {
	t.Parallel()

	t.Run("takes a member out of the team and announces it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support", member)
		removed := record[appevents.TeamMemberRemoved](tc)
		owner := in(tc, tc.Owner)
		path := "/api/v1/teams/" + teamID(support) + "/members/" + memberID(member)

		owner.Delete(path).AssertNoContent()

		require.Len(t, removed.all(), 1)
		assert.Equal(t, tc.Workspace.Owner.ID, removed.all()[0].RemovedBy)
		owner.Delete(path).AssertNotFound()
		owner.Delete("/api/v1/teams/" + teamID(support) + "/members/mem_nope").AssertNotFound()
		owner.Delete("/api/v1/teams/team_nope/members/" + memberID(member)).AssertNotFound()
	})

	t.Run("keeps members from taking others out", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support", member)

		in(tc, user).Delete("/api/v1/teams/" + teamID(support) + "/members/" + memberID(member)).AssertForbidden()
		tc.Table("team_members").AssertCount(1, testkit.Row{"team_id": support.ID})
	})
}

func TestTeamsAndDeactivatedMembers(t *testing.T) {
	t.Parallel()

	t.Run("takes a deactivated member out of every team and leaves them out once back", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support", member)
		billing := team(t, tc, "Billing", member)
		removed := record[appevents.TeamMemberRemoved](tc)
		owner := in(tc, tc.Owner)

		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).AssertOK()

		teams := []any{}
		for _, e := range removed.all() {
			assert.Equal(t, member.ID, e.MemberID)
			teams = append(teams, e.TeamID)
		}
		assert.ElementsMatch(t, []any{support.ID, billing.ID}, teams)

		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "active"}).AssertOK()
		tc.Table("team_members").AssertCount(0, testkit.Row{"workspace_member_id": member.ID})
	})
}

func TestMemberTeams(t *testing.T) {
	t.Parallel()

	t.Run("includes each member's live teams by name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		team(t, tc, "Support", member)
		billing := team(t, tc, "Billing", member)
		gone := team(t, tc, "Archive", member)
		client := in(tc, user)
		in(tc, tc.Owner).Delete("/api/v1/teams/" + teamID(gone)).AssertNoContent()

		client.GetJSON("/api/v1/members/me?include=teams").AssertOK().
			AssertJSONPath("data.teams.#", 2).
			AssertJSONPath("data.teams.0.id", teamID(billing)).
			AssertJSONPath("data.teams.0.object", "team").
			AssertJSONPath("data.teams.1.name", "Support")
		client.GetJSON("/api/v1/members/me").AssertOK().AssertJSONMissing("data.teams")
		client.GetJSON("/api/v1/members/me?include=owners").AssertStatus(400)

		res := client.GetJSON("/api/v1/members?include=teams&q="+tc.Owner.Email).AssertOK().
			AssertJSONPath("data.0.teams.#", 0)
		assert.Equal(t, memberID(tc.Workspace.Owner), res.String("data.0.id"))
		client.GetJSON("/api/v1/members?include=teams&filter[role]=member").AssertOK().
			AssertJSONPath("data.0.teams.#", 2)
	})
}
