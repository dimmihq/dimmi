package auth_test

import (
	"testing"

	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestCrossOriginProtection(t *testing.T) {
	t.Parallel()

	t.Run("lets the dashboard sign in from its own origin without any token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().WithHeader("Sec-Fetch-Site", "same-origin").
			PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertCreated()
	})

	t.Run("refuses a sign-in another site makes the browser send", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().WithHeader("Sec-Fetch-Site", "cross-site").
			PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertForbidden().
			AssertJSONPath("code", "cross_origin_request")
	})

	t.Run("refuses an unsafe request riding on the session cookie from another site", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().WithHeader("Sec-Fetch-Site", "cross-site").Delete("/api/v1/sessions/current").
			AssertForbidden().
			AssertJSONPath("code", "cross_origin_request")
	})

	t.Run("judges an older browser by its Origin when it sends no Sec-Fetch-Site", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().WithHeader("Origin", "http://example.com").
			PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertCreated()
		tc.Client().WithHeader("Origin", "https://evil.example").
			PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertForbidden()
	})

	t.Run("trusts the application's own origin and the configured dashboard origins", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_APP__URL=https://app.example.com", "DIMMI_HTTP__TRUSTED_ORIGINS=http://localhost:5173",
		))

		for _, origin := range []string{"https://app.example.com", "http://localhost:5173"} {
			tc.Client().WithHeader("Sec-Fetch-Site", "cross-site").WithHeader("Origin", origin).
				PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
				AssertCreated()
		}

		tc.Client().WithHeader("Sec-Fetch-Site", "cross-site").WithHeader("Origin", "http://localhost:3000").
			PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertForbidden()
	})

	t.Run("lets through programs that send neither header", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().Delete("/api/v1/sessions/current").AssertNoContent()
	})

	t.Run("lets through a bearer request even from another site", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner).WithHeader("Sec-Fetch-Site", "cross-site").
			PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).
			AssertCreated()
	})
}
