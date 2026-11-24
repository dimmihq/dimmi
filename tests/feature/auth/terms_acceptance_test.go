package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
browserAgent is the User-Agent the terms tests sign up with.
*/
const browserAgent = "Mozilla/5.0 (Macintosh) Terms/1.0"

/*
publishedTerms makes the install publish legal terms, as the cloud does.
*/
const publishedTerms = "DIMMI_AUTH__TERMS__SITE_URL=https://legal.test/"

/*
termsMessage is what a sign-up that did not accept the terms is told.
*/
const termsMessage = "Accept the terms to create your account."

func TestTermsAcceptance(t *testing.T) {
	t.Parallel()

	t.Run("records the version, the time, the address and the browser of a sign-up that accepts", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		body := registration("grace@example.com")

		tc.Client().WithHeader("User-Agent", browserAgent).PostJSON("/api/v1/users", body).AssertCreated()

		assertAccepted(t, tc, userByEmail(t, tc, "grace@example.com"), "1.0")
	})

	t.Run("records the version the install has in force", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__TERMS__VERSION=2.3"))
		body := registration("grace@example.com")

		tc.Client().WithHeader("User-Agent", browserAgent).PostJSON("/api/v1/users", body).AssertCreated()

		assertAccepted(t, tc, userByEmail(t, tc, "grace@example.com"), "2.3")
	})

	t.Run("refuses a sign-up that does not accept the terms the install publishes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(publishedTerms))
		silent := registration("grace@example.com")
		delete(silent, "terms_accepted")
		declined := registration("ada@example.com")
		declined["terms_accepted"] = false

		tc.Client().PostJSON("/api/v1/users", silent).AssertValidationError("terms_accepted").
			AssertJSONPath("message", termsMessage)
		tc.Client().PostJSON("/api/v1/users", declined).AssertValidationError("terms_accepted").
			AssertJSONPath("message", termsMessage)

		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
		tc.Table("users").AssertCount(0, testkit.Row{"email": "ada@example.com"})
	})

	t.Run("creates the account of a sign-up that accepts the terms the install publishes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(publishedTerms))

		tc.Client().WithHeader("User-Agent", browserAgent).
			PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated()

		assertAccepted(t, tc, userByEmail(t, tc, "grace@example.com"), "1.0")
	})

	t.Run("refuses an invitee who does not accept the terms the install publishes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(publishedTerms))
		silent := invitee(inviteGrace(t, tc, time.Hour))
		delete(silent, "terms_accepted")

		tc.Client().PostJSON("/api/v1/users", silent).AssertValidationError("terms_accepted").
			AssertJSONPath("message", termsMessage)
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})

		silent["terms_accepted"] = true
		tc.Client().PostJSON("/api/v1/users", silent).AssertCreated()
	})

	t.Run("records the acceptance of an invitee who creates their account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		accepting := invitee(inviteGrace(t, tc, time.Hour))

		tc.Client().WithHeader("User-Agent", browserAgent).PostJSON("/api/v1/users", accepting).AssertCreated()

		user := userByEmail(t, tc, "grace@example.com")
		assertAccepted(t, tc, user, "1.0")
		tc.Table("workspace_members").AssertHas(testkit.Row{"user_id": user.ID, "joined_via": "invitation"})
	})

	t.Run("carries an acceptance through a provider sign-up", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.Client().WithHeader("User-Agent", browserAgent)
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?terms_accepted=true")

		assertLands(t, tc.FinishOAuth(browser, attempt, person("583231", "grace@example.com")), "/")

		assertAccepted(t, tc, userByEmail(t, tc, "grace@example.com"), "1.0")
	})

	t.Run("creates no account for a provider sign-up that did not accept published terms", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv(publishedTerms))
		tc.Social()
		browser := tc.Client()
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?terms_accepted=false")

		assertLands(t, tc.FinishOAuth(browser, attempt, person("583231", "grace@example.com")),
			"/login?error=terms_not_accepted")

		tc.Table("users").AssertMissing(testkit.Row{"email": "grace@example.com"})
	})

	t.Run("tells the sign-up page where the terms are when the install publishes them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(publishedTerms))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.terms", testkit.JSON{
				"version":             "1.0",
				"terms_url":           "https://legal.test/terms",
				"privacy_url":         "https://legal.test/privacy",
				"data_processing_url": "https://legal.test/privacy#dpa",
			})
	})
}

/*
invitee is the sign-up body of grace@example.com answering the
invitation and accepting the legal terms.
*/
func invitee(token string) testkit.JSON {
	return testkit.JSON{
		"name": "Grace Hopper", "email": "grace@example.com", "password": factories.Password,
		"invitation_token": token, "terms_accepted": true,
	}
}

/*
assertAccepted checks the account holds an acceptance of the version,
given now from the terms tests' browser.
*/
func assertAccepted(t *testing.T, tc *tests.TestCase, user models.User, version string) {
	t.Helper()

	require.NotNil(t, user.TermsVersion)
	require.NotNil(t, user.TermsAcceptedAt)
	require.NotNil(t, user.TermsAcceptedIp)
	require.NotNil(t, user.TermsAcceptedUserAgent)
	assert.Equal(t, version, *user.TermsVersion)
	assert.Equal(t, tc.Clock.Now(), user.TermsAcceptedAt.UTC())
	assert.True(t, user.TermsAcceptedIp.IsValid())
	assert.Equal(t, browserAgent, *user.TermsAcceptedUserAgent)
}
