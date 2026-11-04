package workspaces_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
exportMembers asks the client for the member list as CSV, narrowed by
the query, and returns the answer.
*/
func exportMembers(c *testkit.Client, query string) *testkit.Response {
	return c.WithHeader("Accept", "text/csv").Send("GET", "/api/v1/members?"+query, nil)
}

/*
memberCell returns the value of the named column in a row of an
exported member list.
*/
func memberCell(row []string, name string) string {
	i := slices.Index(members.MemberExportColumns(), name)
	if i < 0 {
		return ""
	}

	return row[i]
}

/*
listed returns the strings at the path, such as every listed id, in
order.
*/
func listed(t *testing.T, res *testkit.Response, path string) []string {
	t.Helper()

	var out []string
	require.NoError(t, json.Unmarshal([]byte(res.String(path)), &out))

	return out
}

/*
touch records that the member used the workspace at the moment.
*/
func touch(t *testing.T, tc *tests.TestCase, m models.WorkspaceMember, at time.Time) {
	t.Helper()

	exec(t, tc, "UPDATE workspace_members SET last_accessed_at = $1 WHERE id = $2", at, m.ID)
}

/*
teamWith creates a team of the test case's workspace with the members.
*/
func teamWith(t *testing.T, tc *tests.TestCase, name string, of ...models.WorkspaceMember) models.Team {
	t.Helper()

	team, err := factories.Team(t.Context(), tc.App, factories.TeamOf{
		Workspace: tc.Workspace.Workspace, Name: name, Members: of,
	})
	require.NoError(t, err)

	return team
}

func TestMemberReview(t *testing.T) {
	t.Parallel()

	t.Run("shows managers who has a second factor and when each was last active", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		_, err := factories.EnableTwoFactor(t.Context(), tc.App, user)
		require.NoError(t, err)
		touch(t, tc, member, tc.Clock.Now())

		_, never := join(t, tc, enums.RoleMember)

		res := in(tc, tc.Owner).GetJSON("/api/v1/members/"+memberID(member)).AssertOK().
			AssertJSONPath("data.two_factor_enabled", true)
		at, err := time.Parse(time.RFC3339, res.String("data.last_active_at"))
		require.NoError(t, err)
		assert.True(t, at.Equal(tc.Clock.Now()))
		in(tc, tc.Owner).GetJSON("/api/v1/members/"+memberID(never)).AssertOK().
			AssertJSONPath("data.two_factor_enabled", false).
			AssertJSONPath("data.last_active_at", nil)
	})

	t.Run("hides both from members who do not manage members", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)
		team := teamWith(t, tc, "Support", tc.Workspace.Owner)

		in(tc, user).GetJSON("/api/v1/members").AssertOK().
			AssertJSONPath("data.0.two_factor_enabled", nil).
			AssertJSONPath("data.0.last_active_at", nil).
			AssertJSONPath("data.0.sign_in_locked", nil)
		teamPath := "/api/v1/teams/" + models.PublicID(models.PrefixTeam, team.ID) + "?include=members"
		in(tc, tc.Owner).GetJSON(teamPath).AssertOK().
			AssertJSONPath("data.members.0.two_factor_enabled", nil)
	})

	t.Run("narrows to members without a second factor, for managers only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		protected, _ := join(t, tc, enums.RoleAdmin)
		_, err := factories.EnableTwoFactor(t.Context(), tc.App, protected)
		require.NoError(t, err)

		res := in(tc, tc.Owner).GetJSON("/api/v1/members?filter[two_factor]=false").AssertOK().
			AssertJSONPath("data.#", 2)
		assert.ElementsMatch(t, []string{memberID(tc.Workspace.Owner), memberID(member)}, listed(t, res, "data.#.id"))
		in(tc, tc.Owner).GetJSON("/api/v1/members?filter[two_factor]=true").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.user.email", protected.Email)

		in(tc, tc.Owner).GetJSON("/api/v1/members?filter[two_factor]=maybe").AssertStatus(400)
		in(tc, tc.Owner).GetJSON("/api/v1/members?filter[two_factor]=true,false").AssertStatus(400)
		in(tc, user).GetJSON("/api/v1/members?filter[two_factor]=false").AssertForbidden()
	})

	t.Run("narrows to a team and orders by last activity", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, early := join(t, tc, enums.RoleMember)
		_, late := join(t, tc, enums.RoleMember)
		_, never := join(t, tc, enums.RoleMember)
		team := teamWith(t, tc, "Support", early, late)
		touch(t, tc, early, tc.Clock.Now().Add(-time.Hour))
		touch(t, tc, late, tc.Clock.Now().Add(-time.Minute))
		owner := in(tc, tc.Owner)

		res := owner.GetJSON("/api/v1/members?filter[team]=" + models.PublicID(models.PrefixTeam, team.ID)).AssertOK()
		assert.ElementsMatch(t, []string{memberID(early), memberID(late)}, listed(t, res, "data.#.id"))

		res = owner.GetJSON("/api/v1/members?sort=-last_active_at").AssertOK()
		assert.Equal(t, []string{memberID(tc.Workspace.Owner), memberID(late), memberID(early), memberID(never)},
			listed(t, res, "data.#.id"))
		owner.GetJSON("/api/v1/members?sort=last_active_at").AssertOK().AssertJSONPath("data.#", 4)
	})
}

