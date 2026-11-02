package auth_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/tests"
)

func TestUserStore(t *testing.T) {
	t.Parallel()

	t.Run("creates the account, its workspace and organization, and signs the person in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated()

		res.AssertJSONPath("data.object", "me").
			AssertJSONPath("data.user.email", "grace@example.com").
			AssertJSONPath("data.user.email_verified", false).
			AssertJSONPath("data.user.has_password", true).
			AssertJSONPath("data.workspace.name", "Navy").
			AssertJSONPath("data.workspace.slug", "navy").
			AssertJSONPath("data.member.role", "owner").
			AssertJSONPath("data.auth.type", "session").
			AssertJSONPath("data.auth.auth_methods", []string{"password"}).
			AssertJSONPath("data.workspaces.0.slug", "navy").
			AssertJSONMissing("data.workspace.organization_id")
		assert.Contains(t, res.String("data.member.permissions"), "workspace:delete")
		assert.NotEmpty(t, res.Cookie(tc.App.Session.CookieName()).Value)
		assert.NotEmpty(t, res.Cookie(tc.App.Cookies.DeviceName()).Value)
		tc.Table("users").AssertHas(testkit.Row{"email": "grace@example.com", "created_via": "register"})
		tc.Table("organizations").AssertHas(testkit.Row{"name": "Navy"})
		tc.Table("workspaces").AssertHas(testkit.Row{"slug": "navy"})
		tc.Table("workspace_members").AssertHas(testkit.Row{"role": "owner", "joined_via": "created"})
		tc.Table("sessions").AssertHas(testkit.Row{"auth_method": "password"})
	})

	t.Run("signs up without a workspace when none is named", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		body := registration("solo@example.com")
		delete(body, "workspace_name")

		tc.Client().PostJSON("/api/v1/users", body).AssertCreated().
			AssertJSONPath("data.workspace", nil).
			AssertJSONPath("data.member", nil).
			AssertJSONPath("data.workspaces", []any{})
	})

	t.Run("refuses an email already taken, whatever its case", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.Client().PostJSON("/api/v1/users", registration("ada@example.com")).AssertCreated()

		tc.Client().PostJSON("/api/v1/users", registration("ADA@Example.com")).
			AssertValidationError("email").
			AssertJSONPath("code", "email_taken")
	})

	t.Run("leaves exactly one account when two sign-ups race for one email", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		statuses := make([]int, 2)

		var wg sync.WaitGroup
		for i := range statuses {
			wg.Go(func() {
				statuses[i] = tc.Client().PostJSON("/api/v1/users", registration("race@example.com")).Status
			})
		}
		wg.Wait()

		assert.ElementsMatch(t, []int{201, 422}, statuses)
		tc.Table("users").AssertCount(1, testkit.Row{"email": "race@example.com"})
		tc.Table("workspaces").AssertCount(1, testkit.Row{"slug": "navy"})
	})

	t.Run("keeps an address with a plus tag apart from the plain one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.Client().PostJSON("/api/v1/users", registration("ada@example.com")).AssertCreated()

		tc.Client().PostJSON("/api/v1/users", registration("ada+dimmi@example.com")).AssertCreated()
	})

	t.Run("stores the name without invisible characters", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		body := registration("rtl@example.com")
		body["name"] = "  Gra\u200bce \u202eHopper\u0000 "

		tc.Client().PostJSON("/api/v1/users", body).AssertCreated().AssertJSONPath("data.user.name", "Grace Hopper")
	})

	t.Run("refuses a name that is only invisible characters", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		body := registration("blank@example.com")
		body["name"] = "\u200b\u200c"

		tc.Client().PostJSON("/api/v1/users", body).AssertValidationError("name")
	})

	t.Run("names every missing field at once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().PostJSON("/api/v1/users", testkit.JSON{}).
			AssertValidationError("name", "email", "password").
			AssertJSONPath("code", "validation_failed")
	})

	t.Run("refuses passwords that are easy to guess", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		weak := []string{
			"short", "password123", "Password123", "1qaz2wsx3edc", "IloveYou123", "weak@example.com", "Grace Hopper",
		}

		for _, password := range weak {
			body := registration("weak@example.com")
			body["password"] = password

			tc.Client().PostJSON("/api/v1/users", body).AssertValidationError("password")
		}
	})

	t.Run("names the password field in plain words when refusing it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		body := registration("common@example.com")
		body["password"] = "password123"

		tc.Client().PostJSON("/api/v1/users", body).
			AssertValidationError("password").
			AssertJSONPath("message", "The password is too common. Choose another one.")
	})

	t.Run("counts a password's length in characters, not bytes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		short := registration("bytes@example.com")
		short["password"] = "ééééééééé"
		long := registration("bytes@example.com")
		long["password"] = "éééééééééé"

		tc.Client().PostJSON("/api/v1/users", short).AssertValidationError("password")
		tc.Client().PostJSON("/api/v1/users", long).AssertCreated()
	})

	t.Run("refuses strangers when signup is by invitation only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))

		tc.Client().PostJSON("/api/v1/users", registration("stranger@example.com")).
			AssertForbidden().
			AssertJSONPath("code", "signup_closed")
		tc.Table("users").AssertMissing(testkit.Row{"email": "stranger@example.com"})
	})

	t.Run("starts a self-hosted install closed and the cloud open", func(t *testing.T) {
		t.Parallel()
		selfHosted := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE="))
		cloud := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=", "DIMMI_APP__EDITION=cloud"))

		selfHosted.Client().PostJSON("/api/v1/users", registration("a@example.com")).AssertForbidden()
		cloud.Client().PostJSON("/api/v1/users", registration("a@example.com")).AssertCreated()
	})

	t.Run("caps the accounts one address creates in an hour", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MAX_PER_HOUR_PER_IP=2"))

		tc.Client().PostJSON("/api/v1/users", registration("one@example.com")).AssertCreated()
		tc.Client().PostJSON("/api/v1/users", registration("two@example.com")).AssertCreated()
		res := tc.Client().PostJSON("/api/v1/users", registration("three@example.com"))

		res.AssertStatus(429).AssertJSONPath("code", "too_many_attempts")
		assert.NotEmpty(t, res.Header.Get("Retry-After"))
	})

	t.Run("replaces the session of someone already signed in on the browser", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		signedIn := tc.ActingAsOwner()
		previous := signedIn.GetJSON("/api/v1/me").AssertOK().String("data.auth.session_id")

		res := signedIn.PostJSON("/api/v1/users", registration("newcomer@example.com")).AssertCreated()

		assert.NotEqual(t, previous, res.String("data.auth.session_id"))
		signedIn.GetJSON("/api/v1/me").AssertUnauthorized()
	})

	t.Run("gives a repeated workspace name the next free slug", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.Client().PostJSON("/api/v1/users", registration("first@example.com")).AssertCreated()

		tc.Client().PostJSON("/api/v1/users", registration("second@example.com")).AssertCreated().
			AssertJSONPath("data.workspace.slug", "navy-2")
	})

	t.Run("never hands out a slug that reads as part of the product", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		for name, slug := range map[string]string{"Admin": "admin-workspace", "QA": "qa-workspace", "🚀": "workspace"} {
			body := registration(slug + "@example.com")
			body["workspace_name"] = name

			tc.Client().PostJSON("/api/v1/users", body).AssertCreated().AssertJSONPath("data.workspace.slug", slug)
		}
	})

	t.Run("keeps a slug from a long name within forty characters, suffix included", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		long := "The Analytical Engine Weaves Algebraic Patterns Like Looms"

		for i, want := range []string{
			"the-analytical-engine-weaves-algebraic-p", "the-analytical-engine-weaves-algebraic-2",
		} {
			body := registration(fmt.Sprintf("long%d@example.com", i))
			body["workspace_name"] = long

			tc.Client().PostJSON("/api/v1/users", body).AssertCreated().AssertJSONPath("data.workspace.slug", want)
		}
	})

	t.Run("asks the login policy before anything else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.LoginPolicy = ssoOnly{} }))

		tc.Client().PostJSON("/api/v1/users", registration("corp@example.com")).
			AssertStatus(409).
			AssertJSONPath("code", "sso_required").
			AssertJSONPath("redirect_url", "https://sso.example.com/start")
		tc.Table("users").AssertMissing(testkit.Row{"email": "corp@example.com"})
	})

	t.Run("fails closed when the login policy cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.LoginPolicy = brokenPolicy{} }))

		res := tc.Client().PostJSON("/api/v1/users", registration("corp@example.com"))

		res.AssertStatus(503).AssertJSONPath("code", "auth_busy").AssertHeader("Retry-After", "1")
		tc.Table("users").AssertMissing(testkit.Row{"email": "corp@example.com"})
	})
}
