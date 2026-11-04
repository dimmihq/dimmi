package user_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
factorsPath is the caller's factor collection.
*/
const factorsPath = "/api/v1/user/auth-factors"

/*
enrollment is an authenticator just added: its public id and the
secret its setup showed.
*/
type enrollment struct {
	id     string
	secret string
}

/*
addAuthenticator adds a pending authenticator for the browser's user.
*/
func addAuthenticator(t *testing.T, browser *testkit.Client) enrollment {
	t.Helper()

	res := browser.PostJSON(factorsPath, testkit.JSON{"type": "totp"}).AssertCreated()

	return enrollment{id: res.String("data.id"), secret: res.String("data.setup.secret")}
}

/*
codeFor returns the code the authenticator shows now.
*/
func codeFor(t *testing.T, tc *tests.TestCase, secret string) string {
	t.Helper()

	code, err := twofactor.Code(secret, tc.Clock.Now())
	require.NoError(t, err)

	return code
}

/*
turnOn adds and activates an authenticator for the browser's user and
returns it with the browser its rotated session belongs to and the
recovery codes shown.
*/
func turnOn(t *testing.T, tc *tests.TestCase, browser *testkit.Client) (enrollment, *testkit.Client, []string) {
	t.Helper()

	added := addAuthenticator(t, browser)
	res := browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"code": codeFor(t, tc, added.secret)}).AssertOK()

	var body struct {
		Data struct {
			RecoveryCodes struct {
				Codes []string `json:"codes"`
			} `json:"recovery_codes"`
		} `json:"data"`
	}
	res.Decode(&body)

	return added, rotated(tc, res), body.Data.RecoveryCodes.Codes
}

func TestAuthFactorStore(t *testing.T) {
	t.Parallel()

	t.Run("starts an authenticator, showing its setup once and keeping the secret sealed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.ActingAsOwner().PostJSON(factorsPath, testkit.JSON{"type": "totp"}).AssertCreated()

		res.AssertHeader("Cache-Control", "no-store").
			AssertJSONPath("data.object", "auth_factor").
			AssertJSONPath("data.type", "totp").
			AssertJSONPath("data.name", "Authenticator app").
			AssertJSONPath("data.status", "pending").
			AssertJSONPath("data.preferred", false).
			AssertJSONPath("data.confirmed_at", nil)
		secret := res.String("data.setup.secret")
		assert.Len(t, secret, 32)
		assert.Contains(t, res.String("data.setup.otpauth_url"), "issuer=dimmi")
		assert.True(t, strings.HasPrefix(res.String("data.setup.qr_svg"), "<svg "))
		var sealed []byte
		err := tc.App.DB.QueryRow(t.Context(), "SELECT secret_encrypted FROM user_auth_factors").Scan(&sealed)
		require.NoError(t, err)
		assert.NotContains(t, string(sealed), secret)
		tc.ActingAsOwner().GetJSON(factorsPath).AssertOK().AssertJSONMissing("data.0.setup")
	})

	t.Run("names the authenticator as asked, without invisible characters", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()

		browser.PostJSON(factorsPath, testkit.JSON{"type": "totp", "name": " Work\u200b phone "}).AssertCreated().
			AssertJSONPath("data.name", "Work phone")
		browser.PostJSON(factorsPath, testkit.JSON{"type": "totp", "name": "\u200b"}).AssertValidationError("name")
		browser.PostJSON(factorsPath, testkit.JSON{"type": "sms"}).AssertValidationError("type")
	})

	t.Run("replaces an unfinished authenticator when asked again", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		first := addAuthenticator(t, browser)

		addAuthenticator(t, browser)

		tc.Table("user_auth_factors").AssertCount(1)
		browser.PatchJSON(factorsPath+"/"+first.id, testkit.JSON{"code": codeFor(t, tc, first.secret)}).
			AssertNotFound()
	})

	t.Run("allows one authenticator at a time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, browser, _ := turnOn(t, tc, tc.ActingAsOwner())

		browser.PostJSON(factorsPath, testkit.JSON{"type": "totp"}).
			AssertValidationError("type").
			AssertJSONPath("code", "factor_limit_reached")
	})

	t.Run("refuses a kind the install does not enable", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__FACTORS__ENABLED=recovery_code"))

		tc.ActingAsOwner().PostJSON(factorsPath, testkit.JSON{"type": "totp"}).
			AssertValidationError("type").
			AssertJSONPath("code", "factor_type_not_available")
	})

	t.Run("needs a recent confirmation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		tc.Clock.Advance(confirmationWindow + time.Second)

		browser.PostJSON(factorsPath, testkit.JSON{"type": "totp"}).
			AssertStatus(fhttp.StatusLocked).
			AssertJSONPath("code", "password_confirmation_required")
	})

	t.Run("makes new recovery codes only with two-factor on, and the old ones stop working", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		browser.PostJSON(factorsPath, testkit.JSON{"type": "recovery_code"}).
			AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "two_factor_not_enabled")
		_, browser, old := turnOn(t, tc, browser)

		res := browser.PostJSON(factorsPath, testkit.JSON{"type": "recovery_code"}).AssertCreated()

		res.AssertHeader("Cache-Control", "no-store").
			AssertJSONPath("data.type", "recovery_code").
			AssertJSONPath("data.status", "active").
			AssertJSONPath("data.remaining", 10).
			AssertJSONPath("data.total", 10).
			AssertJSONPath("data.codes.#", 10)
		assert.NotEqual(t, old[0], res.String("data.codes.0"))
		tc.Table("user_auth_factors").AssertCount(1, testkit.Row{"type": "recovery_code"})
		tc.Table("security_events").AssertHas(testkit.Row{"type": "auth_factor.recovery_codes_regenerated"})
	})

	t.Run("adds no more than ten factors an hour", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()

		for range 10 {
			addAuthenticator(t, browser)
		}

		browser.PostJSON(factorsPath, testkit.JSON{"type": "totp"}).
			AssertStatus(fhttp.StatusTooManyRequests).
			AssertJSONPath("code", "too_many_attempts")
	})
}

