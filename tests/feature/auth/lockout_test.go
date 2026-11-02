package auth_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
behindProxy trusts the test client's address as a proxy, so each test
can sign in from addresses of its choosing through X-Forwarded-For.
*/
const behindProxy = "DIMMI_HTTP__TRUSTED_PROXIES=192.0.2.0/24"

/*
from returns a client whose requests come from the address.
*/
func from(tc *tests.TestCase, ip string) *testkit.Client {
	return tc.Client().WithHeader("X-Forwarded-For", ip)
}

/*
fail signs in with a wrong password the given number of times.
*/
func fail(t *testing.T, client *testkit.Client, email string, times int) {
	t.Helper()

	for range times {
		client.PostJSON("/api/v1/sessions", login(email, "wrong-password")).AssertValidationError("email")
	}
}

func TestLockout(t *testing.T) {
	t.Parallel()

	t.Run("locks an address out of an account for longer with each run of failures", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_IP_EMAIL=100"))
		client := tc.Client()

		fail(t, client, tc.Owner.Email, 5)
		client.PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(429).
			AssertJSONPath("code", "too_many_attempts").
			AssertHeader("Retry-After", "30")

		tc.Clock.Advance(31 * time.Second)
		fail(t, client, tc.Owner.Email, 5)
		client.PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(429).
			AssertHeader("Retry-After", "300")

		tc.Clock.Advance(5*time.Minute + time.Second)
		fail(t, client, tc.Owner.Email, 5)
		client.PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(429).
			AssertHeader("Retry-After", "1800")

		tc.Clock.Advance(30*time.Minute + time.Second)
		fail(t, client, tc.Owner.Email, 5)
		client.PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(429).
			AssertHeader("Retry-After", "3600")
	})

	t.Run("forgets a run of failures once the right password gets in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_IP_EMAIL=100"))
		client := tc.Client()

		fail(t, client, tc.Owner.Email, 4)
		client.PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertCreated()
		fail(t, client, tc.Owner.Email, 4)

		client.PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertCreated()
	})

	t.Run("records failed and locked sign-ins in the account's security log", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_IP_EMAIL=100"))

		fail(t, tc.Client(), tc.Owner.Email, 5)
		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertStatus(429)

		tc.Table("security_events").
			AssertCount(4, testkit.Row{"user_id": tc.Owner.ID, "type": "login.failed"}).
			AssertCount(2, testkit.Row{"user_id": tc.Owner.ID, "type": "login.locked"})
	})

	t.Run("caps the attempts one address makes at one account each minute", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_IP_EMAIL=3"))

		fail(t, tc.Client(), tc.Owner.Email, 3)

		tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(429).
			AssertJSONPath("code", "too_many_attempts")
		from(tc, "198.51.100.9").PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(429)
	})

	t.Run("locks an account for strangers but lets in a browser that signed in before", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(behindProxy, "DIMMI_AUTH__LOCKOUT__PER_EMAIL_SOFT_LOCK=3"))
		device := tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			Cookie(tc.App.Cookies.DeviceName())

		for _, ip := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.3"} {
			fail(t, from(tc, ip), tc.Owner.Email, 1)
		}

		from(tc, "198.51.100.4").PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(429)
		from(tc, "198.51.100.4").WithCookie(device).
			PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertCreated()
	})

	t.Run("blocks an address that tries one password across many accounts", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(behindProxy, "DIMMI_AUTH__LOCKOUT__PER_IP_SPRAY=3"))
		sprayer := from(tc, "203.0.113.7")

		for _, email := range []string{"a@example.com", "b@example.com", "c@example.com"} {
			fail(t, sprayer, email, 1)
		}

		sprayer.PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertStatus(429).
			AssertHeader("Retry-After", "3600")
		from(tc, "203.0.113.8").PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertCreated()
	})

	t.Run("answers a locked unknown email exactly like a locked account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__LOCKOUT__PER_IP_EMAIL=100"))

		fail(t, tc.Client(), tc.Owner.Email, 5)
		fail(t, tc.Client(), "nobody@example.com", 5)

		known := tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, "x")).AssertStatus(429)
		unknown := tc.Client().PostJSON("/api/v1/sessions", login("nobody@example.com", "x")).AssertStatus(429)
		if !bytes.Equal(known.Body, unknown.Body) {
			t.Errorf("locked answers differ:\n%s\n%s", known.Body, unknown.Body)
		}
	})
}
