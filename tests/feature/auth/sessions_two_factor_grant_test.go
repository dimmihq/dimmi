package auth_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
protectedAccount returns a new account with two-factor authentication
on.
*/
func protectedAccount(t *testing.T, tc *tests.TestCase) factories.TwoFactorAccount {
	t.Helper()

	account, err := factories.WithTwoFactor(t.Context(), tc.App, tc.CreateUser())
	require.NoError(t, err)

	return account
}

/*
challenge signs in with the password and returns the browser holding the
challenge, with the answer that began it.
*/
func challenge(t *testing.T, tc *tests.TestCase, email string) (*testkit.Client, *testkit.Response) {
	t.Helper()

	res := tc.Client().PostJSON("/api/v1/sessions", login(email, factories.Password)).AssertOK()

	return tc.Client().WithCookie(res.Cookie(tc.App.Cookies.TwoFactorName())), res
}

/*
answer answers the browser's challenge with the method and code.
*/
func answer(browser *testkit.Client, method, code string) *testkit.Response {
	return browser.PostJSON("/api/v1/sessions", testkit.JSON{"grant": "two_factor", "method": method, "code": code})
}

/*
codeNow returns the code the authenticator with the secret shows now.
*/
func codeNow(t *testing.T, tc *tests.TestCase, secret string) string {
	t.Helper()

	code, err := twofactor.Code(secret, tc.Clock.Now())
	require.NoError(t, err)

	return code
}

