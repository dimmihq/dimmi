package auth_test

import (
	"bytes"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
verificationLink signs grace@example.com up and returns the browser,
the verification link's id and its token, read from the mail.
*/
func verificationLink(t *testing.T, tc *tests.TestCase) (*testkit.Client, string, string) {
	t.Helper()

	fake := tc.Mail()
	browser := tc.Browser(tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated())
	sent, _ := tests.LastSent[appmail.VerifyEmailMail](tc, fake)
	link := tc.Fragment(sent.URL)

	return browser, link.Get("id"), link.Get("token")
}

/*
verify redeems a verification link with a client carrying nothing.
*/
func verify(tc *tests.TestCase, id, token string) *testkit.Response {
	return verifyIn(tc.Client(), id, token)
}

/*
verifyIn redeems a verification link in the browser.
*/
func verifyIn(browser *testkit.Client, id, token string) *testkit.Response {
	return browser.PatchJSON("/api/v1/email-verifications/"+id, testkit.JSON{"token": token})
}

/*
disown redeems a verification link with a client carrying nothing,
saying the account the link names is not theirs.
*/
func disown(tc *tests.TestCase, id, token string) *testkit.Response {
	return tc.Client().PatchJSON("/api/v1/email-verifications/"+id, testkit.JSON{"token": token, "claim": true})
}

func TestEmailVerificationUpdate(t *testing.T) {
	t.Parallel()

	t.Run("proves the address from the browser signed in to the account, keeping everything", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser, id, token := verificationLink(t, tc)

		res := verifyIn(browser, id, token).AssertOK()

		res.AssertJSONPath("data.object", "email_verification").
			AssertJSONPath("data.id", id).
			AssertJSONPath("data.email", "grace@example.com").
			AssertJSONPath("data.status", "verified").
			AssertJSONPath("data.verified_at", tests.Epoch().Format(time.RFC3339))
		assert.Contains(t, res.String("data.user_id"), "usr_")
		browser.GetJSON("/api/v1/me").
			AssertJSONPath("data.user.email_verified", true).
			AssertJSONPath("data.user.has_password", true)
		tc.Client().PostJSON("/api/v1/sessions", login("grace@example.com", factories.Password)).AssertCreated()
	})

	t.Run("asks another device for a choice, verifying nothing and spending nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser, id, token := verificationLink(t, tc)

		verify(tc, id, token).AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "verification_needs_choice")

		browser.GetJSON("/api/v1/me").
			AssertJSONPath("data.user.email_verified", false).
			AssertJSONPath("data.user.has_password", true)
		verifyIn(browser, id, token).AssertOK()
	})

	t.Run("lets the registrant finish on another device by signing in there first", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)
		phone := tc.Browser(tc.Client().
			PostJSON("/api/v1/sessions", login("grace@example.com", factories.Password)).AssertCreated())

		verifyIn(phone, id, token).AssertOK()

		phone.GetJSON("/api/v1/me").AssertJSONPath("data.user.has_password", true)
	})

	t.Run("asks for a choice while another user is signed in, and claims the named account on it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)
		owner := tc.ActingAsOwner()

		verifyIn(owner, id, token).AssertStatus(fhttp.StatusConflict)

		owner.PatchJSON("/api/v1/email-verifications/"+id, testkit.JSON{"token": token, "claim": true}).AssertOK().
			AssertJSONPath("data.email", "grace@example.com")
		owner.GetJSON("/api/v1/me").AssertJSONPath("data.user.email", tc.Owner.Email)
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com", "email_verified_at": nil})
	})

	t.Run("answers that the address is verified on a second click, even once the link ran out", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser, id, token := verificationLink(t, tc)
		verifyIn(browser, id, token).AssertOK()
		tc.Clock.Advance(48 * time.Hour)

		verify(tc, id, token).AssertOK().
			AssertJSONPath("data.status", "verified").
			AssertJSONPath("data.verified_at", tests.Epoch().Format(time.RFC3339))
	})

	t.Run("refuses a wrong token, an unknown id and the id of another kind of secret alike", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)
		other, err := tc.App.Secrets.Issue(t.Context(), appauth.Issue{
			Purpose: enums.OneTimePurposeMagicLink, Email: "grace@example.com", TTL: time.Hour,
		})
		require.NoError(t, err)

		answers := [][]byte{
			verify(tc, id, "wrong-token").AssertValidationError("token").Body,
			verify(tc, "ott_01jc0000000000000000000000", token).AssertValidationError("token").Body,
			verify(tc, models.PublicID(models.PrefixOneTimeToken, other.ID), other.Token).
				AssertValidationError("token").Body,
			verify(tc, "not-an-id", token).AssertValidationError("token").Body,
		}

		for _, answer := range answers[1:] {
			assert.True(t, bytes.Equal(answers[0], answer), "answers differ:\n%s\n%s", answers[0], answer)
		}
		assert.Contains(t, string(answers[0]), "invalid_verification_token")
	})

	t.Run("burns a link after five wrong tokens", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)

		for range appauth.MaxSecretAttempts {
			verify(tc, id, "wrong-token").AssertValidationError("token")
		}

		verify(tc, id, token).AssertValidationError("token").AssertJSONPath("code", "invalid_verification_token")
	})

	t.Run("calls a link stale once the account moved to another address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)
		_, err := tc.App.DB.Exec(t.Context(),
			"UPDATE users SET email = 'hopper@example.com' WHERE email = 'grace@example.com'")
		require.NoError(t, err)

		verify(tc, id, token).AssertStatus(fhttp.StatusGone).AssertJSONPath("code", "verification_link_stale")
	})

	t.Run("calls a link expired after a day when the account is still unverified", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)
		tc.Clock.Advance(24*time.Hour + time.Second)

		verify(tc, id, token).AssertStatus(fhttp.StatusGone).AssertJSONPath("code", "verification_link_expired")
	})

	t.Run("refuses the link of a deleted account like an unknown one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)
		require.NoError(t, factories.DeleteAccount(t.Context(), tc.App, userByEmail(t, tc, "grace@example.com")))

		verify(tc, id, token).AssertValidationError("token").AssertJSONPath("code", "invalid_verification_token")
	})

	t.Run("keeps only the three newest links working", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		user := tc.CreateUser(factories.Unverified)
		for range 4 {
			require.NoError(t, tc.App.Actions.SendEmailVerification.Handle(t.Context(), user, appauth.Client{}))
			tc.Clock.Advance(time.Second)
		}
		sent, _ := tests.Sent[appmail.VerifyEmailMail](tc, fake)
		require.Len(t, sent, 4)
		links := make([]url.Values, 0, len(sent))
		for _, m := range sent {
			links = append(links, tc.Fragment(m.URL))
		}
		slices.SortFunc(links, func(a, b url.Values) int { return strings.Compare(a.Get("id"), b.Get("id")) })

		oldest, newer := links[0], links[1]

		verify(tc, oldest.Get("id"), oldest.Get("token")).
			AssertStatus(fhttp.StatusGone).
			AssertJSONPath("code", "verification_link_expired")
		verifyIn(tc.ActingAs(user), newer.Get("id"), newer.Get("token")).AssertOK()
	})

	t.Run("refuses an address that keeps trying links", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		for range 10 {
			verify(tc, "ott_01jc0000000000000000000000", "x").AssertValidationError("token")
		}

		verify(tc, "ott_01jc0000000000000000000000", "x").
			AssertStatus(fhttp.StatusTooManyRequests).
			AssertJSONPath("code", "too_many_attempts")
	})

	t.Run("reads the token from the body only, never from the address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, id, token := verificationLink(t, tc)

		tc.Client().PatchJSON("/api/v1/email-verifications/"+id+"?token="+token, testkit.JSON{}).
			AssertValidationError("token")
	})
}

