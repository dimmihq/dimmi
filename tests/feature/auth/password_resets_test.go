package auth_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
newPassword is the password every reset in these tests chooses.
*/
const newPassword = "grace-hopper-1906"

/*
resetLink asks for a reset of the email and returns the link's id and
token, read from the mail.
*/
func resetLink(t *testing.T, tc *tests.TestCase, email string) (string, string) {
	t.Helper()

	fake := tc.Mail()
	tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": email}).AssertStatus(fhttp.StatusAccepted)
	sent, _ := tests.LastSent[appmail.ResetPasswordMail](tc, fake)
	link := tc.Fragment(sent.URL)

	return link.Get("id"), link.Get("token")
}

/*
redeem chooses the new password through the reset link, from a client
carrying nothing.
*/
func redeem(client *testkit.Client, id, token string) *testkit.Response {
	return client.PatchJSON("/api/v1/password-resets/"+id, testkit.JSON{"token": token, "password": newPassword})
}

func TestPasswordResetStore(t *testing.T) {
	t.Parallel()

	t.Run("mails a link to an account and answers an unknown email exactly the same", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()

		known := tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)
		unknown := tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": "nobody@example.com"}).
			AssertStatus(fhttp.StatusAccepted)

		assert.True(t, bytes.Equal(known.Body, unknown.Body), "answers differ:\n%s\n%s", known.Body, unknown.Body)
		sent, messages := tests.Sent[appmail.ResetPasswordMail](tc, fake)
		require.Len(t, sent, 1)
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(messages[0]))
		assert.Contains(t, sent[0].URL, "/reset-password#id=prs_")
	})

	t.Run("drops a second request within the minute without saying so", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()

		for range 2 {
			tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}).
				AssertStatus(fhttp.StatusAccepted)
		}

		sent, _ := tests.Sent[appmail.ResetPasswordMail](tc, fake)
		assert.Len(t, sent, 1)
	})

	t.Run("replaces an earlier reset, so only the newest link works", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		oldID, oldToken := resetLink(t, tc, tc.Owner.Email)
		require.NoError(t, tc.App.Limiter.Clear(t.Context(), "reset:minute:"+tc.Owner.Email))
		newID, newToken := resetLink(t, tc, tc.Owner.Email)

		assert.NotEqual(t, oldID, newID)
		redeem(tc.Client(), oldID, oldToken).
			AssertValidationError("token").
			AssertJSONPath("code", "invalid_reset_token")
		redeem(tc.Client(), newID, newToken).AssertOK()
	})
}

