package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
The address a stranger registers before its owner arrives, and the
provider account the stranger links to it.
*/
const (
	victim          = "grace@example.com"
	strangerSubject = "stranger-at-google"
	strangerEmail   = "stranger@example.net"
)

/*
stranger is everything someone who registered another person's address
holds: a signed-in browser, an access token, a linked provider, an
authenticator's secret, and a second-factor challenge they passed the
password for and can still answer.
*/
type stranger struct {
	browser      *testkit.Client
	token        string
	secret       string
	challenge    *testkit.Client
	verification [2]string
	user         models.User
}

/*
registerAsStranger signs the address up and arms the account with every
way back in a stranger could prepare.
*/
func registerAsStranger(t *testing.T, tc *tests.TestCase) stranger {
	t.Helper()

	browser, id, token := verificationLink(t, tc)
	s := stranger{browser: browser, verification: [2]string{id, token}, user: userByEmail(t, tc, victim)}

	_, s.token = tc.MintToken(membershipOf(t, tc, s.user), "*")

	tc.Social()
	attempt := tc.StartOAuth(browser, "/auth/google/redirect?intent=link")
	tc.FinishOAuth(browser, attempt, person(strangerSubject, strangerEmail))
	tc.Table("oauth_identities").AssertCount(1, testkit.Row{"user_id": s.user.ID})

	added := browser.PostJSON("/api/v1/user/auth-factors", testkit.JSON{"type": "totp"}).AssertCreated()
	s.secret = added.String("data.setup.secret")
	code, err := twofactor.Code(s.secret, tc.Clock.Now())
	require.NoError(t, err)
	activated := browser.PatchJSON("/api/v1/user/auth-factors/"+added.String("data.id"), testkit.JSON{"code": code})
	s.browser = tc.Client().WithCookie(activated.AssertOK().Cookie(tc.App.Session.CookieName()))
	s.browser.GetJSON("/api/v1/me").AssertOK()

	s.challenge, _ = challenge(t, tc, victim)

	return s
}

/*
assertNothingLeft proves every way the stranger prepared is shut, and
that none of them reaches the owner's account.
*/
func assertNothingLeft(t *testing.T, tc *tests.TestCase, s stranger) {
	t.Helper()

	s.browser.GetJSON("/api/v1/me").AssertUnauthorized()
	tc.Client().WithHeader("Authorization", "Bearer "+s.token).GetJSON("/api/v1/me").AssertUnauthorized()
	tc.Client().PostJSON("/api/v1/sessions", login(victim, factories.Password)).
		AssertJSONPath("code", "invalid_credentials")

	code, err := twofactor.Code(s.secret, tc.Clock.Now())
	require.NoError(t, err)
	assert.NotEqual(t, 201, answer(s.challenge, "totp", code).Status)

	tc.Table("oauth_identities").AssertCount(0, testkit.Row{"provider_user_id": strangerSubject})
	tc.Table("user_auth_factors").AssertCount(0, testkit.Row{"user_id": s.user.ID})

	before := sessionsOf(t, tc, s.user)
	res := tc.SignInWith("google", person(strangerSubject, strangerEmail))
	tc.Browser(res).GetJSON("/api/v1/me").AssertJSONPath("data.user.email", strangerEmail)
	assert.Equal(t, before, sessionsOf(t, tc, s.user))

	workspace, err := models.New(tc.App.DB).ListUserWorkspaces(t.Context(), s.user.ID)
	require.NoError(t, err)
	assert.Empty(t, workspace)
}

/*
sessionsOf counts the sessions of the user.
*/
func sessionsOf(t *testing.T, tc *tests.TestCase, user models.User) int {
	t.Helper()

	sessions, err := tc.App.Session.Sessions(t.Context(), user.ID)
	require.NoError(t, err)

	return len(sessions)
}

