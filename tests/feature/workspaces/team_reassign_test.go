package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
teamConversations opens one conversation per status on the team, held
by the member when one is given, and returns their ids.
*/
func teamConversations(t *testing.T, tc *tests.TestCase, team models.Team, holder *models.WorkspaceMember) []string {
	t.Helper()

	owner := in(tc, tc.Owner)
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	changes := []testkit.JSON{
		{"status": "open"},
		{"status": "snoozed", "snoozed_until": tests.Epoch().Add(48 * time.Hour)},
		{"status": "closed"},
	}

	out := make([]string, 0, len(changes))
	for _, change := range changes {
		id := owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": models.PublicID(models.PrefixInbox, inbox.Inbox.ID), "author": "contact",
			"body": "Hello", "contact": testkit.JSON{"email": "emily@example.com"},
		}).AssertCreated().String("data.id")

		change["team_id"] = teamID(team)
		if holder != nil {
			change["assignee_id"] = memberID(*holder)
		}

		owner.PatchJSON("/api/v1/conversations/"+id, change).AssertOK()
		out = append(out, id)
	}

	return out
}

func TestTeamsDestroyWithReassign(t *testing.T) {
	t.Parallel()

	t.Run("shows how many conversations a team holds before deleting it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		support := team(t, tc, "Support")
		teamConversations(t, tc, support, nil)
		member, _ := join(t, tc, enums.RoleMember)

		in(tc, tc.Owner).GetJSON("/api/v1/teams/"+teamID(support)+"?include=conversation_counts").AssertOK().
			AssertJSONPath("data.conversation_counts.open", 1).
			AssertJSONPath("data.conversation_counts.snoozed", 1).
			AssertJSONPath("data.conversation_counts.closed", 1)
		in(tc, tc.Owner).GetJSON("/api/v1/teams/" + teamID(support)).AssertOK().
			AssertJSONMissing("data.conversation_counts")
		in(tc, member).GetJSON("/api/v1/teams/" + teamID(support) + "?include=conversation_counts").AssertForbidden()
	})

	t.Run("deleting a team with a target moves open, snoozed and closed conversations", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, holder := join(t, tc, enums.RoleMember)
		support, billing := team(t, tc, "Support", holder), team(t, tc, "Billing")
		moved := teamConversations(t, tc, support, &holder)
		inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
		require.NoError(t, err)
		exec(t, tc, `UPDATE inboxes SET default_team_id = $1 WHERE id = $2`, support.ID, inbox.Inbox.ID)
		owner := in(tc, tc.Owner)

		owner.Delete("/api/v1/teams/" + teamID(support) + "?reassign_to_team_id=" + teamID(billing)).AssertNoContent()

		for _, id := range moved {
			owner.GetJSON("/api/v1/conversations/"+id).AssertOK().
				AssertJSONPath("data.team.id", teamID(billing)).
				AssertJSONPath("data.assignee.id", memberID(holder))
		}
		tc.Table("inboxes").AssertHas(testkit.Row{"id": inbox.Inbox.ID, "default_team_id": billing.ID})
	})

	t.Run("deleting a team toward a teammate hands them the conversations without a team", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, taker := join(t, tc, enums.RoleMember)
		support := team(t, tc, "Support")
		moved := teamConversations(t, tc, support, nil)
		owner := in(tc, tc.Owner)

		owner.Delete("/api/v1/teams/" + teamID(support) + "?reassign_to_member_id=" + memberID(taker)).
			AssertNoContent()

		for _, id := range moved {
			owner.GetJSON("/api/v1/conversations/"+id).AssertOK().
				AssertJSONPath("data.team", nil).
				AssertJSONPath("data.assignee.id", memberID(taker))
		}
	})

	t.Run("deleting a team with both targets is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, taker := join(t, tc, enums.RoleMember)
		_, gone := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, gone))
		support, billing := team(t, tc, "Support"), team(t, tc, "Billing")
		other := tests.New(t)
		foreign := team(t, other, "Elsewhere")
		owner := in(tc, tc.Owner)
		path := "/api/v1/teams/" + teamID(support)

		for _, query := range []string{
			"?reassign_to_team_id=" + teamID(billing) + "&reassign_to_member_id=" + memberID(taker),
			"?reassign_to_team_id=" + teamID(support),
			"?reassign_to_team_id=" + teamID(foreign),
			"?reassign_to_team_id=team_nope",
			"?reassign_to_member_id=" + memberID(gone),
			"?reassign_to_member_id=" + memberID(other.Workspace.Owner),
		} {
			owner.Delete(path + query).AssertValidationError("reassign_to")
		}

		owner.GetJSON(path).AssertOK()
	})
}