func TestSessionStoreWithTwoFactor(t *testing.T) {
	t.Parallel()

	t.Run("asks a protected account for a second factor instead of signing in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account := protectedAccount(t, tc)

		_, res := challenge(t, tc, account.User.Email)

		res.AssertJSONPath("data.object", "two_factor_challenge").
			AssertJSONPath("data.expires_at", tests.Epoch().Add(5*time.Minute).Format(time.RFC3339)).
			AssertJSONPath("data.methods.0.method", "totp").
			AssertJSONPath("data.methods.0.factor_id", models.PublicID(models.PrefixAuthFactor, account.TOTP.ID)).
			AssertJSONPath("data.methods.0.name", "Authenticator app").
			AssertJSONPath("data.methods.0.preferred", true).
			AssertJSONPath("data.methods.0.options", nil).
			AssertJSONPath("data.methods.1.method", "recovery_code").
			AssertJSONPath("data.methods.1.preferred", false)
		cookie := res.Cookie(tc.App.Cookies.TwoFactorName())
		assert.True(t, cookie.HttpOnly)
		tc.Table("sessions").AssertCount(0)
		tc.Table("one_time_tokens").AssertHas(testkit.Row{
			"user_id": account.User.ID, "purpose": "two_factor_challenge", "attempts": 0,
		})
	})

	t.Run("signs in with a code from the authenticator, recording both methods", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account := protectedAccount(t, tc)
		browser, _ := challenge(t, tc, account.User.Email)

		res := answer(browser, "totp", codeNow(t, tc, account.Secret)).AssertCreated()

		res.AssertJSONPath("data.user.email", account.User.Email).
			AssertJSONPath("data.user.two_factor_enabled", true).
			AssertJSONPath("data.auth.auth_methods", []string{"password", "totp"}).
			AssertJSONPath("data.auth.two_factor_used", true)
		assert.Empty(t, res.Cookie(tc.App.Cookies.TwoFactorName()).Value)
		tc.Table("sessions").AssertHas(testkit.Row{
			"user_id": account.User.ID, "auth_method": "password", "confirmed_method": "totp",
		})
		tc.Table("user_auth_factors").AssertHas(testkit.Row{"id": account.TOTP.ID, "last_used_at": tests.Epoch()})
		tc.Browser(res).GetJSON("/api/v1/me").AssertOK()
	})

	t.Run("works once, and never takes the same code twice", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account := protectedAccount(t, tc)
		browser, _ := challenge(t, tc, account.User.Email)
		code := codeNow(t, tc, account.Secret)
		answer(browser, "totp", code).AssertCreated()

		answer(browser, "totp", code).AssertUnauthorized().AssertJSONPath("code", "two_factor_challenge_expired")

		again, _ := challenge(t, tc, account.User.Email)
		answer(again, "totp", code).AssertValidationError("code").AssertJSONPath("code", "invalid_code")
		tc.Clock.Advance(30 * time.Second)
		answer(again, "totp", codeNow(t, tc, account.Secret)).AssertCreated()
	})

	t.Run("signs in with a recovery code once and mails how many are left", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		account := protectedAccount(t, tc)
		browser, _ := challenge(t, tc, account.User.Email)
		typed := " " + strings.ToUpper(strings.ReplaceAll(account.RecoveryCodes[3], "-", " ")) + " "

		answer(browser, "recovery_code", typed).AssertCreated().
			AssertJSONPath("data.auth.auth_methods", []string{"password", "recovery_code"})

		sent, _ := tests.LastSent[appmail.RecoveryCodeUsedMail](tc, fake)
		assert.Equal(t, 9, sent.Remaining)
		tc.Table("security_events").AssertHas(testkit.Row{
			"user_id": account.User.ID, "type": "auth_factor.recovery_code_used",
		})
		again, _ := challenge(t, tc, account.User.Email)
		answer(again, "recovery_code", account.RecoveryCodes[3]).AssertValidationError("code")
		answer(again, "recovery_code", account.RecoveryCodes[4]).AssertCreated()
	})

	t.Run("lets only one of two tabs spend the same recovery code", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account := protectedAccount(t, tc)
		first, _ := challenge(t, tc, account.User.Email)
		second, _ := challenge(t, tc, account.User.Email)

		statuses := make([]int, 2)
		var wg sync.WaitGroup
		for i, browser := range []*testkit.Client{first, second} {
			wg.Go(func() { statuses[i] = answer(browser, "recovery_code", account.RecoveryCodes[0]).Status })
		}
		wg.Wait()

		assert.ElementsMatch(t, []int{fhttp.StatusCreated, fhttp.StatusUnprocessableEntity}, statuses)
	})

	t.Run("leaves out a used-up recovery set, and refuses it without counting a guess", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account := protectedAccount(t, tc)
		_, err := tc.Operator().Exec(t.Context(),
			`UPDATE user_auth_factors SET credential = '{"total":10,"remaining":0}' WHERE id = $1`, account.Recovery.ID)
		require.NoError(t, err)

		browser, res := challenge(t, tc, account.User.Email)

		res.AssertJSONPath("data.methods.#", 1)
		answer(browser, "recovery_code", account.RecoveryCodes[0]).
			AssertValidationError("method").
			AssertJSONPath("code", "method_not_available")
		answer(browser, "webauthn", "x").AssertValidationError("method")
		tc.Table("one_time_tokens").AssertHas(testkit.Row{"purpose": "two_factor_challenge", "attempts": 0})
	})

	t.Run("requires the method and the code", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{"grant": "two_factor"}).
			AssertValidationError("method", "code")
	})

	t.Run("burns the challenge after five wrong codes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account := protectedAccount(t, tc)
		browser, _ := challenge(t, tc, account.User.Email)

		for range 4 {
			answer(browser, "totp", "000000").AssertValidationError("code").AssertJSONPath("code", "invalid_code")
		}

		answer(browser, "totp", "000000").AssertUnauthorized().AssertJSONPath("code", "two_factor_challenge_expired")
		answer(browser, "totp", codeNow(t, tc, account.Secret)).AssertUnauthorized()
		tc.Table("sessions").AssertCount(0)
	})

	t.Run("answers a browser without a live challenge as expired", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account := protectedAccount(t, tc)
		browser, _ := challenge(t, tc, account.User.Email)

		answer(tc.Client(), "totp", "123456").AssertUnauthorized().
			AssertJSONPath("code", "two_factor_challenge_expired")

		tc.Clock.Advance(5 * time.Minute)
		answer(browser, "totp", codeNow(t, tc, account.Secret)).AssertUnauthorized()
	})

	t.Run("still challenges when no factor can answer", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, err := factories.EnableTwoFactor(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		_, res := challenge(t, tc, tc.Owner.Email)

		res.AssertJSONPath("data.methods", []any{})
		tc.Table("sessions").AssertCount(0)
	})

	t.Run("offers only the kinds the install enables", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__FACTORS__ENABLED=recovery_code"))
		account := protectedAccount(t, tc)

		browser, res := challenge(t, tc, account.User.Email)

		res.AssertJSONPath("data.methods.#", 1).AssertJSONPath("data.methods.0.method", "recovery_code")
		answer(browser, "totp", codeNow(t, tc, account.Secret)).AssertJSONPath("code", "method_not_available")
	})

	t.Run("challenges after a mailed code too", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		account := protectedAccount(t, tc)
		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": account.User.Email}).
			AssertStatus(fhttp.StatusAccepted)
		sent, _ := tests.LastSent[appmail.MagicLinkMail](tc, fake)

		res := tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "email_code", "email": account.User.Email, "code": sent.Code,
		}).AssertOK()

		browser := tc.Client().WithCookie(res.Cookie(tc.App.Cookies.TwoFactorName()))
		answer(browser, "totp", codeNow(t, tc, account.Secret)).AssertCreated().
			AssertJSONPath("data.auth.auth_methods", []string{"email_code", "totp"})
	})

	t.Run("keeps a fresh sign-in on the session's own user", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		account := protectedAccount(t, tc)
		first, _ := challenge(t, tc, account.User.Email)
		session := tc.Browser(answer(first, "totp", codeNow(t, tc, account.Secret)).AssertCreated())
		tc.Clock.Advance(30 * time.Second)

		reauth := session.PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": account.User.Email, "password": factories.Password, "reauth": true,
		}).AssertOK()
		cookie := reauth.Cookie(tc.App.Cookies.TwoFactorName())

		answer(tc.ActingAsOwner().WithCookie(cookie), "totp", codeNow(t, tc, account.Secret)).
			AssertValidationError("email").
			AssertJSONPath("code", "reauth_user_mismatch")
		answer(session.WithCookie(cookie), "totp", codeNow(t, tc, account.Secret)).AssertCreated()
	})
}
