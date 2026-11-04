package members_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
importPath is where member imports are posted.
*/
const importPath = "/api/v1/member-imports"

/*
upload posts the lines as an import file, with mode "true" for a dry
run and "false" for a real one.
*/
func upload(c *testkit.Client, mode string, lines ...string) *testkit.Response {
	file := testkit.FormFile{Field: "file", Name: "members.csv", Contents: []byte(strings.Join(lines, "\n"))}

	return c.PostForm(importPath, file, map[string]string{"dry_run": mode})
}

/*
The modes of an upload.
*/
const (
	dryRun = "true"
	apply  = "false"
)

/*
team creates a team of the test case's workspace with the members.
*/
func team(t *testing.T, tc *tests.TestCase, name string, of ...models.WorkspaceMember) models.Team {
	t.Helper()

	made, err := factories.Team(t.Context(), tc.App, factories.TeamOf{
		Workspace: tc.Workspace.Workspace, Name: name, Members: of,
	})
	require.NoError(t, err)

	return made
}

func TestMemberImport(t *testing.T) {
	t.Parallel()

	t.Run("changes roles, teams and status and invites new addresses", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, _ := join(t, tc, enums.RoleAdmin)
		grace, graceMember := join(t, tc, enums.RoleMember)
		bob, bobMember := join(t, tc, enums.RoleMember)
		calm, _ := join(t, tc, enums.RoleMember)
		dora, doraMember := join(t, tc, enums.RoleMember)
		team(t, tc, "Support", bobMember, doraMember)
		team(t, tc, "Billing")

		upload(in(tc, admin), apply,
			"\xef\xbb\xbfEmail,role,teams,status",
			grace.Email+",admin,Support;billing,",
			bob.Email+",,Billing,deactivated",
			calm.Email+",member,,active,extra",
			dora.Email+",,Billing;;,",
			"new@example.com,admin,Support,",
			"",
		).AssertOK().
			AssertJSONPath("data.object", "member_import").
			AssertJSONPath("data.dry_run", false).
			AssertJSONPath("data.created", 1).
			AssertJSONPath("data.updated", 3).
			AssertJSONPath("data.unchanged", 1).
			AssertJSONPath("data.failed", 0).
			AssertJSONPath("data.rows.0.row", 2).
			AssertJSONPath("data.rows.0.result", "updated").
			AssertJSONPath("data.rows.1.changes", []any{"status"}).
			AssertJSONPath("data.rows.2.result", "unchanged").
			AssertJSONPath("data.rows.0.changes", []any{"role", "teams"}).
			AssertJSONPath("data.rows.3.changes", []any{"teams"}).
			AssertJSONPath("data.rows.4.result", "created")

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": graceMember.ID, "role": "admin"})
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": bobMember.ID, "status": "deactivated"})
		tc.Table("team_members").AssertCount(2, testkit.Row{"workspace_member_id": graceMember.ID})
		tc.Table("team_members").AssertCount(0, testkit.Row{"workspace_member_id": bobMember.ID})
		tc.Table("invitations").AssertHas(testkit.Row{"email": "new@example.com", "role": "admin"})
		tc.Table("team_members").AssertCount(1, testkit.Row{"workspace_member_id": doraMember.ID})
	})

	t.Run("a dry run reports the same and changes nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		grace, graceMember := join(t, tc, enums.RoleMember)

		upload(in(tc, tc.Owner), dryRun, "email,role", grace.Email+",admin", "new@example.com,").AssertOK().
			AssertJSONPath("data.dry_run", true).
			AssertJSONPath("data.updated", 1).
			AssertJSONPath("data.created", 1)

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": graceMember.ID, "role": "member"})
		tc.Table("invitations").AssertCount(0, testkit.Row{"email": "new@example.com"})
	})

	t.Run("fails bad rows on their own and goes on", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		grace, _ := join(t, tc, enums.RoleMember)
		bob, _ := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)

		upload(owner, apply,
			"email,role,teams,custom_role,status",
			"not-an-address,,,,",
			grace.Email+",owner,,,",
			grace.Email+",admin,,,",
			bob.Email+",admin,,,",
			"x@example.com,boss,,,",
			"y@example.com,,,,gone",
			"z@example.com,,Nowhere,,",
			"w@example.com,,,Read only,",
			"v@example.com,,,,deactivated",
			tc.Owner.Email+",admin,,,",
		).AssertOK().
			AssertJSONPath("data.failed", 9).
			AssertJSONPath("data.updated", 1).
			AssertJSONPath("data.rows.0.error.code", "invalid_email").
			AssertJSONPath("data.rows.1.error.code", "role_not_allowed").
			AssertJSONPath("data.rows.2.error.code", "duplicate_email").
			AssertJSONPath("data.rows.3.result", "updated").
			AssertJSONPath("data.rows.4.error.code", "invalid_role").
			AssertJSONPath("data.rows.5.error.code", "invalid_status").
			AssertJSONPath("data.rows.6.error.code", "unknown_team").
			AssertJSONPath("data.rows.7.error.code", "custom_role_unavailable").
			AssertJSONPath("data.rows.8.error.code", "unknown_member").
			AssertJSONPath("data.rows.9.error.code", "owner_not_changed").
			AssertJSONPath("data.rows.9.error.message", "An owner's role and status cannot change through an import.")
	})

	t.Run("lets an owner's teams change", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		team(t, tc, "Support")

		upload(in(tc, tc.Owner), apply, "email,teams", tc.Owner.Email+",Support").AssertOK().
			AssertJSONPath("data.updated", 1)
		tc.Table("team_members").AssertCount(1, testkit.Row{"workspace_member_id": tc.Workspace.Owner.ID})
	})

	t.Run("refuses a file it cannot read as a whole", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		upload(owner, apply, "email,shade", "a@example.com,red").
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "unknown_column")
		upload(owner, apply, "role", "admin").
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "missing_email_column")
		upload(owner, apply).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "invalid_csv")
		upload(owner, apply, "email", `"unterminated`).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "invalid_csv")
		owner.PostForm(importPath, testkit.FormFile{Field: "other", Name: "a.csv"}, nil).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "invalid_csv")
	})

	t.Run("refuses more rows than one import takes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		lines := []string{"email"}

		for i := range 1001 {
			lines = append(lines, "p"+strings.Repeat("x", i%5)+"@example.com")
		}

		upload(in(tc, tc.Owner), dryRun, lines...).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "too_many_rows")
	})

	t.Run("is only for admins and owners, with a recent confirmation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)

		owner := in(tc, tc.Owner)

		upload(in(tc, member), dryRun, "email").AssertForbidden()

		tc.Clock.Travel(tc.App.Config.Auth.PasswordConfirmation.TTL + time.Second)
		upload(owner, dryRun, "email").
			AssertStatus(fhttp.StatusLocked).AssertJSONPath("code", "password_confirmation_required")
	})

	t.Run("allows five imports an hour", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		for range 5 {
			upload(owner, dryRun, "email").AssertOK()
		}

		upload(owner, dryRun, "email").
			AssertStatus(fhttp.StatusTooManyRequests).AssertJSONPath("code", "too_many_imports")
	})
}