func TestMemberExport(t *testing.T) {
	t.Parallel()

	t.Run("an admin downloads the members with teams and second factors", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, adminMember := join(t, tc, enums.RoleAdmin)
		user, member := join(t, tc, enums.RoleMember, func(u models.User) models.User {
			u.Name = "=HYPERLINK(\"x\")"

			return u
		})
		_, err := factories.EnableTwoFactor(t.Context(), tc.App, user)
		require.NoError(t, err)
		teamWith(t, tc, "Support", member, adminMember)
		teamWith(t, tc, "Billing", member)
		touch(t, tc, member, tc.Clock.Now())

		res := exportMembers(in(tc, admin), "").AssertOK().
			AssertHeader("Content-Type", "text/csv; charset=utf-8").
			AssertHeader("Content-Disposition",
				`attachment; filename="dimmi-members-`+tc.Workspace.Workspace.Slug+`.csv"`).
			AssertHeader("Cache-Control", "no-store")

		rows, err := csv.NewReader(bytes.NewReader(res.Body)).ReadAll()
		require.NoError(t, err)
		require.Len(t, rows, 4)
		assert.Equal(t, members.MemberExportColumns(), rows[0])

		i := slices.IndexFunc(rows, func(row []string) bool { return memberCell(row, "email") == user.Email })
		require.Positive(t, i)
		row := rows[i]
		assert.Equal(t, `'=HYPERLINK("x")`, memberCell(row, "name"))
		assert.Equal(t, "member", memberCell(row, "role"))
		assert.Equal(t, "Billing;Support", memberCell(row, "teams"))
		assert.Equal(t, "true", memberCell(row, "two_factor"))
		assert.Equal(t, tc.Clock.Now().UTC().Format(time.RFC3339), memberCell(row, "last_active_at"))
		assert.Equal(t, "active", memberCell(row, "status"))

		tc.Table("audit_logs").AssertHas(testkit.Row{"event": members.EventMembersExported})
	})

	t.Run("follows the list filters and is refused to plain members", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)
		join(t, tc, enums.RoleAdmin)

		res := exportMembers(in(tc, tc.Owner), "filter[role]=admin").AssertOK()
		rows, err := csv.NewReader(bytes.NewReader(res.Body)).ReadAll()
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Equal(t, "admin", memberCell(rows[1], "role"))

		exportMembers(in(tc, user), "").AssertForbidden()
		exportMembers(in(tc, tc.Owner), "filter[status]=gone").AssertStatus(400)
		exportMembers(in(tc, tc.Owner), "filter[bogus]=1").AssertStatus(400)
		tc.Table("audit_logs").AssertCount(1, testkit.Row{"event": members.EventMembersExported})
	})
}