func TestAuthFactorActivate(t *testing.T) {
	t.Parallel()

	t.Run("turns two-factor on with the first code, once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		browser := tc.ActingAsOwner()
		added := addAuthenticator(t, browser)

		code := codeFor(t, tc, added.secret)

		res := browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"code": code}).AssertOK()

		res.AssertHeader("Cache-Control", "no-store").
			AssertJSONPath("data.status", "active").
			AssertJSONPath("data.preferred", true).
			AssertJSONPath("data.confirmed_at", tests.Epoch().Format(time.RFC3339)).
			AssertJSONPath("data.recovery_codes.type", "recovery_code").
			AssertJSONPath("data.recovery_codes.remaining", 10).
			AssertJSONPath("data.recovery_codes.codes.#", 10)
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "two_factor_confirmed_at": tests.Epoch()})
		tc.Table("security_events").AssertHas(testkit.Row{"type": "auth_factor.added"}).
			AssertHas(testkit.Row{"type": "two_factor.enabled"})
		tests.LastSent[appmail.TwoFactorEnabledMail](tc, fake)
		browser.GetJSON("/api/v1/me").AssertUnauthorized()
		rotated(tc, res).GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.user.two_factor_enabled", true)
		rotated(tc, res).PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"code": "123456"}).
			AssertValidationError("code").
			AssertJSONPath("code", "factor_already_active")
	})

	t.Run("throws an unfinished authenticator away after five wrong codes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		added := addAuthenticator(t, browser)

		for range 5 {
			browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"code": "000000"}).
				AssertValidationError("code").
				AssertJSONPath("code", "invalid_code")
		}

		browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"code": codeFor(t, tc, added.secret)}).AssertNotFound()
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "two_factor_confirmed_at": nil})
	})

	t.Run("treats an authenticator left unfinished for an hour as gone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		added := addAuthenticator(t, browser)
		tc.Clock.Advance(time.Hour)

		browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"code": codeFor(t, tc, added.secret)}).AssertNotFound()
		browser.GetJSON(factorsPath).AssertOK().AssertJSONPath("data.#", 0)
	})

	t.Run("refuses a code sent with other changes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		added := addAuthenticator(t, browser)

		browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"code": "123456", "name": "Phone"}).
			AssertValidationError("name").
			AssertJSONPath("code", "prohibited")
	})

	t.Run("finds no factor of another user, nor a malformed id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		added := addAuthenticator(t, tc.ActingAsOwner())
		stranger := tc.ActingAs(tc.CreateUser())

		stranger.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"code": "123456"}).AssertNotFound()
		stranger.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"name": "Mine"}).AssertNotFound()
		stranger.Delete(factorsPath + "/" + added.id).AssertNotFound()
		stranger.PatchJSON(factorsPath+"/fac_nope", testkit.JSON{"name": "Mine"}).AssertNotFound()
		stranger.Delete(factorsPath + "/fac_nope").AssertNotFound()
	})
}

func TestAuthFactorUpdate(t *testing.T) {
	t.Parallel()

	t.Run("renames a factor and logs it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		added, browser, _ := turnOn(t, tc, tc.ActingAsOwner())

		browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"name": "Work phone"}).AssertOK().
			AssertJSONPath("data.name", "Work phone")
		browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"name": " "}).AssertValidationError("name")
		tc.Table("security_events").AssertHas(testkit.Row{"type": "auth_factor.renamed"})
	})

	t.Run("moves the preferred factor, but never to recovery codes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		added, browser, _ := turnOn(t, tc, tc.ActingAsOwner())
		recovery := browser.GetJSON(factorsPath).AssertOK().String("data.1.id")

		browser.PatchJSON(factorsPath+"/"+recovery, testkit.JSON{"preferred": true}).
			AssertValidationError("preferred")
		browser.PatchJSON(factorsPath+"/"+added.id, testkit.JSON{"preferred": true}).AssertOK().
			AssertJSONPath("data.preferred", true)
		tc.Table("user_auth_factors").AssertCount(1, testkit.Row{"is_preferred": true})
	})
}

func TestAuthFactorIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists the factors, preferred first, with the recovery codes left and no secrets", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		added, browser, _ := turnOn(t, tc, tc.ActingAsOwner())

		res := browser.GetJSON(factorsPath).AssertOK()

		res.AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.id", added.id).
			AssertJSONPath("data.0.type", "totp").
			AssertJSONPath("data.0.last_used_at", nil).
			AssertJSONMissing("data.0.remaining").
			AssertJSONPath("data.1.type", "recovery_code").
			AssertJSONPath("data.1.remaining", 10).
			AssertJSONPath("data.1.total", 10).
			AssertJSONMissing("data.1.codes").
			AssertJSONMissing("data.0.secret_encrypted").
			AssertJSONMissing("data.0.last_counter")
	})

	t.Run("still lists factors of a kind the install no longer enables", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__FACTORS__ENABLED=recovery_code"))
		account, err := factories.WithTwoFactor(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		res := tc.Browser(signInWithRecoveryCode(t, tc, account)).GetJSON(factorsPath).AssertOK()

		res.AssertJSONPath("data.0.type", "totp").AssertJSONPath("data.1.remaining", 9)
	})

	t.Run("is for the user themselves, not their tokens", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "*").GetJSON(factorsPath).
			AssertForbidden().
			AssertJSONPath("code", "first_party_only")
	})
}

/*
signInWithRecoveryCode signs the account in with its password and its
first recovery code.
*/
func signInWithRecoveryCode(t *testing.T, tc *tests.TestCase, account factories.TwoFactorAccount) *testkit.Response {
	t.Helper()

	res := tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "password", "email": account.User.Email, "password": factories.Password,
	}).AssertOK()

	return tc.Client().WithCookie(res.Cookie(tc.App.Cookies.TwoFactorName())).PostJSON("/api/v1/sessions",
		testkit.JSON{"grant": "two_factor", "method": "recovery_code", "code": account.RecoveryCodes[0]}).
		AssertCreated()
}

func TestAuthFactorDestroy(t *testing.T) {
	t.Parallel()

	t.Run("removes an unfinished authenticator and nothing else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		browser := tc.ActingAsOwner()
		added := addAuthenticator(t, browser)

		browser.Delete(factorsPath + "/" + added.id).AssertNoContent()

		tc.Table("user_auth_factors").AssertCount(0)
		mails, _ := tests.Sent[appmail.TwoFactorDisabledMail](tc, fake)
		assert.Empty(t, mails)
	})

	t.Run("keeps the recovery codes while two-factor is on", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, browser, _ := turnOn(t, tc, tc.ActingAsOwner())
		recovery := browser.GetJSON(factorsPath).AssertOK().String("data.1.id")

		browser.Delete(factorsPath+"/"+recovery).
			AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "recovery_codes_required")
	})

	t.Run("turns two-factor off with the last authenticator, keeping the browser signed in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		added, browser, _ := turnOn(t, tc, tc.ActingAsOwner())

		browser.Delete(factorsPath + "/" + added.id).AssertNoContent()

		tc.Table("user_auth_factors").AssertCount(0)
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "two_factor_confirmed_at": nil})
		tc.Table("security_events").AssertHas(testkit.Row{"type": "auth_factor.removed"}).
			AssertHas(testkit.Row{"type": "two_factor.disabled"})
		tests.LastSent[appmail.TwoFactorDisabledMail](tc, fake)
		browser.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.user.two_factor_enabled", false)
		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": tc.Owner.Email, "password": factories.Password,
		}).AssertCreated()
	})

	t.Run("needs a recent confirmation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		added, browser, _ := turnOn(t, tc, tc.ActingAsOwner())
		tc.Clock.Advance(confirmationWindow + time.Second)

		browser.Delete(factorsPath + "/" + added.id).AssertStatus(fhttp.StatusLocked)
		browser.Delete(factorsPath).AssertStatus(fhttp.StatusLocked)
	})
}

func TestAuthFactorDestroyMany(t *testing.T) {
	t.Parallel()

	t.Run("turns two-factor off in one step", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		_, browser, _ := turnOn(t, tc, tc.ActingAsOwner())

		browser.Delete(factorsPath).AssertNoContent()

		tc.Table("user_auth_factors").AssertCount(0)
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "two_factor_confirmed_at": nil})
		tests.LastSent[appmail.TwoFactorDisabledMail](tc, fake)
	})

	t.Run("only clears unfinished factors of an account without two-factor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		browser := tc.ActingAsOwner()
		browser.Delete(factorsPath).AssertNoContent()
		addAuthenticator(t, browser)

		browser.Delete(factorsPath).AssertNoContent()

		tc.Table("user_auth_factors").AssertCount(0)
		mails, _ := tests.Sent[appmail.TwoFactorDisabledMail](tc, fake)
		assert.Empty(t, mails)
	})
}

/*
rotated returns the browser holding the session the response rotated
to.
*/
func rotated(tc *tests.TestCase, res *testkit.Response) *testkit.Client {
	return tc.Client().WithCookie(res.Cookie(tc.App.Session.CookieName()))
}
