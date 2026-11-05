package accesspolicy_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestSessionLength(t *testing.T) {
	t.Parallel()

	t.Run("a session older than the workspace limit must sign in again", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"session_max_age_minutes": 60})

		tc.Clock.Advance(61 * time.Minute)

		owner.GetJSON("/api/v1/workspace").AssertForbidden().
			AssertJSONPath("code", "reauthentication_required").
			AssertJSONPath("requirement", "reauthentication_required").
			AssertJSONPath("action_url", "/login?reauth=1").
			AssertJSONPath("reason", "max_age")
	})

	t.Run("a session within the limit keeps working", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"session_max_age_minutes": 60})

		tc.Clock.Advance(59 * time.Minute)

		owner.GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("signing in again on the same session works straight away", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"session_max_age_minutes": 60})
		tc.Clock.Advance(61 * time.Minute)
		owner.GetJSON("/api/v1/workspace").AssertForbidden()

		res := owner.PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": tc.Owner.Email, "password": factories.Password, "reauth": true,
		}).AssertCreated()

		working(tc, tc.Client().WithCookie(res.Cookie(tc.App.Session.CookieName()))).
			GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("another workspace of the same person is not affected", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		browser := tc.ActingAs(tc.Owner)
		strict := models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
		lax := models.PublicID(models.PrefixWorkspace, other.Workspace.ID)
		ruled(t, tc, map[string]any{"session_max_age_minutes": 60})

		tc.Clock.Advance(2 * time.Hour)

		browser.WithHeader("Dimmi-Workspace", strict).GetJSON("/api/v1/workspace").AssertForbidden()
		browser.WithHeader("Dimmi-Workspace", lax).GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("an idle session must sign in again, and asking who is signed in does not wake it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"session_idle_timeout_minutes": 15})
		owner.GetJSON("/api/v1/workspace").AssertOK()

		tc.Clock.Advance(16 * time.Minute)

		owner.GetJSON("/api/v1/me").AssertOK()
		owner.GetJSON("/api/v1/workspace").AssertForbidden().AssertJSONPath("reason", "idle")
		owner.GetJSON("/api/v1/workspace").AssertForbidden().AssertJSONPath("reason", "idle")
	})

	t.Run("working keeps a session from going idle", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"session_idle_timeout_minutes": 15})

		for range 4 {
			tc.Clock.Advance(10 * time.Minute)
			owner.GetJSON("/api/v1/workspace").AssertOK()
		}
	})

	t.Run("a token ignores session length", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		token := tc.ActingAsToken(tc.Workspace.Owner, "settings:read")
		ruled(t, tc, map[string]any{"session_max_age_minutes": 60, "session_idle_timeout_minutes": 15})

		tc.Clock.Advance(90 * 24 * time.Hour)

		token.GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("a limit above what the install allows is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 43200}).AssertOK()
		owner.PatchJSON(policyPath, testkit.JSON{"session_idle_timeout_minutes": 20160}).AssertOK()
		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 59}).AssertStatus(422)
		owner.PatchJSON(policyPath, testkit.JSON{"session_idle_timeout_minutes": 14}).AssertStatus(422)
		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 43201}).AssertStatus(422)

		tight := tests.New(t, tests.WithEnv("DIMMI_AUTH__SESSIONS__ABSOLUTE_LIFETIME=48h",
			"DIMMI_AUTH__SESSIONS__IDLE_LIFETIME=24h"))
		narrow := in(tight, tight.Owner)
		narrow.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 49 * 60}).
			AssertValidationError("session_max_age_minutes")
		narrow.PatchJSON(policyPath, testkit.JSON{"session_idle_timeout_minutes": 25 * 60}).
			AssertValidationError("session_idle_timeout_minutes")
		narrow.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 47 * 60}).AssertOK()
	})

	t.Run("a limit sent as null is cleared and a missing one stays", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 720, "session_idle_timeout_minutes": 60}).
			AssertOK().AssertJSONPath("data.session_max_age_minutes", 720.0)
		owner.PatchJSON(policyPath, testkit.JSON{"session_idle_timeout_minutes": nil}).AssertOK().
			AssertJSONPath("data.session_max_age_minutes", 720.0).
			AssertJSONPath("data.session_idle_timeout_minutes", nil)
	})

	t.Run("damaged settings ask nothing", func(t *testing.T) {
		t.Parallel()
		w := models.Workspace{Settings: json.RawMessage(`{"security": "not a document"}`)}

		assert.Equal(t, models.SecuritySettings{}, w.Security())
		assert.False(t, models.Workspace{Settings: json.RawMessage(`{not json`)}.RequiresTwoFactor())
		assert.False(t, models.Workspace{}.RequiresTwoFactor())
	})
}
