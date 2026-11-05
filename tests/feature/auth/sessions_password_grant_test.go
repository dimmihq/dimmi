package auth_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestSessionStoreWithPassword(t *testing.T) {
	t.Parallel()

	t.Run("signs in, records how, and hands out a device cookie", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertCreated()

		res.AssertJSONPath("data.user.email", tc.Owner.Email).
			AssertJSONPath("data.auth.type", "session").
			AssertJSONPath("data.auth.auth_methods", []string{"password"}).
			AssertJSONPath("data.auth.two_factor_used", false).
			AssertJSONPath("data.auth.authenticated_at", "2026-11-02T09:00:00Z").
			AssertJSONPath("data.workspace", nil).
			AssertJSONPath("data.workspaces.0.role", "owner")
		device := res.Cookie(tc.App.Cookies.DeviceName()).Value
		tc.Table("sessions").AssertHas(testkit.Row{
			"user_id": tc.Owner.ID, "auth_method": "password", "confirmed_method": "password",
			"device_id": hashing.SHA256Hex([]byte(device)),
		})
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "login.succeeded"})
	})

	t.Run("keeps the device cookie a browser already holds", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		first := tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password))
		device := first.Cookie(tc.App.Cookies.DeviceName())

		again := tc.Client().WithCookie(device).PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password))

		again.AssertCreated()
		assert.NotContains(t, strings.Join(again.Header.Values("Set-Cookie"), ";"), tc.App.Cookies.DeviceName())
		tc.Table("sessions").AssertCount(2, testkit.Row{"device_id": hashing.SHA256Hex([]byte(device.Value))})
	})

	t.Run("answers an unknown email, a passwordless account and a wrong password alike", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		passwordless := tc.CreateUser(factories.WithoutPassword)

		for _, body := range []testkit.JSON{
			login("nobody@example.com", factories.Password),
			login(passwordless.Email, factories.Password),
			login(tc.Owner.Email, "not-the-password"),
		} {
			tc.Client().PostJSON("/api/v1/sessions", body).
				AssertValidationError("email").
				AssertJSONPath("code", "invalid_credentials").
				AssertJSONPath("message", "These credentials do not match our records.")
		}
	})

	t.Run("signs in whatever the case of the email", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().PostJSON("/api/v1/sessions", login(strings.ToUpper(tc.Owner.Email), factories.Password)).
			AssertCreated()
	})

	t.Run("refuses an unknown grant and missing fields without counting a failure", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{"grant": "telepathy"}).AssertValidationError("grant")
		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{}).AssertValidationError("grant")
		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{"grant": "password"}).
			AssertValidationError("email", "password")
		tc.Table("security_events").AssertMissing(testkit.Row{"type": "login.failed"})
	})

	t.Run("replaces a hash made with older settings on the next sign-in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		legacy := legacyHash(t, tc.App)
		user := tc.CreateUser(func(u models.User) models.User { u.PasswordHash = &legacy; return u })

		tc.Client().PostJSON("/api/v1/sessions", login(user.Email, factories.Password)).AssertCreated()

		stored, err := models.New(tc.App.DB).FindUser(t.Context(), user.ID)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(*stored.PasswordHash, "$argon2id$"))
		tc.Client().PostJSON("/api/v1/sessions", login(user.Email, factories.Password)).AssertCreated()
	})

	t.Run("gives two tabs signing in at once a session each on one device", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		device := tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			Cookie(tc.App.Cookies.DeviceName())

		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() {
				tc.Client().WithCookie(device).PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password))
			})
		}
		wg.Wait()

		tc.Table("sessions").AssertCount(3, testkit.Row{"device_id": hashing.SHA256Hex([]byte(device.Value))})
	})

	t.Run("asks the login policy before checking any password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.LoginPolicy = ssoOnly{} }))

		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, "wrong")).
			AssertStatus(409).
			AssertJSONPath("code", "sso_required")
		tc.Table("security_events").AssertCount(0)
	})

	t.Run("answers busy when too many passwords are being checked at once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_HASHING__CONCURRENCY=1", "DIMMI_HASHING__QUEUE=1",
			"DIMMI_HASHING__HASHERS__ARGON2ID__MEMORY=32768", "DIMMI_HASHING__HASHERS__ARGON2ID__ITERATIONS=4",
		))
		statuses := make([]int, 8)

		var wg sync.WaitGroup
		for i := range statuses {
			wg.Go(func() {
				statuses[i] = tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, "wrong")).Status
			})
		}
		wg.Wait()

		assert.Contains(t, statuses, 503)
	})
}

func TestReauthentication(t *testing.T) {
	t.Parallel()

	t.Run("gives the same user a fresh session with a new sign-in time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		before := browser.GetJSON("/api/v1/me").String("data.auth.session_id")
		tc.Clock.Advance(10 * time.Minute)

		body := login(tc.Owner.Email, factories.Password)
		body["reauth"] = true
		res := browser.PostJSON("/api/v1/sessions", body).AssertCreated()

		assert.NotEqual(t, before, res.String("data.auth.session_id"))
		res.AssertJSONPath("data.auth.authenticated_at", "2026-11-02T09:10:00Z")
		tc.Table("sessions").AssertCount(1, testkit.Row{"user_id": tc.Owner.ID})
		browser.GetJSON("/api/v1/me").AssertUnauthorized()
	})

	t.Run("refuses another user's password and keeps the session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tc.CreateUser()
		browser := tc.ActingAsOwner()

		body := login(other.Email, factories.Password)
		body["reauth"] = true
		browser.PostJSON("/api/v1/sessions", body).
			AssertValidationError("email").
			AssertJSONPath("code", "reauth_user_mismatch")

		browser.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.user.email", tc.Owner.Email)
		tc.Drain()
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": other.ID, "type": "login.failed"})
	})

	t.Run("signs in normally when there is no session to re-authenticate", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		body := login(tc.Owner.Email, factories.Password)
		body["reauth"] = true
		tc.Client().PostJSON("/api/v1/sessions", body).AssertCreated()
	})
}

/*
legacyHash returns the factory password hashed with bcrypt, the way an
older release stored it.
*/
func legacyHash(t *testing.T, a *app.App) string {
	t.Helper()

	bcrypt, err := a.Hash.Hasher("bcrypt")
	require.NoError(t, err)

	hash, err := bcrypt.Make(t.Context(), factories.Password)
	require.NoError(t, err)

	return hash
}
