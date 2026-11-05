package auth_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
siteverify is where the server checks Turnstile answers.
*/
const siteverify = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

/*
withTurnstile is a test case on an open server that asks for Turnstile,
and the fake standing in for Cloudflare.
*/
func withTurnstile(t *testing.T) (*tests.TestCase, *httpclienttest.Fake) {
	t.Helper()

	tc := tests.New(t, tests.WithEnv(
		"DIMMI_AUTH__SIGNUP__MODE=open", "DIMMI_AUTH__CAPTCHA__DRIVER=turnstile",
		"DIMMI_AUTH__CAPTCHA__SITE_KEY=site-key", "DIMMI_AUTH__CAPTCHA__SECRET_KEY=secret-key",
	))

	return tc, httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
}

/*
answering stubs the provider's verdict on every answer.
*/
func answering(fake *httpclienttest.Fake, status int, body string) {
	fake.Stub("POST "+siteverify, httpclient.Response{Status: status, Body: []byte(body)})
}

/*
solved is a registration body carrying a captcha answer.
*/
func solved(email string) testkit.JSON {
	body := registration(email)
	body["captcha_token"] = "answer"

	return body
}

func TestCaptcha(t *testing.T) {
	t.Parallel()

	t.Run("tells the browser which challenge to show", func(t *testing.T) {
		t.Parallel()
		tc, _ := withTurnstile(t)

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.captcha.provider", "turnstile").
			AssertJSONPath("data.captcha.site_key", "site-key")
	})

	t.Run("signs up only with an answer the provider accepts", func(t *testing.T) {
		t.Parallel()
		tc, fake := withTurnstile(t)

		tc.Client().PostJSON("/api/v1/users", registration("a@example.com")).
			AssertValidationError("captcha_token").AssertJSONPath("code", "captcha_failed")

		answering(fake, fhttp.StatusOK, `{"success":false}`)
		tc.Client().PostJSON("/api/v1/users", solved("a@example.com")).AssertValidationError("captcha_token")

		answering(fake, fhttp.StatusOK, `{"success":true}`)
		tc.Client().PostJSON("/api/v1/users", solved("b@example.com")).AssertCreated()
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(string(req.Body), `"secret":"secret-key"`) &&
				strings.Contains(string(req.Body), `"response":"answer"`)
		})
	})

	t.Run("refuses to sign up while the provider cannot tell", func(t *testing.T) {
		t.Parallel()

		for name, verdict := range map[string]struct {
			status int
			body   string
		}{"down": {fhttp.StatusServiceUnavailable, ""}, "garbled": {fhttp.StatusOK, "not json"}} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc, fake := withTurnstile(t)
				answering(fake, verdict.status, verdict.body)

				tc.Client().PostJSON("/api/v1/users", solved("a@example.com")).
					AssertStatus(fhttp.StatusServiceUnavailable).AssertJSONPath("code", "captcha_unavailable")
				tc.Table("users").AssertMissing(testkit.Row{"email": "a@example.com"})
			})
		}
	})

	t.Run("asks before mailing a sign-in link", func(t *testing.T) {
		t.Parallel()
		tc, fake := withTurnstile(t)

		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}).
			AssertValidationError("captcha_token")

		answering(fake, fhttp.StatusOK, `{"success":true}`)
		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email, "captcha_token": "answer"}).
			AssertStatus(fhttp.StatusAccepted)
	})

	t.Run("asks nothing on a server without a captcha", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=open"))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().AssertJSONPath("data.captcha", nil)
		tc.Client().PostJSON("/api/v1/users", registration("a@example.com")).AssertCreated()
	})

	t.Run("refuses to boot with turnstile but no keys", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t, tests.WithEnv("DIMMI_AUTH__CAPTCHA__DRIVER=turnstile"))
		require.ErrorContains(t, err, "auth.captcha")
	})
}
