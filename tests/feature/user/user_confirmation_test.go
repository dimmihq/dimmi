package user_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
confirmationWindow is how long a confirmation holds by default.
*/
const confirmationWindow = 15 * time.Minute

/*
signInByMail signs a user in with a mailed code, the way a user without
a password does, and returns their browser.
*/
func signInByMail(t *testing.T, tc *tests.TestCase, user models.User) *testkit.Client {
	t.Helper()

	fake := tc.Mail()
	tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": user.Email}).AssertStatus(fhttp.StatusAccepted)
	sent, _ := tests.LastSent[appmail.MagicLinkMail](tc, fake)

	return tc.Browser(tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "email_code", "email": user.Email, "code": sent.Code,
	}).AssertCreated())
}

/*
confirm confirms the browser's user with the body.
*/
func confirm(browser *testkit.Client, body testkit.JSON) *testkit.Response {
	return browser.PostJSON("/api/v1/user/confirmation", body)
}

func TestUserConfirmationShow(t *testing.T) {
	t.Parallel()

	t.Run("counts a fresh sign-in as a confirmation until the window closes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()

		browser.GetJSON("/api/v1/user/confirmation").AssertOK().
			AssertJSONPath("data.object", "confirmation").
			AssertJSONPath("data.method", "password").
			AssertJSONPath("data.confirmed_until", tests.Epoch().Add(confirmationWindow).Format(time.RFC3339))

		tc.Clock.Advance(confirmationWindow)

		browser.GetJSON("/api/v1/user/confirmation").AssertNotFound().AssertJSONPath("code", "not_found")
	})

	t.Run("refuses an access token, which is not the user themselves", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "*").GetJSON("/api/v1/user/confirmation").
			AssertForbidden().
			AssertJSONPath("code", "first_party_only")
	})
}

func TestUserConfirmationStore(t *testing.T) {
	t.Parallel()

	t.Run("confirms with the password and leaves the sign-in time alone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		tc.Clock.Advance(time.Hour)

		confirmed := confirm(browser, testkit.JSON{"method": "password", "password": factories.Password}).
			AssertCreated().
			AssertJSONPath("data.method", "password").
			AssertJSONPath("data.confirmed_until", tests.Epoch().Add(time.Hour+confirmationWindow).Format(time.RFC3339))

		browser.GetJSON("/api/v1/me").AssertJSONPath("data.auth.authenticated_at", tests.Epoch().Format(time.RFC3339))
		browser.GetJSON("/api/v1/user/confirmation").AssertOK().
			AssertJSONPath("data.method", "password").
			AssertJSONPath("data.confirmed_until", confirmed.String("data.confirmed_until"))
		tc.Table("sessions").AssertHas(testkit.Row{
			"user_id": tc.Owner.ID, "authenticated_at": tests.Epoch(), "confirmed_at": tests.Epoch().Add(time.Hour),
			"confirmed_method": "password",
		})
	})

	t.Run("lets a sensitive step through only within the window", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		tc.Clock.Advance(confirmationWindow + time.Second)

		browser.Delete("/api/v1/sessions").
			AssertStatus(fhttp.StatusLocked).
			AssertJSONPath("code", "password_confirmation_required")
		confirm(browser, testkit.JSON{"method": "password", "password": factories.Password}).AssertCreated()

		browser.Delete("/api/v1/sessions").AssertNoContent()
	})

	t.Run("refuses a wrong password and counts it toward the lockout", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()

		for range 4 {
			confirm(browser, testkit.JSON{"method": "password", "password": "wrong-password"}).
				AssertValidationError("password").
				AssertJSONPath("code", "invalid_password")
		}

		confirm(browser, testkit.JSON{"method": "password", "password": "wrong-password"}).
			AssertValidationError("password")
		confirm(browser, testkit.JSON{"method": "password", "password": factories.Password}).
			AssertStatus(fhttp.StatusTooManyRequests).
			AssertJSONPath("code", "too_many_attempts")
	})

	t.Run("refuses a way of confirming the account cannot use, without counting it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()

		for _, method := range []string{"email_code", "totp", "recovery_code", "webauthn"} {
			confirm(browser, testkit.JSON{"method": method, "code": "123456"}).
				AssertValidationError("method").
				AssertJSONPath("code", "method_not_available")
		}

		tc.Table("security_events").AssertCount(0, testkit.Row{"user_id": tc.Owner.ID, "type": "login.failed"})
	})

	t.Run("asks for a method", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		confirm(tc.ActingAsOwner(), testkit.JSON{"password": factories.Password}).AssertValidationError("method")
	})

	t.Run("confirms a user without a password with a code mailed to them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(factories.WithoutPassword)
		browser := signInByMail(t, tc, user)
		fake := tc.Mail()

		browser.PostJSON("/api/v1/user/confirmation-codes", nil).AssertStatus(fhttp.StatusAccepted)
		sent, message := tests.LastSent[appmail.ConfirmCodeMail](tc, fake)

		assert.Equal(t, []string{user.Email}, tests.SentTo(message))
		confirm(browser, testkit.JSON{"method": "password", "password": "anything"}).
			AssertJSONPath("code", "method_not_available")
		confirm(browser, testkit.JSON{"method": "email_code", "code": "000-000"}).
			AssertValidationError("code").
			AssertJSONPath("code", "invalid_code")
		confirm(browser, testkit.JSON{"method": "email_code", "code": sent.Code}).AssertCreated().
			AssertJSONPath("data.method", "email_code")
		confirm(browser, testkit.JSON{"method": "email_code", "code": sent.Code}).AssertValidationError("code")
	})

	t.Run("stops a mailed code working after ten minutes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := signInByMail(t, tc, tc.CreateUser(factories.WithoutPassword))
		fake := tc.Mail()
		browser.PostJSON("/api/v1/user/confirmation-codes", nil).AssertStatus(fhttp.StatusAccepted)
		sent, _ := tests.LastSent[appmail.ConfirmCodeMail](tc, fake)
		tc.Clock.Advance(10 * time.Minute)

		confirm(browser, testkit.JSON{"method": "email_code", "code": sent.Code}).AssertValidationError("code")
	})
}

func TestUserConfirmationCodeStore(t *testing.T) {
	t.Parallel()

	t.Run("mails codes only to users without a password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().PostJSON("/api/v1/user/confirmation-codes", nil).
			AssertValidationError("method").
			AssertJSONPath("code", "method_not_available")
	})

	t.Run("mails at most one code a minute", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := signInByMail(t, tc, tc.CreateUser(factories.WithoutPassword))

		browser.PostJSON("/api/v1/user/confirmation-codes", nil).AssertStatus(fhttp.StatusAccepted)
		browser.PostJSON("/api/v1/user/confirmation-codes", nil).AssertStatus(fhttp.StatusTooManyRequests)
	})
}
