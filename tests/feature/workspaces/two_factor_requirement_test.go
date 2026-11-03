package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
requireTwoFactor makes the workspace require a second factor, as an
admin turning it on would leave it.
*/
func requireTwoFactor(t *testing.T, tc *tests.TestCase, workspace models.Workspace) {
	t.Helper()

	exec(t, tc, `UPDATE workspaces SET settings = jsonb_set(settings, '{security}', '{"require_two_factor": true}')
		WHERE id = $1`, workspace.ID)
}

/*
protect turns two-factor authentication on for the user, the way
finishing the setup leaves the account.
*/
func protect(t *testing.T, tc *tests.TestCase, user models.User) {
	t.Helper()

	_, err := factories.WithTwoFactor(t.Context(), tc.App, user)
	require.NoError(t, err)
}

/*
assertTwoFactorRequired checks the refusal of a caller without a second
factor in a workspace that requires one.
*/
func assertTwoFactorRequired(t *testing.T, res *testkit.Response) {
	t.Helper()

	res.AssertForbidden().
		AssertJSONPath("code", "two_factor_required").
		AssertJSONPath("requirement", "two_factor").
		AssertJSONPath("action_url", "/d/settings/security")
	assert.NotEmpty(t, res.String("message"))
}

func TestWorkspaceTwoFactorRequirement(t *testing.T) {
	t.Parallel()

	t.Run("lets an owner with two-factor require it and mails the members without it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		updated := record[appevents.WorkspaceUpdated](tc)
		owner := in(tc, tc.Owner)
		protect(t, tc, tc.Owner)
		unprotected, _ := join(t, tc, enums.RoleMember)
		protected, _ := join(t, tc, enums.RoleAdmin)
		protect(t, tc, protected)

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"require_two_factor": true}).AssertOK().
			AssertJSONPath("data.require_two_factor", true)

		owner.GetJSON("/api/v1/workspace").AssertOK().AssertJSONPath("data.require_two_factor", true)
		require.Len(t, updated.all(), 1)
		assert.Equal(t, []string{"require_two_factor"}, updated.all()[0].Changes)
		mails, sent := tests.Sent[appmail.TwoFactorRequiredMail](tc, fake)
		require.Len(t, mails, 1)
		assert.Equal(t, []string{unprotected.Email}, tests.SentTo(sent[0]))
		assert.Equal(t, tc.Workspace.Workspace.Name, mails[0].Workspace)
		assert.Contains(t, mails[0].SecurityURL, "/d/settings/security")
	})

	t.Run("turns the requirement off again without mailing anyone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		owner := in(tc, tc.Owner)
		protect(t, tc, tc.Owner)
		join(t, tc, enums.RoleMember)
		requireTwoFactor(t, tc, tc.Workspace.Workspace)

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"require_two_factor": false}).AssertOK().
			AssertJSONPath("data.require_two_factor", false)

		mails, _ := tests.Sent[appmail.TwoFactorRequiredMail](tc, fake)
		assert.Empty(t, mails)
	})

	t.Run("refuses an owner without two-factor of their own", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		in(tc, tc.Owner).PatchJSON("/api/v1/workspace", testkit.JSON{"require_two_factor": true}).
			AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "two_factor_not_enabled")
		in(tc, tc.Owner).GetJSON("/api/v1/workspace").AssertOK().AssertJSONPath("data.require_two_factor", false)
	})

	t.Run("refuses access tokens, a stale confirmation and members without settings:write", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		protect(t, tc, tc.Owner)
		token := tc.ActingAsToken(tc.Workspace.Owner, "settings:write")
		member, _ := join(t, tc, enums.RoleMember)
		memberBrowser := in(tc, member)
		protect(t, tc, member)

		token.PatchJSON("/api/v1/workspace", testkit.JSON{"require_two_factor": true}).
			AssertForbidden().AssertJSONPath("code", "first_party_only")
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Renamed"}).AssertOK()
		memberBrowser.PatchJSON("/api/v1/workspace", testkit.JSON{"require_two_factor": true}).
			AssertForbidden()

		tc.Clock.Advance(tc.App.Config.Auth.PasswordConfirmation.TTL + time.Second)
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"require_two_factor": true}).
			AssertStatus(fhttp.StatusLocked).
			AssertJSONPath("code", "password_confirmation_required")
	})
}

