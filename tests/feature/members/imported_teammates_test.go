package members_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/importedteammates"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
teammate is someone of the other tool: their address, role and teams.
*/
type teammate struct {
	email string
	role  enums.Role
	teams []ids.UUID
}

/*
importTeammate brings the teammate into the test case's workspace.
*/
func importTeammate(t *testing.T, tc *tests.TestCase, who teammate) importedteammates.Imported {
	t.Helper()

	imported, err := tc.App.Actions.CreateImportedTeammate.Handle(t.Context(),
		importedteammates.CreateImportedTeammateInput{
			WorkspaceID: tc.Workspace.Workspace.ID, Email: who.email, Name: "Amara Okafor", Role: who.role,
			TeamIDs: who.teams, Source: "intercom",
		})
	require.NoError(t, err)

	return imported
}

/*
signUp registers amara@example.com with a password and returns the browser,
signed in, with the verification link it was mailed.
*/
func signUp(t *testing.T, tc *tests.TestCase) (*testkit.Client, appmail.VerifyEmailMail) {
	t.Helper()

	fake := tc.Mail()
	browser := tc.Browser(tc.Client().PostJSON("/api/v1/users", testkit.JSON{
		"name": "Amara O.", "email": "amara@example.com", "password": factories.Password, "terms_accepted": true,
	}).AssertCreated())
	sent, _ := tests.LastSent[appmail.VerifyEmailMail](tc, fake)

	return browser, sent
}

/*
verify follows the verification link in the browser.
*/
func verify(tc *tests.TestCase, browser *testkit.Client, sent appmail.VerifyEmailMail) {
	link := tc.Fragment(sent.URL)
	browser.PatchJSON("/api/v1/email-verifications/"+link.Get("id"), testkit.JSON{"token": link.Get("token")}).
		AssertOK()
}

/*
mailsTo returns the mailables of type M the fake sent to the address.
*/
func mailsTo[M mail.Mailable](tc *tests.TestCase, fake *mail.Fake, address string) []M {
	mailables, messages := tests.Sent[M](tc, fake)

	var out []M
	for i, m := range mailables {
		if slices.Contains(tests.SentTo(messages[i]), address) {
			out = append(out, m)
		}
	}

	return out
}

/*
column reads one column of one row, for values a table assertion cannot
match, such as a time that was set.
*/
func column[T any](t *testing.T, tc *tests.TestCase, query string, args ...any) T {
	t.Helper()

	var value T
	require.NoError(t, tc.Operator().QueryRow(t.Context(), query, args...).Scan(&value))

	return value
}

