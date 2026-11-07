package visitoridentity_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
setMode switches the workspace's sign-in mode through the API.
*/
func (d *desk) setMode(body testkit.JSON) *testkit.Response {
	d.t.Helper()
	return d.owner.PatchJSON("/api/v1/identity-verification/settings", body)
}

/*
seedStarts records a day of session starts as the counter would.
*/
func (d *desk) seedStarts(counts map[string]int64) {
	d.t.Helper()
	day := d.tc.Clock.Now().UTC().Format(time.DateOnly)
	d.inWorkspace(func(ctx context.Context, q *models.Queries) error {
		for outcome, n := range counts {
			err := q.AddIdentityVerificationStat(ctx, models.AddIdentityVerificationStatParams{
				WorkspaceID: d.tc.Workspace.Workspace.ID, Day: day, Outcome: outcome, Count: n,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func TestSignInModes(t *testing.T) {
	t.Parallel()

	t.Run("defaults to optional and tells messengers the mode", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.owner.GetJSON("/api/v1/identity-verification/settings").AssertOK().AssertJSONPath("data.mode", "optional")
		d.tc.Client().GetJSON("/widget/v1/apps/"+d.app+"/config").AssertOK().
			AssertJSONPath("data.features.identity_mode", "optional").
			AssertJSONPath("data.features.allow_anonymous", true)
	})

	t.Run("refuses required without a secret, then without enough signed-in starts", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.setMode(testkit.JSON{"mode": "required"}).AssertStatus(409).
			AssertJSONPath("code", "identity_not_configured")
		d.newSecret()
		d.seedStarts(map[string]int64{"verified": 80, "anonymous_started": 30, "token_expired": 5})
		d.setMode(testkit.JSON{"mode": "required"}).AssertStatus(409).
			AssertJSONPath("code", "rollout_guard").AssertJSONPath("verified", float64(80))
		d.setMode(testkit.JSON{"mode": "required", "force": true}).AssertOK().
			AssertJSONPath("data.mode", "required")
	})

	t.Run("required hides the messenger from anonymous visitors only", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		d.seedStarts(map[string]int64{"verified": 100, "anonymous_started": 2})
		d.setMode(testkit.JSON{"mode": "required"}).AssertOK()
		d.tc.Client().GetJSON("/widget/v1/apps/"+d.app+"/config").AssertOK().
			AssertJSONPath("data.features.identity_mode", "required").
			AssertJSONPath("data.features.allow_anonymous", false)
		d.tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": d.app}).AssertForbidden().
			AssertJSONPath("code", "identity_required")
		d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), nil)
	})

	t.Run("off ignores tokens and starts anonymous sessions", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		d.setMode(testkit.JSON{"mode": "off"}).AssertOK()
		res := d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), nil).AssertCreated()
		res.AssertJSONPath("data.kind", "anonymous")
	})

	t.Run("counts how starts went, without anyone's details", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		now := d.tc.Clock.Now()
		d.signedIn(s.sign(t, now, map[string]any{"sub": "user_1"}), nil)
		d.anonymous()
		d.signIn(s.sign(t, now, map[string]any{"sub": "u", "exp": now.Add(-time.Hour)}), nil).AssertUnauthorized()
		require.NoError(t, d.tc.App.Actions.IdentityStats.Flush(t.Context()))
		res := d.owner.GetJSON("/api/v1/identity-verification/stats?days=7").AssertOK()
		res.AssertJSONPath("data.verified", float64(1)).AssertJSONPath("data.anonymous", float64(1)).
			AssertJSONPath("data.invalid", float64(1)).AssertJSONPath("data.totals.token_expired", float64(1))
		assert.True(t, strings.HasPrefix(res.String("data.verified_share"), "0.333"))
	})

	t.Run("keeps the attributes a signed-in messenger may write", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.setMode(testkit.JSON{"client_writable_attributes": []string{"plan_name", "plan_name", "seats"}}).AssertOK().
			AssertJSONPath("data.client_writable_attributes", []any{"plan_name", "seats"})
		d.setMode(testkit.JSON{"client_writable_attributes": []string{"Bad Key"}}).AssertStatus(422)

		s := d.newSecret()
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), nil)
		v.client().PatchJSON("/widget/v1/contact", testkit.JSON{"custom_attributes": testkit.JSON{"seats": 4}}).
			AssertOK()
		v.client().PatchJSON("/widget/v1/contact", testkit.JSON{"custom_attributes": testkit.JSON{"plan": "x"}}).
			AssertForbidden().AssertJSONPath("code", "identity_locked")
		v.client().PatchJSON("/widget/v1/contact", testkit.JSON{"name": "Mallory"}).AssertForbidden()
	})

	t.Run("signs a customer out of every messenger at once", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		now := d.tc.Clock.Now()
		a := d.signedIn(s.sign(t, now, map[string]any{"sub": "user_1"}), nil)
		b := d.signedIn(s.sign(t, now, map[string]any{"sub": "user_1"}), nil)
		d.owner.Delete("/api/v1/contacts/"+a.contact+"/widget-sessions").AssertOK().
			AssertJSONPath("sessions_revoked", float64(2))
		a.client().GetJSON("/widget/v1/session").AssertUnauthorized()
		b.client().GetJSON("/widget/v1/session").AssertUnauthorized()
	})
}
