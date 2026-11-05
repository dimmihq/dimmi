package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
brokenGrants is a first-party grant store that cannot be reached, as
when the OAuth server's tables are down.
*/
type brokenGrants struct{}

func (brokenGrants) RevokeUserAppGrants(context.Context, ids.UUID, string, ids.UUID) error {
	return errors.New("grant store unreachable")
}

/*
assertServerError checks the answer of a request that failed on the
server's side.
*/
func assertServerError(res *testkit.Response) {
	res.AssertStatus(fhttp.StatusInternalServerError).AssertJSONPath("code", "internal_error")
}

func TestMailedSecretFailures(t *testing.T) {
	t.Parallel()

	t.Run("sends and redeems nothing while the counters are out of reach", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		unverified := tc.ActingAs(tc.CreateUser(factories.Unverified))
		passwordless := signInByCode(t, tc, tc.CreateUser(factories.WithoutPassword))
		tc.BreakValkey()

		for _, res := range []*testkit.Response{
			tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}),
			tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}),
			redeem(tc.Client(), "prs_x", "x"),
			verify(tc, "ott_x", "x"),
			withCode(tc.Client(), tc.Owner.Email, "123-456"),
			unverified.PostJSON("/api/v1/email-verifications", nil),
			unverified.PostJSON("/api/v1/user/confirmation", testkit.JSON{"method": "password", "password": "x"}),
			passwordless.PostJSON("/api/v1/user/confirmation-codes", nil),
		} {
			res.AssertStatus(fhttp.StatusServiceUnavailable).AssertJSONPath("code", "auth_busy")
		}
	})

	t.Run("creates no account and sends nothing when a secret cannot be stored", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		unverified := tc.ActingAs(tc.CreateUser(factories.Unverified))
		passwordless := signInByCode(t, tc, tc.CreateUser(factories.WithoutPassword))
		tc.BreakTable("one_time_tokens")

		for _, res := range []*testkit.Response{
			tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")),
			tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}),
			tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": "newcomer@example.com"}),
			unverified.PostJSON("/api/v1/email-verifications", nil),
			passwordless.PostJSON("/api/v1/user/confirmation-codes", nil),
			passwordless.PostJSON("/api/v1/user/confirmation", testkit.JSON{"method": "email_code", "code": "123456"}),
			withCode(tc.Client(), tc.Owner.Email, "123-456"),
			tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{"grant": "magic_link", "token": "x"}),
			verify(tc, "ott_01jc0000000000000000000000", "x"),
		} {
			assertServerError(res)
		}
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
	})

	t.Run("signs nobody in when a secret cannot be used up or counted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		link := magicLink(t, tc, tc.Owner.Email, nil)
		_, id, token := verificationLink(t, tc)
		passwordless := signInByCode(t, tc, tc.CreateUser(factories.WithoutPassword))
		passwordless.PostJSON("/api/v1/user/confirmation-codes", nil).AssertStatus(fhttp.StatusAccepted)
		code, _ := tests.LastSent[appmail.ConfirmCodeMail](tc, tc.Mail())
		tc.FailWrites("one_time_tokens", "UPDATE")

		for _, res := range []*testkit.Response{
			withLink(tc.Client(), link, tc),
			withCode(tc.Client(), tc.Owner.Email, "000-000"),
			disown(tc, id, token),
			verify(tc, id, "wrong-token"),
			passwordless.PostJSON("/api/v1/user/confirmation", testkit.JSON{"method": "email_code", "code": code.Code}),
			tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}),
		} {
			assertServerError(res)
		}
		tc.Table("sessions").AssertCount(0, testkit.Row{"user_id": tc.Owner.ID})
	})

	t.Run("changes no password when the reset cannot be read, counted or used up", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)
		tc.FailWrites("password_reset_tokens", "UPDATE")

		redeem(tc.Client(), id, "wrong-token").AssertValidationError("token")

		tc.FailWrites("password_reset_tokens", "DELETE")
		assertServerError(redeem(tc.Client(), id, token))

		tc.BreakTable("password_reset_tokens")
		assertServerError(redeem(tc.Client(), id, token))
		require.NoError(t, tc.App.Limiter.Clear(t.Context(), "reset:minute:"+tc.Owner.Email))
		assertServerError(tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}))
		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertCreated()
	})

	t.Run("keeps the reset when burning it after wrong tokens fails", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(behindProxy))
		id, _ := resetLink(t, tc, tc.Owner.Email)
		for range 4 {
			redeem(tc.Client(), id, "wrong-token").AssertValidationError("token")
		}
		tc.Drain()
		tc.FailWrites("password_reset_tokens", "DELETE")

		redeem(tc.Client(), id, "wrong-token").AssertValidationError("token")
		tc.Drain()

		tc.Table("password_reset_tokens").AssertCount(1, testkit.Row{"user_id": tc.Owner.ID})
	})

	t.Run("keeps the old password and every session when the account cannot be updated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		elsewhere := tc.ActingAsOwner()
		id, token := resetLink(t, tc, tc.Owner.Email)
		unverified := tc.CreateUser(factories.Unverified, factories.WithoutPassword)
		unverifiedLink := magicLink(t, tc, unverified.Email, nil)
		_, verifyID, verifyToken := verificationLink(t, tc)
		tc.FailWrites("users", "UPDATE")

		assertServerError(redeem(tc.Client(), id, token))
		assertServerError(withLink(tc.Client(), unverifiedLink, tc))
		assertServerError(disown(tc, verifyID, verifyToken))
		elsewhere.GetJSON("/api/v1/me").AssertOK()
	})

	t.Run("keeps the old password when sessions cannot be ended", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)
		tc.FailWrites("sessions", "DELETE")

		assertServerError(redeem(tc.Client(), id, token))
		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertCreated()
	})

	t.Run("keeps the old password when the reset cannot be logged", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)
		tc.FailWrites("security_events", "INSERT")

		assertServerError(redeem(tc.Client(), id, token))
		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertStatus(500)
	})

	t.Run("resets nothing and claims nothing while app grants cannot be ended", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.AppGrants = brokenGrants{} }))
		id, token := resetLink(t, tc, tc.Owner.Email)
		link := magicLink(t, tc, "grace@example.com", nil)
		tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated()

		assertServerError(redeem(tc.Client(), id, token))
		assertServerError(withLink(tc.Client(), link, tc))
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com", "password_hash": nil})
	})

	t.Run("claims nothing when the squatter's sessions, tokens or log cannot be touched", func(t *testing.T) {
		t.Parallel()

		for _, broken := range []struct{ table, kind string }{
			{"sessions", "DELETE"},
			{"personal_access_tokens", "UPDATE"},
			{"security_events", "INSERT"},
			{"river_job", "INSERT"},
			{"oauth_identities", "DELETE"},
			{"user_auth_factors", "DELETE"},
			{"workspaces", "UPDATE"},
		} {
			tc := tests.New(t)
			link := magicLink(t, tc, "grace@example.com", nil)
			tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated()
			tc.FailWrites(broken.table, broken.kind)

			assertServerError(withLink(tc.Client(), link, tc))
			tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com", "password_hash": nil})
		}
	})

	t.Run("claims nothing when the account's memberships cannot be listed or left", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		link := magicLink(t, tc, "grace@example.com", nil)
		tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated()
		member := membershipOf(t, tc, userByEmail(t, tc, "grace@example.com"))
		_, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: models.Workspace{ID: member.WorkspaceID}, User: tc.CreateUser(), Role: enums.RoleOwner,
		})
		require.NoError(t, err)
		tc.FailWrites("workspace_members", "UPDATE")

		assertServerError(withLink(tc.Client(), link, tc))
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com", "password_hash": nil})
	})

	t.Run("resets nothing when the user's access tokens cannot be revoked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)
		tc.MintToken(membershipOf(t, tc, tc.Owner), "*")
		tc.FailWrites("personal_access_tokens", "UPDATE")

		assertServerError(redeem(tc.Client(), id, token))

		tc.Table("password_reset_tokens").AssertCount(1, testkit.Row{"user_id": tc.Owner.ID})
	})

	t.Run("creates no account when signing up through a link fails", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		link := magicLink(t, tc, "grace@example.com", nil)
		tc.FailWrites("users", "INSERT")

		assertServerError(withLink(tc.Client(), link, tc))
		tc.Table("one_time_tokens").AssertCount(1, testkit.Row{"email": "grace@example.com", "consumed_at": nil})
	})

	t.Run("starts no session when a new-device mail cannot be queued", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.ActingAsOwner()
		tc.FailWrites("river_job", "INSERT")

		assertServerError(tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)))
	})

	t.Run("fails a locking sign-in whose warning cannot be queued", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_EMAIL_SOFT_LOCK=1"))
		tc.FailWrites("river_job", "INSERT")

		assertServerError(tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, "wrong-password")))
	})

	t.Run("signs nobody in when the use of a link cannot be logged", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		link := magicLink(t, tc, tc.Owner.Email, nil)
		tc.FailWrites("security_events", "INSERT")

		assertServerError(withLink(tc.Client(), link, tc))
		tc.Table("one_time_tokens").AssertCount(1, testkit.Row{"email": tc.Owner.Email, "consumed_at": nil})
	})
}
