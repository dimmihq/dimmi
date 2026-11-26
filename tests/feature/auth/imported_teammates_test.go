package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/importedteammates"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
imported brings amara@example.com, a teammate of another tool, into the
test case's workspace.
*/
func imported(t *testing.T, tc *tests.TestCase) importedteammates.Imported {
	t.Helper()

	out, err := tc.App.Actions.CreateImportedTeammate.Handle(t.Context(), importedteammates.CreateImportedTeammateInput{
		WorkspaceID: tc.Workspace.Workspace.ID, Email: "amara@example.com", Name: "Amara Okafor",
		Role: enums.RoleMember, Source: "intercom",
	})
	require.NoError(t, err)

	return out
}

/*
assertWaits checks the person is signed in with no workspace, waits in
the test case's workspace, and is still disabled there.
*/
func assertWaits(t *testing.T, tc *tests.TestCase, browser *testkit.Client, member models.WorkspaceMember) {
	t.Helper()

	workspace := models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
	me := browser.GetJSON("/api/v1/me").AssertOK()
	me.AssertJSONPath("data.workspaces", []any{})
	me.AssertJSONPath("data.awaiting_access.0.id", workspace)
	me.AssertJSONPath("data.awaiting_access.0.name", tc.Workspace.Workspace.Name)
	browser.WithHeader("Dimmi-Workspace", workspace).GetJSON("/api/v1/workspace").
		AssertForbidden().AssertJSONPath("code", "membership_deactivated")
	tc.Table("workspace_members").AssertHas(testkit.Row{
		"id": member.ID, "status": "deactivated", "import_pending": true,
	})
}

func TestImportedTeammateSignIn(t *testing.T) {
	t.Parallel()

	t.Run("a magic link claims the placeholder and keeps the membership disabled", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate := imported(t, tc)
		fake := tc.Mail()

		sent := magicLink(t, tc, "amara@example.com", nil)
		res := withLink(tc.Client(), sent, tc).AssertCreated()

		assertWaits(t, tc, tc.Browser(res), teammate.Member)
		tc.Table("users").AssertCount(1, testkit.Row{"email": "amara@example.com"})
		waiting, _ := tests.Sent[appmail.ImportedTeammateWaitingMail](tc, fake)
		require.Len(t, waiting, 1)
	})

	t.Run("a provider claims the placeholder, records the terms and keeps it disabled", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		teammate := imported(t, tc)
		tc.Social()
		browser := tc.Client().WithHeader("User-Agent", browserAgent)
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?terms_accepted=true")

		res := tc.FinishOAuth(browser, attempt, person("583231", "amara@example.com"))

		assertLands(t, res, "/")
		assertWaits(t, tc, tc.Browser(res), teammate.Member)
		tc.Table("oauth_identities").AssertHas(testkit.Row{"user_id": teammate.User.ID, "provider": "github"})
		assertAccepted(t, tc, userByEmail(t, tc, "amara@example.com"), "1.0")
	})

	t.Run("a provider claim without the published terms is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv(publishedTerms))
		teammate := imported(t, tc)
		tc.Social()
		browser := tc.Client()
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?terms_accepted=false")

		assertLands(t, tc.FinishOAuth(browser, attempt, person("583231", "amara@example.com")),
			"/login?error=terms_not_accepted")

		tc.Table("users").AssertHas(testkit.Row{"id": teammate.User.ID, "email_verified_at": nil})
	})

	t.Run("a placeholder nobody claimed has no password to sign in with", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		imported(t, tc)

		tc.Client().PostJSON("/api/v1/sessions", login("amara@example.com", "")).AssertValidationError()
		tc.Client().PostJSON("/api/v1/sessions", login("amara@example.com", "anything-at-all")).
			AssertValidationError("email")
		assert.False(t, userByEmail(t, tc, "amara@example.com").IsVerified())
	})
}
