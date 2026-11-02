package auth_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestSignInFailures(t *testing.T) {
	t.Parallel()

	t.Run("lets nobody in while the attempt counters are out of reach", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakValkey()

		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(503).
			AssertJSONPath("code", "auth_busy").
			AssertHeader("Retry-After", "5")
		tc.Client().PostJSON("/api/v1/users", registration("late@example.com")).
			AssertStatus(503).
			AssertJSONPath("code", "auth_busy")
		tc.Table("sessions").AssertCount(0)
	})

	t.Run("fails a correct password like a wrong one when the stored hash is unreadable", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(func(u models.User) models.User { u.PasswordHash = new("not-a-hash"); return u })

		tc.Client().PostJSON("/api/v1/sessions", login(user.Email, factories.Password)).
			AssertValidationError("email").
			AssertJSONPath("code", "invalid_credentials")
	})

	t.Run("starts no session when the sign-in cannot be logged", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("security_events")

		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(500).
			AssertJSONPath("code", "internal_error").
			AssertJSONPath("message", "Server Error.")
		tc.Table("sessions").AssertCount(0)
	})

	t.Run("does not check a device it cannot look up", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		device := tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			Cookie(tc.App.Cookies.DeviceName())
		tc.BreakTable("security_events")

		tc.Client().WithCookie(device).PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(500)
	})

	t.Run("signs nobody in when accounts cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("users")

		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertStatus(500)
	})

	t.Run("creates no account when its first workspace cannot be created", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("organizations")

		tc.Client().PostJSON("/api/v1/users", registration("atomic@example.com")).AssertStatus(500)

		tc.Table("users").AssertMissing(testkit.Row{"email": "atomic@example.com"})
	})

	t.Run("still refuses a wrong password when the failure cannot be logged", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_IP_EMAIL=100"))
		fail(t, tc.Client(), tc.Owner.Email, 5)
		tc.BreakTable("security_events")

		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, "wrong")).AssertStatus(500)
		tc.Clock.Advance(time.Minute)
		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, "wrong")).AssertStatus(500)
	})

	t.Run("answers a plain 500 when the new session's workspaces cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("workspace_members")

		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertStatus(500)
		solo := registration("solo@example.com")
		delete(solo, "workspace_name")
		tc.Client().PostJSON("/api/v1/users", solo).AssertStatus(500)
		tc.Client().PostJSON("/api/v1/users", registration("team@example.com")).AssertStatus(500)
	})

	t.Run("answers a plain 500 when a sign-up cannot be stored", func(t *testing.T) {
		t.Parallel()
		for _, table := range []string{"users", "workspaces", "security_events"} {
			tc := tests.New(t)
			tc.BreakTable(table)

			tc.Client().PostJSON("/api/v1/users", registration("broken@example.com")).AssertStatus(500)
		}
	})

	t.Run("answers busy when too many passwords are being hashed for sign-ups", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_HASHING__CONCURRENCY=1", "DIMMI_HASHING__QUEUE=1",
			"DIMMI_HASHING__HASHERS__ARGON2ID__MEMORY=32768", "DIMMI_HASHING__HASHERS__ARGON2ID__ITERATIONS=4",
			"DIMMI_AUTH__SIGNUP__MAX_PER_HOUR_PER_IP=100",
		))
		statuses := make([]int, 8)

		var wg sync.WaitGroup
		for i := range statuses {
			wg.Go(func() {
				body := registration(fmt.Sprintf("busy%d@example.com", i))
				statuses[i] = tc.Client().PostJSON("/api/v1/users", body).Status
			})
		}
		wg.Wait()

		assert.Contains(t, statuses, 503)
	})

	t.Run("refuses to re-authenticate on a request without a session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		_, err := tc.App.Actions.AttemptLogin.Handle(t.Context(), auth.AttemptLoginInput{
			Email: tc.Owner.Email, Password: factories.Password, Reauth: true,
		})

		require.Error(t, err)
	})

	t.Run("refuses to start a session for a request that has none", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		_, err := tc.App.Actions.StartSession.Handle(t.Context(), auth.StartSessionInput{
			User: tc.Owner, Methods: []string{appauth.MethodPassword},
		})

		require.Error(t, err)
	})
}