func TestAccessPolicy(t *testing.T) {
	t.Parallel()

	t.Run("refuses every kind of caller without two-factor in a workspace that requires it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleAdmin)
		browser := in(tc, user)
		token := tc.ActingAsToken(member, "members:read")
		requireTwoFactor(t, tc, tc.Workspace.Workspace)

		assertTwoFactorRequired(t, browser.GetJSON("/api/v1/workspace"))
		assertTwoFactorRequired(t, browser.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine"}))
		assertTwoFactorRequired(t, token.GetJSON("/api/v1/members"))
		assertTwoFactorRequired(t, token.GetJSON("/api/v1/workspace"))
	})

	t.Run("lets the member set up a second factor or leave, and lets them in right after", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		browser := in(tc, user)
		requireTwoFactor(t, tc, tc.Workspace.Workspace)

		browser.GetJSON("/api/v1/me").AssertOK()
		browser.GetJSON("/api/v1/user/auth-factors").AssertOK()
		browser.GetJSON("/api/v1/sessions").AssertOK()
		browser.GetJSON("/api/v1/user/memberships").AssertOK()

		protect(t, tc, user)

		browser.GetJSON("/api/v1/workspace").AssertOK()
		browser.Delete("/api/v1/user/memberships/" + memberID(member)).AssertNoContent()
	})

	t.Run("lets members through in a workspace that requires nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		other, err := factories.Workspace(t.Context(), tc.App, user)
		require.NoError(t, err)
		_, err = factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)
		requireTwoFactor(t, tc, tc.Workspace.Workspace)
		browser := tc.ActingAs(user)

		browser.WithHeader("Dimmi-Workspace", publicWorkspace(other)).GetJSON("/api/v1/workspace").AssertOK()
		assertTwoFactorRequired(t, browser.WithHeader("Dimmi-Workspace", workspaceID(tc)).GetJSON("/api/v1/workspace"))
	})
}

func TestTwoFactorRemovalUnderRequirement(t *testing.T) {
	t.Parallel()

	t.Run("refuses turning two-factor off while a workspace requires it, naming the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		account, err := factories.WithTwoFactor(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		requireTwoFactor(t, tc, tc.Workspace.Workspace)

		totp := models.PublicID(models.PrefixAuthFactor, account.TOTP.ID)

		res := browser.Delete("/api/v1/user/auth-factors/"+totp).
			AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "two_factor_required_by_workspace").
			AssertJSONPath("workspaces.0.id", workspaceID(tc)).
			AssertJSONPath("workspaces.0.name", tc.Workspace.Workspace.Name)
		assert.NotEmpty(t, res.String("message"))
		browser.Delete("/api/v1/user/auth-factors").AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "two_factor_required_by_workspace")

		tc.Table("user_auth_factors").AssertCount(2, testkit.Row{"user_id": tc.Owner.ID})
		tc.Table("security_events").AssertCount(0, testkit.Row{"type": "auth_factor.removed"})
	})

	t.Run("lets an operator override the requirement, after which the member is kept out", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)
		browser := in(tc, user)
		protect(t, tc, user)
		requireTwoFactor(t, tc, tc.Workspace.Workspace)
		browser.GetJSON("/api/v1/workspace").AssertOK()

		code, out := tc.Command("user:disable-2fa", user.Email)

		assert.Equal(t, 0, code, out)
		tc.Table("user_auth_factors").AssertCount(0, testkit.Row{"user_id": user.ID})
		assertTwoFactorRequired(t, browser.GetJSON("/api/v1/workspace"))
	})

	t.Run("allows turning two-factor off once the user left the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		browser := in(tc, user)
		protect(t, tc, user)
		requireTwoFactor(t, tc, tc.Workspace.Workspace)
		browser.Delete("/api/v1/user/auth-factors").AssertStatus(fhttp.StatusConflict)

		browser.Delete("/api/v1/user/memberships/" + memberID(member)).AssertNoContent()

		browser.Delete("/api/v1/user/auth-factors").AssertNoContent()
	})
}
