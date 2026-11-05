package members_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestTwoFactorResetStore(t *testing.T) {
	t.Parallel()

	t.Run("an admin starts a two factor reset and the mail goes to the teammate", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		admin, adminMember := join(t, tc, enums.RoleAdmin)
		teammate, member := protected(t, tc, enums.RoleMember)

		in(tc, admin).PostJSON(resetPath(member), testkit.JSON{}).
			AssertCreated().
			AssertJSONPath("data.object", "two_factor_reset").
			AssertJSONPath("data.member_id", memberID(member)).
			AssertJSONPath("data.status", "pending")

		sent, message := tests.LastSent[appmail.TwoFactorResetMail](tc, fake)
		assert.Equal(t, []string{teammate.Email}, tests.SentTo(message))
		assert.Contains(t, sent.URL, "/two-factor-reset#id=ott_")
		assert.Equal(t, admin.Name, sent.Admin)
		assert.Equal(t, tc.Workspace.Workspace.Name, sent.Workspace)
		tc.Table("security_events").AssertHas(testkit.Row{
			"user_id": teammate.ID, "type": auth.EventTwoFactorResetAsked,
		})
		tc.Table("audit_logs").AssertHas(testkit.Row{
			"event": "member.two_factor_reset_requested", "actor_id": adminMember.ID, "subject_id": member.ID,
		})
		tc.Table("users").AssertMissing(testkit.Row{"id": teammate.ID, "two_factor_confirmed_at": nil})
	})

	t.Run("an admin cannot reset an owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, _ := join(t, tc, enums.RoleAdmin)
		_, owner := protected(t, tc, enums.RoleOwner)

		in(tc, admin).PostJSON(resetPath(owner), testkit.JSON{}).AssertForbidden()
	})

	t.Run("an owner resets another owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, owner := protected(t, tc, enums.RoleOwner)

		in(tc, tc.Owner).PostJSON(resetPath(owner), testkit.JSON{}).AssertCreated()
	})

	t.Run("a member cannot start a reset", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)
		_, other := protected(t, tc, enums.RoleMember)

		in(tc, member).PostJSON(resetPath(other), testkit.JSON{}).AssertForbidden()
	})

	t.Run("a reset is refused when the teammate works in another company", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate, member := protected(t, tc, enums.RoleMember)
		elsewhere, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		_, err = factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: elsewhere.Workspace, User: teammate, Role: enums.RoleMember,
		})
		require.NoError(t, err)

		in(tc, tc.Owner).PostJSON(resetPath(member), testkit.JSON{}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "two_factor_reset_blocked")
		tc.Table("one_time_tokens").AssertCount(0, testkit.Row{"purpose": "two_factor_reset"})
	})

	t.Run("refuses a teammate without two factor, unverified, inactive, or oneself", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		_, plain := join(t, tc, enums.RoleMember)
		_, unverified := protected(t, tc, enums.RoleMember, factories.Unverified)
		_, gone := protected(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, gone))

		owner.PostJSON(resetPath(plain), testkit.JSON{}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "two_factor_not_enabled")
		owner.PostJSON(resetPath(unverified), testkit.JSON{}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "target_unverified")
		owner.PostJSON(resetPath(gone), testkit.JSON{}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "member_inactive")
		owner.PostJSON(resetPath(tc.Workspace.Owner), testkit.JSON{}).
			AssertValidationError("member").AssertJSONPath("code", "cannot_reset_self")
	})

	t.Run("allows three resets a day for one teammate", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		_, member := protected(t, tc, enums.RoleMember)

		for range 3 {
			owner.PostJSON(resetPath(member), testkit.JSON{}).AssertCreated()
			tc.Clock.Advance(time.Second)
		}

		owner.PostJSON(resetPath(member), testkit.JSON{}).AssertStatus(fhttp.StatusTooManyRequests)
	})

	t.Run("needs a recent confirmation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		_, member := protected(t, tc, enums.RoleMember)
		tc.Clock.Travel(tc.App.Config.Auth.PasswordConfirmation.TTL + time.Second)

		owner.PostJSON(resetPath(member), testkit.JSON{}).
			AssertStatus(fhttp.StatusLocked).AssertJSONPath("code", "password_confirmation_required")
	})

	t.Run("answers not found for a member of another workspace and a malformed id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		stranger := tc.CreateUser()
		elsewhere, err := factories.Workspace(t.Context(), tc.App, stranger)
		require.NoError(t, err)

		owner := in(tc, tc.Owner)
		owner.PostJSON(resetPath(elsewhere.Owner), testkit.JSON{}).AssertNotFound()
		owner.PostJSON("/api/v1/members/mem_nope/two-factor-resets", testkit.JSON{}).AssertNotFound()
	})

	t.Run("an access token cannot start a reset", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := protected(t, tc, enums.RoleMember)

		tc.ActingAsToken(tc.Workspace.Owner, "members:write").PostJSON(resetPath(member), testkit.JSON{}).
			AssertForbidden().AssertJSONPath("code", "first_party_only")
	})
}