func TestAccountClaim(t *testing.T) {
	t.Parallel()

	for _, path := range []struct {
		name  string
		prove func(t *testing.T, tc *tests.TestCase, s stranger) *testkit.Response
	}{
		{"a magic link", func(t *testing.T, tc *tests.TestCase, _ stranger) *testkit.Response {
			t.Helper()

			return withLink(tc.Client(), magicLink(t, tc, victim, nil), tc).AssertCreated()
		}},
		{"a mailed code", func(t *testing.T, tc *tests.TestCase, _ stranger) *testkit.Response {
			t.Helper()

			return withCode(tc.Client(), victim, magicLink(t, tc, victim, nil).Code).AssertCreated()
		}},
		{"a password reset", func(t *testing.T, tc *tests.TestCase, _ stranger) *testkit.Response {
			t.Helper()
			id, token := resetLink(t, tc, victim)

			return redeem(tc.Client(), id, token).AssertOK()
		}},
		{"the verification mail", func(_ *testing.T, tc *tests.TestCase, s stranger) *testkit.Response {
			return disown(tc, s.verification[0], s.verification[1]).AssertOK()
		}},
	} {
		t.Run("leaves a stranger nothing once the owner proves the inbox by "+path.name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t, withProviders())
			s := registerAsStranger(t, tc)

			path.prove(t, tc, s)

			assertNothingLeft(t, tc, s)
			user := userByEmail(t, tc, victim)
			assert.True(t, user.IsVerified())
			assert.False(t, user.TwoFactorEnabled())
		})
	}

	t.Run("leaves a stranger nothing once a provider vouches for the owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		s := registerAsStranger(t, tc)

		res := tc.SignInWith("google", person("the-real-owner", victim))

		tc.Browser(res).GetJSON("/api/v1/me").AssertJSONPath("data.user.email", victim)
		assertNothingLeft(t, tc, s)
		tc.Table("oauth_identities").AssertCount(1, testkit.Row{"provider_user_id": "the-real-owner"})
	})

	t.Run("signs the owner in without the stranger's second factor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		registerAsStranger(t, tc)

		withLink(tc.Client(), magicLink(t, tc, victim, nil), tc).AssertCreated().
			AssertJSONPath("data.user.email_verified", true).
			AssertJSONPath("data.user.two_factor_enabled", false)
	})

	t.Run("gives the owner a password only they know after a reset", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		registerAsStranger(t, tc)
		fake := tc.Mail()
		id, token := resetLink(t, tc, victim)

		redeem(tc.Client(), id, token).AssertOK()

		tc.Client().PostJSON("/api/v1/sessions", login(victim, factories.Password)).
			AssertJSONPath("code", "invalid_credentials")
		mail, _ := tests.LastSent[appmail.PasswordChangedMail](tc, fake)
		assert.Equal(t, appmail.PasswordWasReset, mail.Reason)
	})

	t.Run("moves a workspace only the stranger owned to the trash, where the owner can restore it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)
		user := userByEmail(t, tc, victim)
		member := membershipOf(t, tc, user)
		fake := tc.Mail()

		disown(tc, id, token).AssertOK()

		tc.Table("workspaces").AssertMissing(testkit.Row{"id": member.WorkspaceID, "deleted_at": nil})
		_, sent := tests.LastSent[appmail.WorkspaceDeletionScheduledMail](tc, fake)
		assert.Contains(t, sent.Message.Subject, "Navy")

		restored := withLink(tc.Client(), magicLink(t, tc, victim, nil), tc).AssertCreated()
		rows := tc.Browser(restored).GetJSON("/api/v1/user/memberships?filter[trashed]=only").AssertOK()
		assert.Equal(t, "Navy", rows.String("data.0.workspace.name"))
		workspace := rows.String("data.0.workspace.id")
		tc.Browser(restored).WithHeader("Dimmi-Workspace", workspace).
			PatchJSON("/api/v1/workspaces/"+workspace, testkit.JSON{"trashed": false}).AssertOK()
		tc.Table("workspaces").AssertHas(testkit.Row{"id": member.WorkspaceID, "deleted_at": nil})
	})

	t.Run("leaves a workspace another owner holds, without touching it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)
		user := userByEmail(t, tc, victim)
		member := membershipOf(t, tc, user)
		_, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: models.Workspace{ID: member.WorkspaceID}, User: tc.CreateUser(), Role: enums.RoleOwner,
		})
		require.NoError(t, err)

		disown(tc, id, token).AssertOK()

		tc.Table("workspaces").AssertHas(testkit.Row{"id": member.WorkspaceID, "deleted_at": nil})
		rows, err := models.New(tc.App.DB).ListUserWorkspaces(t.Context(), user.ID)
		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("claims nothing when the browser proving the inbox holds the account's own session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		s := registerAsStranger(t, tc)
		fake := tc.Mail()

		verifyIn(s.browser, s.verification[0], s.verification[1]).AssertOK()

		s.browser.GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.has_password", true).
			AssertJSONPath("data.user.two_factor_enabled", true)
		tc.Client().WithHeader("Authorization", "Bearer "+s.token).GetJSON("/api/v1/me").AssertOK()
		tc.Table("oauth_identities").AssertCount(1, testkit.Row{"user_id": s.user.ID})
		removed, _ := tests.Sent[appmail.PasswordChangedMail](tc, fake)
		assert.Empty(t, removed)
	})

	t.Run("keeps a proven account's factors and providers through a reset", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		account := protectedAccount(t, tc)
		tc.SignInWith("github", person("1", account.User.Email))
		id, token := resetLink(t, tc, account.User.Email)

		redeem(tc.Client(), id, token).AssertOK().AssertJSONPath("data.object", "two_factor_challenge")

		tc.Table("user_auth_factors").AssertHas(testkit.Row{"user_id": account.User.ID})
		tc.Table("oauth_identities").AssertCount(1, testkit.Row{"user_id": account.User.ID})
	})

	t.Run("records what the claim ended", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		s := registerAsStranger(t, tc)

		withLink(tc.Client(), magicLink(t, tc, victim, nil), tc).AssertCreated()

		var metadata map[string]any
		err := tc.App.DB.QueryRow(t.Context(),
			"SELECT metadata FROM security_events WHERE user_id = $1 AND type = 'sessions.revoked_others'",
			s.user.ID).Scan(&metadata)
		require.NoError(t, err)
		assert.Equal(t, appauth.RevokedForPreHijackDefense, metadata["reason"])
		assert.EqualValues(t, 1, metadata["tokens_revoked"])
		assert.EqualValues(t, 1, metadata["identities_removed"])
		assert.EqualValues(t, 2, metadata["factors_removed"])
		assert.EqualValues(t, 1, metadata["workspaces_trashed"])
		assert.EqualValues(t, 0, metadata["memberships_left"])
	})
}