func TestPasswordResetUpdate(t *testing.T) {
	t.Parallel()

	t.Run("refuses a new password that is the person's own email or name, keeping the link good", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(func(u models.User) models.User {
			u.Email, u.Name = "correct-horse-battery@example.com", "Staple Battery Horse"

			return u
		})
		id, token := resetLink(t, tc, user.Email)

		for _, personal := range []string{user.Email, strings.ToUpper(user.Email), user.Name} {
			tc.Client().PatchJSON("/api/v1/password-resets/"+id, testkit.JSON{"token": token, "password": personal}).
				AssertValidationError("password").
				AssertJSONPath("code", "password_too_common")
		}

		redeem(tc.Client(), id, token).AssertOK()
	})

	t.Run("ends every access token of the user and counts them in the log", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, plain := tc.MintToken(membershipOf(t, tc, tc.Owner), "*")
		bearer := tc.Client().WithHeader("Authorization", "Bearer "+plain)
		bearer.GetJSON("/api/v1/me").AssertOK()
		id, token := resetLink(t, tc, tc.Owner.Email)

		redeem(tc.Client(), id, token).AssertOK()

		bearer.GetJSON("/api/v1/me").AssertUnauthorized()
		var metadata map[string]any
		err := tc.Operator().QueryRow(t.Context(),
			"SELECT metadata FROM security_events WHERE user_id = $1 AND type = 'password.reset'",
			tc.Owner.ID).Scan(&metadata)
		require.NoError(t, err)
		assert.EqualValues(t, 1, metadata["tokens_revoked"])
	})

	t.Run("sets the new password, ends every session and signs the requester in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		elsewhere := tc.ActingAsOwner()
		id, token := resetLink(t, tc, tc.Owner.Email)
		fake := tc.Mail()

		res := redeem(tc.Client(), id, token).AssertOK()

		res.AssertJSONPath("data.object", "me").
			AssertJSONPath("data.user.email", tc.Owner.Email).
			AssertJSONPath("data.auth.auth_methods", []string{"password_reset"})
		tc.Browser(res).GetJSON("/api/v1/me").AssertOK()
		elsewhere.GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertValidationError("email")
		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, newPassword)).AssertCreated()
		tc.Table("sessions").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "auth_method": "password_reset"})
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "password.reset"})
		tc.Table("password_reset_tokens").AssertCount(0, testkit.Row{"user_id": tc.Owner.ID})
		mail, _ := tests.LastSent[appmail.PasswordChangedMail](tc, fake)
		assert.Equal(t, appmail.PasswordWasReset, mail.Reason)
	})

	t.Run("proves the inbox of an account that never verified it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(factories.Unverified)
		id, token := resetLink(t, tc, user.Email)

		redeem(tc.Client(), id, token).AssertOK().AssertJSONPath("data.user.email_verified", true)
	})

	t.Run("gives an account without a password its first one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(factories.WithoutPassword)
		id, token := resetLink(t, tc, user.Email)

		redeem(tc.Client(), id, token).AssertOK().AssertJSONPath("data.user.has_password", true)
		tc.Client().PostJSON("/api/v1/sessions", login(user.Email, newPassword)).AssertCreated()
	})

	t.Run("refuses a wrong token, an unknown id and a malformed id with one answer", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)

		answers := [][]byte{
			redeem(tc.Client(), id, "wrong-token").AssertValidationError("token").Body,
			redeem(tc.Client(), models.PublicID(models.PrefixPasswordReset, ids.New()), token).
				AssertValidationError("token").Body,
			redeem(tc.Client(), "ott_not_a_reset", token).AssertValidationError("token").Body,
		}

		for _, answer := range answers[1:] {
			assert.True(t, bytes.Equal(answers[0], answer), "answers differ:\n%s\n%s", answers[0], answer)
		}
		assert.Contains(t, string(answers[0]), "invalid_reset_token")
	})

	t.Run("deletes a reset after five wrong tokens", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(behindProxy))
		id, token := resetLink(t, tc, tc.Owner.Email)

		for range appauth.MaxSecretAttempts {
			redeem(tc.Client(), id, "wrong-token").AssertValidationError("token")
		}
		tc.Drain()

		tc.Table("password_reset_tokens").AssertCount(0, testkit.Row{"user_id": tc.Owner.ID})
		redeem(tc.Client(), id, token).AssertValidationError("token")
	})

	t.Run("refuses a link an hour after it was sent", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)
		tc.Clock.Advance(time.Hour)

		redeem(tc.Client(), id, token).AssertValidationError("token").AssertJSONPath("code", "invalid_reset_token")
	})

	t.Run("refuses a link of an account deleted since", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		id, token := resetLink(t, tc, user.Email)
		require.NoError(t, factories.DeleteAccount(t.Context(), tc.App, user))

		redeem(tc.Client(), id, token).AssertValidationError("token")
	})

	t.Run("still works after the password changed some other way, since it proves the inbox", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)
		_, err := tc.Operator().Exec(t.Context(), "UPDATE users SET password_hash = $1 WHERE id = $2",
			*tc.CreateUser().PasswordHash, tc.Owner.ID)
		require.NoError(t, err)

		redeem(tc.Client(), id, token).AssertOK()
	})

	t.Run("works once: a second use of the link is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)
		redeem(tc.Client(), id, token).AssertOK()

		redeem(tc.Client(), id, token).AssertValidationError("token")
	})

	t.Run("refuses a password the rules reject and keeps the link", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := resetLink(t, tc, tc.Owner.Email)

		tc.Client().PatchJSON("/api/v1/password-resets/"+id, testkit.JSON{"token": token, "password": "password123"}).
			AssertValidationError("password")
		redeem(tc.Client(), id, token).AssertOK()
	})

	t.Run("never signs in an account protected by a second factor on the reset alone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		_, err := factories.WithTwoFactor(t.Context(), tc.App, user)
		require.NoError(t, err)
		id, token := resetLink(t, tc, user.Email)

		redeem(tc.Client(), id, token).AssertOK().
			AssertJSONPath("data.object", "two_factor_challenge").
			AssertJSONPath("data.methods.0.method", "totp")
		tc.Table("sessions").AssertCount(0, testkit.Row{"user_id": user.ID})
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": user.ID, "type": "password.reset"})
	})

	t.Run("asks the login policy once the link proved the inbox", func(t *testing.T) {
		t.Parallel()
		policy := &switchablePolicy{}
		tc := tests.New(t, tests.Before(func(a *app.App) { a.LoginPolicy = policy }))
		id, token := resetLink(t, tc, tc.Owner.Email)
		policy.ssoOnly.Store(true)

		redeem(tc.Client(), id, token).AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "sso_required")
		redeem(tc.Client(), id, "wrong-token").AssertValidationError("token")
	})

	t.Run("refuses an address that keeps trying links", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		unknown := models.PublicID(models.PrefixPasswordReset, ids.New())

		for range 10 {
			redeem(tc.Client(), unknown, "x").AssertValidationError("token")
		}

		redeem(tc.Client(), unknown, "x").AssertStatus(fhttp.StatusTooManyRequests)
	})

	t.Run("takes as long for an unknown id as for a wrong token", func(t *testing.T) {
		t.Parallel()
		tests.RequireTimingAccuracy(t)
		tc := tests.New(t, tests.WithEnv(behindProxy))

		unknown, wrong := resetTimings(t, tc)

		tests.AssertSameTiming(t, unknown, wrong, "unknown id", "wrong token")
	})
}

/*
resetTimings redeems an unknown reset id and a real reset with a wrong
token in turn, each from a fresh address so no throttle interferes, and
returns the median time each answer took. The real reset is replaced
before every try, so wrong tokens never burn it.
*/
func resetTimings(t *testing.T, tc *tests.TestCase) (time.Duration, time.Duration) {
	t.Helper()

	durations := [2][]time.Duration{}
	system := clock.System()

	for i := range timingRuns {
		q := models.New(tc.App.DB)
		reset, err := q.ReplacePasswordResetToken(t.Context(), models.ReplacePasswordResetTokenParams{
			ID: ids.New(), UserID: tc.Owner.ID, TokenHash: appauth.Digest("right"),
			ExpiresAt: tc.Clock.Now().Add(time.Hour), Now: tc.Clock.Now(),
		})
		require.NoError(t, err)

		targets := []string{
			models.PublicID(models.PrefixPasswordReset, ids.New()),
			models.PublicID(models.PrefixPasswordReset, reset.ID),
		}
		for n, target := range targets {
			client := from(tc, fmt.Sprintf("198.51.%d.%d", (2*i+n)/250, (2*i+n)%250+1))
			start := system.Now()
			redeem(client, target, "wrong-token").AssertValidationError("token")
			durations[n] = append(durations[n], system.Since(start))
		}
	}

	return median(durations[0]), median(durations[1])
}