func TestImportedTeammates(t *testing.T) {
	t.Parallel()

	t.Run("creates a disabled member on a placeholder account nobody can sign in to", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		imported := importTeammate(t, tc, teammate{email: "amara@example.com", role: enums.RoleAdmin})

		assert.True(t, imported.Created)
		assert.True(t, imported.User.IsPlaceholder())
		tc.Table("users").AssertHas(testkit.Row{
			"email": "amara@example.com", "created_via": "import", "password_hash": nil, "email_verified_at": nil,
		})
		tc.Table("workspace_members").AssertHas(testkit.Row{
			"id": imported.Member.ID, "status": "deactivated", "joined_via": "import", "import_pending": true,
			"import_role": "admin", "import_source": "intercom",
		})
		tc.Client().PostJSON("/api/v1/sessions", login("amara@example.com", factories.Password)).
			AssertValidationError("email")
	})

	t.Run("returns an existing member as they are and reuses an existing account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		existing, err := models.New(tc.App.DB).FindUser(t.Context(), member.UserID)
		require.NoError(t, err)

		again := importTeammate(t, tc, teammate{email: existing.Email, role: enums.RoleAdmin})
		assert.False(t, again.Created)
		assert.Equal(t, member.ID, again.Member.ID)
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": member.ID, "status": "active", "role": "member"})

		elsewhere := tc.CreateUser()
		reused := importTeammate(t, tc, teammate{email: elsewhere.Email, role: enums.RoleMember})
		assert.Equal(t, elsewhere.ID, reused.User.ID)
		assert.False(t, reused.User.IsPlaceholder())
	})

	t.Run("an owner of the other tool is imported as an admin", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		imported := importTeammate(t, tc, teammate{email: "boss@example.com", role: enums.RoleOwner})

		assert.Equal(t, "admin", imported.Member.Role)
	})

	t.Run("signing up adopts the placeholder, grants no access and tells admins once proven", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		imported := importTeammate(t, tc, teammate{email: "amara@example.com", role: enums.RoleMember})
		fake := tc.Mail()

		browser, link := signUp(t, tc)

		tc.Table("users").AssertCount(1, testkit.Row{"email": "amara@example.com"})
		tc.Table("users").AssertHas(testkit.Row{"id": imported.User.ID, "name": "Amara O.", "email_verified_at": nil})
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": imported.Member.ID, "status": "deactivated"})
		workspace := models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
		browser.WithHeader("Dimmi-Workspace", workspace).GetJSON("/api/v1/conversations").
			AssertStatus(fhttp.StatusForbidden)
		me := browser.GetJSON("/api/v1/me").AssertOK()
		me.AssertJSONPath("data.workspaces", []any{})
		me.AssertJSONPath("data.awaiting_access.0.id", workspace)
		assert.Empty(t, mailsTo[appmail.ImportedTeammateWaitingMail](tc, fake, tc.Owner.Email))

		verify(tc, browser, link)

		waiting := mailsTo[appmail.ImportedTeammateWaitingMail](tc, fake, tc.Owner.Email)
		require.Len(t, waiting, 1)
		assert.Equal(t, "Amara O.", waiting[0].Teammate)
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": imported.Member.ID, "status": "deactivated"})
		in(tc, tc.Owner).GetJSON("/api/v1/members/"+memberID(imported.Member)).AssertOK().
			AssertJSONPath("data.awaiting_enable", true).AssertJSONPath("data.user.email_verified", true)
	})

	t.Run("signing up twice at a claimed address is still refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		importTeammate(t, tc, teammate{email: "amara@example.com", role: enums.RoleMember})
		signUp(t, tc)

		tc.Client().PostJSON("/api/v1/users", testkit.JSON{
			"name": "Eve", "email": "amara@example.com", "password": factories.Password, "terms_accepted": true,
		}).AssertValidationError("email")
	})

	t.Run("enabling someone who signed up applies the role and teams and mails them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		team, err := factories.Team(t.Context(), tc.App, factories.TeamOf{
			Workspace: tc.Workspace.Workspace, Name: "Support",
		})
		require.NoError(t, err)
		amara := teammate{email: "amara@example.com", role: enums.RoleAdmin, teams: []ids.UUID{team.ID}}
		imported := importTeammate(t, tc, amara)
		browser, link := signUp(t, tc)
		verify(tc, browser, link)
		fake := tc.Mail()

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(imported.Member), testkit.JSON{"status": "active"}).
			AssertOK().AssertJSONPath("data.status", "active").AssertJSONPath("data.role", "admin").
			AssertJSONPath("data.awaiting_enable", false)

		tc.Table("team_members").AssertHas(testkit.Row{"team_id": team.ID, "workspace_member_id": imported.Member.ID})
		enabled := mailsTo[appmail.AccessEnabledMail](tc, fake, "amara@example.com")
		require.Len(t, enabled, 1)
		workspace := models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
		browser.WithHeader("Dimmi-Workspace", workspace).GetJSON("/api/v1/conversations").AssertOK()
		browser.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.awaiting_access", []any{})
	})

	t.Run("enabling before sign-up sends an invitation that brings them in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		team, err := factories.Team(t.Context(), tc.App, factories.TeamOf{
			Workspace: tc.Workspace.Workspace, Name: "Billing",
		})
		require.NoError(t, err)
		amara := teammate{email: "amara@example.com", role: enums.RoleMember, teams: []ids.UUID{team.ID}}
		imported := importTeammate(t, tc, amara)
		fake := tc.Mail()

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(imported.Member), testkit.JSON{"status": "active"}).
			AssertOK().AssertJSONPath("data.status", "deactivated").AssertJSONPath("data.awaiting_enable", true)

		tc.Table("invitations").AssertHas(testkit.Row{"email": "amara@example.com", "status": "pending"})
		sent := mailsTo[appmail.InvitationMail](tc, fake, "amara@example.com")
		require.Len(t, sent, 1)
		token := tc.Fragment(sent[0].AcceptURL).Get("token")
		tc.Client().WithHeader("Dimmi-Invitation-Token", token).GetJSON("/api/v1/invitations/preview").AssertOK().
			AssertJSONPath("data.has_account", false)
		invited := in(tc, tc.Owner).GetJSON("/api/v1/members/" + memberID(imported.Member)).AssertOK().
			String("data.enable_invited_at")
		assert.NotEmpty(t, invited)

		tc.Client().PostJSON("/api/v1/users", testkit.JSON{
			"name": "Amara", "email": "amara@example.com", "password": factories.Password, "terms_accepted": true,
			"invitation_token": token,
		}).AssertCreated()

		tc.Table("users").AssertCount(1, testkit.Row{"email": "amara@example.com"})
		tc.Table("workspace_members").AssertHas(testkit.Row{
			"id": imported.Member.ID, "status": "active", "role": "member", "import_pending": false,
		})
		tc.Table("team_members").AssertHas(testkit.Row{"team_id": team.ID, "workspace_member_id": imported.Member.ID})
	})

	t.Run("a member cannot enable and the filter lists only who waits", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		imported := importTeammate(t, tc, teammate{email: "amara@example.com", role: enums.RoleMember})
		plain, _ := join(t, tc, enums.RoleMember)

		in(tc, plain).PatchJSON("/api/v1/members/"+memberID(imported.Member), testkit.JSON{"status": "active"}).
			AssertForbidden()
		owner := in(tc, tc.Owner)
		owner.GetJSON("/api/v1/members?filter[awaiting_enable]=true").AssertOK().
			AssertJSONPath("data.#", 1).AssertJSONPath("data.0.id", memberID(imported.Member)).
			AssertJSONPath("data.0.imported_from", "intercom")
		owner.GetJSON("/api/v1/members?filter[awaiting_enable]=false").AssertOK().
			AssertJSONMissing(`data.#(id=="` + memberID(imported.Member) + `")`)
		owner.GetJSON("/api/v1/members?filter[awaiting_enable]=maybe").AssertStatus(fhttp.StatusBadRequest)
	})

	t.Run("an adopted placeholder can still delete its account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		imported := importTeammate(t, tc, teammate{email: "amara@example.com", role: enums.RoleMember})
		signUp(t, tc)
		adopted, err := models.New(tc.App.DB).FindUser(t.Context(), imported.User.ID)
		require.NoError(t, err)

		confirm := testkit.JSON{"confirm_email": "amara@example.com"}
		tc.ActingAs(adopted).Send(fhttp.MethodDelete, "/api/v1/user", confirm).AssertNoContent()

		deleted := column[*time.Time](t, tc, `SELECT deleted_at FROM users WHERE id = $1`, imported.User.ID)
		assert.NotNil(t, deleted)
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": imported.Member.ID, "status": "deactivated"})
	})

	t.Run("a token of a member who waits works nowhere", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		imported := importTeammate(t, tc, teammate{email: "amara@example.com", role: enums.RoleAdmin})

		tc.ActingAsToken(imported.Member, "conversations:read", "members:read").GetJSON("/api/v1/conversations").
			AssertUnauthorized().AssertJSONPath("code", "invalid_token")
		tc.ActingAsToken(imported.Member, "members:read").GetJSON("/api/v1/members").AssertUnauthorized()
	})

	t.Run("an import never reaches another workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tc.CreateUser()
		created, err := factories.Workspace(t.Context(), tc.App, other)
		require.NoError(t, err)
		imported := importTeammate(t, tc, teammate{email: "amara@example.com", role: enums.RoleMember})

		tc.ActingAs(other).WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, created.Workspace.ID)).
			GetJSON("/api/v1/members/" + memberID(imported.Member)).AssertNotFound()
		tc.ActingAs(other).WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, created.Workspace.ID)).
			PatchJSON("/api/v1/members/"+memberID(imported.Member), testkit.JSON{"status": "active"}).AssertNotFound()
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": imported.Member.ID, "status": "deactivated"})
	})
}