func TestTwoFactorResetUpdate(t *testing.T) {
	t.Parallel()

	t.Run("a reset needs the teammate's password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		teammate, member := join(t, tc, enums.RoleMember)
		elsewhere := tc.ActingAs(teammate)
		_, err := factories.WithTwoFactor(t.Context(), tc.App, teammate)
		require.NoError(t, err)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)

		redeem(tc.Client(), id, testkit.JSON{"token": token}).
			AssertValidationError("password").AssertJSONPath("code", "password_required")
		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": "not-the-password"}).
			AssertValidationError("password").AssertJSONPath("code", "invalid_credentials")
		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": factories.Password}).
			AssertOK().
			AssertJSONPath("data.status", "completed").
			AssertJSONPath("data.member_id", memberID(member))

		tc.Table("users").AssertHas(testkit.Row{"id": teammate.ID, "two_factor_confirmed_at": nil})
		tc.Table("user_auth_factors").AssertCount(0, testkit.Row{"user_id": teammate.ID})
		elsewhere.GetJSON("/api/v1/me").AssertUnauthorized()
		_, message := tests.LastSent[appmail.TwoFactorResetCompletedMail](tc, fake)
		assert.Equal(t, []string{teammate.Email}, tests.SentTo(message))
		tc.Table("security_events").AssertHas(testkit.Row{
			"user_id": teammate.ID, "type": auth.EventTwoFactorResetDone,
		})
		tc.Table("audit_logs").AssertHas(testkit.Row{
			"event": "member.two_factor_reset_completed", "actor_id": member.ID, "subject_id": member.ID,
		})
		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": factories.Password}).
			AssertValidationError("token").AssertJSONPath("code", "invalid_two_factor_reset_token")
	})

	t.Run("a teammate without a password resets with the link alone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate, member := protected(t, tc, enums.RoleMember, factories.WithoutPassword)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)

		redeem(tc.Client(), id, testkit.JSON{"token": token}).AssertOK()

		tc.Table("users").AssertHas(testkit.Row{"id": teammate.ID, "two_factor_confirmed_at": nil})
	})

	t.Run("works where the workspace requires a second factor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate, member := protected(t, tc, enums.RoleMember)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)
		_, err := tc.Operator().Exec(t.Context(), `UPDATE workspaces
			SET settings = jsonb_set(settings, '{security}', '{"require_two_factor": true}') WHERE id = $1`,
			tc.Workspace.Workspace.ID)
		require.NoError(t, err)

		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": factories.Password}).AssertOK()

		tc.Table("users").AssertHas(testkit.Row{"id": teammate.ID, "two_factor_confirmed_at": nil})
	})

	t.Run("five wrong passwords burn the link", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate, member := protected(t, tc, enums.RoleMember)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)

		for range 5 {
			redeem(tc.Client(), id, testkit.JSON{"token": token, "password": "not-the-password"}).
				AssertJSONPath("code", "invalid_credentials")
		}

		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": factories.Password}).
			AssertJSONPath("code", "invalid_two_factor_reset_token")
		tc.Table("users").AssertMissing(testkit.Row{"id": teammate.ID, "two_factor_confirmed_at": nil})
	})

	t.Run("refuses an unknown, wrong or expired link alike", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := protected(t, tc, enums.RoleMember)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)

		redeem(tc.Client(), "ott_nope", testkit.JSON{"token": token}).
			AssertJSONPath("code", "invalid_two_factor_reset_token")
		redeem(tc.Client(), id, testkit.JSON{"token": "wrong"}).
			AssertJSONPath("code", "invalid_two_factor_reset_token")
		tc.Clock.Advance(31 * time.Minute)
		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": factories.Password}).
			AssertJSONPath("code", "invalid_two_factor_reset_token")
		redeem(tc.Client(), id, testkit.JSON{}).AssertValidationError("token")
	})

	t.Run("lifts the sign-in lock so the teammate gets straight back in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_EMAIL_SOFT_LOCK=3"))
		teammate, member := protected(t, tc, enums.RoleMember)
		fail(t, tc.Client(), teammate.Email, 3)
		tc.Client().PostJSON("/api/v1/sessions", login(teammate.Email, factories.Password)).AssertStatus(429)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)

		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": factories.Password}).AssertOK()

		tc.Client().PostJSON("/api/v1/sessions", login(teammate.Email, factories.Password)).AssertCreated()
	})
}