func TestEmailVerificationStore(t *testing.T) {
	t.Parallel()

	t.Run("mails a signed-in unverified user a new link, at most once a minute", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser, _, _ := verificationLink(t, tc)
		fake := tc.Mail()
		signUpMail, _ := tests.Sent[appmail.VerifyEmailMail](tc, fake)

		browser.PostJSON("/api/v1/email-verifications", nil).
			AssertStatus(fhttp.StatusAccepted).
			AssertJSONPath("data.message", "A new verification link is on its way.")
		limited := browser.PostJSON("/api/v1/email-verifications", nil).
			AssertStatus(fhttp.StatusTooManyRequests).
			AssertJSONPath("code", "too_many_attempts")
		assert.Contains(t, limited.String("message"), "Too many attempts. Try again in")

		sent, _ := tests.Sent[appmail.VerifyEmailMail](tc, fake)
		assert.Len(t, sent, len(signUpMail)+1)
	})

	t.Run("sends nothing to a user whose address is already verified", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()

		tc.ActingAsOwner().PostJSON("/api/v1/email-verifications", nil).
			AssertOK().
			AssertJSONPath("data.message", "Your email address is already verified.")

		sent, _ := tests.Sent[appmail.VerifyEmailMail](tc, fake)
		assert.Empty(t, sent)
	})

	t.Run("refuses a caller without a session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().PostJSON("/api/v1/email-verifications", nil).AssertUnauthorized()
	})
}

/*
userByEmail returns the live account with the email.
*/
func userByEmail(t *testing.T, tc *tests.TestCase, email string) models.User {
	t.Helper()

	user, err := models.New(tc.App.DB).FindUserByEmail(t.Context(), email)
	require.NoError(t, err)

	return user
}
