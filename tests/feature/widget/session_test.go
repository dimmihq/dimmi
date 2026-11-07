package widget_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetSessionStart(t *testing.T) {
	t.Parallel()

	t.Run("loading the config alone makes no contact", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().GetJSON(configPath(t, tc)).AssertOK()

		tc.Table("contacts").AssertCount(0, testkit.Row{})
		tc.Table("widget_sessions").AssertCount(0, testkit.Row{})
	})

	t.Run("keeps only what fits of the pages and context a browser sends", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		pages := []testkit.JSON{{"url": "https://acme.com/" + strings.Repeat("a", 2100)}}
		for i := range 9 {
			pages = append(pages, testkit.JSON{"url": "https://acme.com/p" + strconv.Itoa(i/2)})
		}

		res := tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{
			"app_id": appID(t, tc), "pages": pages,
			"context": testkit.JSON{
				"": "x", "plan": "pro", "seats": 3, "beta": true, "nested": testkit.JSON{"a": 1}, "gone": nil,
				"note": strings.Repeat("é", 300),
			},
		}).AssertCreated()

		var session struct {
			RecentPages []struct{ URL string } `json:"recent_pages"`
			Context     map[string]any         `json:"context"`
		}
		row := (&visitor{tc: tc, id: res.String("data.id")}).session(t)
		require.NoError(t, json.Unmarshal(row.RecentPages, &session.RecentPages))
		require.NoError(t, json.Unmarshal(row.Context, &session.Context))

		assert.Len(t, session.RecentPages, 5)
		assert.Equal(t, "https://acme.com/p0", session.RecentPages[0].URL)
		assert.Equal(t, map[string]any{"plan": "pro", "seats": 3.0, "beta": true, "note": strings.Repeat("é", 250)},
			session.Context)
	})

	t.Run("a first interaction makes a lead named by its session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().WithHeader("Origin", "https://acme.com").WithHeader("User-Agent", "Firefox").
			PostJSON("/widget/v1/sessions", testkit.JSON{
				"app_id": appID(t, tc), "locale": "FR-ca", "timezone": "Europe/Paris",
				"page":  testkit.JSON{"url": "https://acme.com/pricing?token=abc&page=2#top", "title": "Pricing"},
				"pages": []testkit.JSON{{"url": "https://acme.com/"}, {"url": "nonsense"}},
				"sdk":   testkit.JSON{"name": "dimmi-browser", "version": "1.0.0"}, "unknown": true,
			}).AssertCreated()

		res.AssertJSONPath("data.object", "widget_session")
		res.AssertJSONPath("data.kind", "anonymous")
		res.AssertJSONPath("data.test", false)
		res.AssertJSONPath("data.contact.kind", "lead")
		res.AssertJSONPath("data.identity.status", "anonymous")
		res.AssertJSONPath("data.unread_count", float64(0))
		res.AssertJSONPath("data.realtime.channel", "widget_contact:"+res.String("data.contact.id"))
		assert.True(t, strings.HasPrefix(res.String("data.token"), widget.SessionTokenPrefix))

		v := &visitor{tc: tc, id: res.String("data.id"), token: res.String("data.token")}
		row := v.session(t)
		assert.Equal(t, widget.HashSessionToken(v.token), row.TokenHash)
		assert.Equal(t, "fr-CA", *row.Locale)
		assert.Equal(t, "Europe/Paris", *row.Timezone)
		assert.Equal(t, "https://acme.com", *row.Origin)
		assert.Contains(t, string(row.CurrentPage), `"url": "https://acme.com/pricing?page=2"`)
		assert.Contains(t, string(row.RecentPages), "https://acme.com/")
		assert.NotContains(t, string(row.RecentPages), "nonsense")
		assert.Equal(t, int16(0), row.BotScore)

		tc.Table("contact_identities").AssertCount(1, testkit.Row{"type": "widget_visitor", "value": v.id})
	})

	t.Run("a filled hidden field passes silently but marks a bot", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{
			"app_id": appID(t, tc), "website": "https://spam.example",
		}).AssertCreated()

		v := &visitor{tc: tc, id: res.String("data.id")}
		assert.Equal(t, int16(100), v.session(t).BotScore)
	})

	t.Run("an identity token is refused until the workspace has a secret", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": appID(t, tc), "user_jwt": "a.b.c"}).
			AssertUnauthorized().AssertJSONPath("code", "identity_not_configured")
		tc.Table("contacts").AssertCount(0, testkit.Row{})
	})

	t.Run("a widget for signed-in visitors refuses anonymous ones", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		patch(t, tc, testkit.JSON{"allow_anonymous": false})

		tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": appID(t, tc)}).
			AssertForbidden().AssertJSONPath("code", "anonymous_not_allowed")
	})

	t.Run("an unknown app is not found", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": "wgt_nope"}).
			AssertNotFound().AssertJSONPath("code", "app_not_found")
	})
}

func TestWidgetSessionResume(t *testing.T) {
	t.Parallel()

	t.Run("an anonymous session resumes without its token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		res := v.client().GetJSON("/widget/v1/session").AssertOK()

		res.AssertJSONPath("data.id", v.id)
		res.AssertJSONMissing("data.token")
		res.AssertHeader("Cache-Control", "no-store")
	})

	t.Run("each visit slides the expiry on, written at most once a minute", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		before := v.session(t).ExpiresAt

		tc.Clock.Travel(30 * time.Second)
		v.client().GetJSON("/widget/v1/session").AssertOK()
		assert.Equal(t, before, v.session(t).ExpiresAt)

		tc.Clock.Travel(time.Hour)
		v.client().GetJSON("/widget/v1/session").AssertOK()
		assert.True(t, v.session(t).ExpiresAt.After(before))
	})

	t.Run("an expired session says so", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		tc.Clock.Travel(tc.App.Config.Widget.AnonymousSessionTTL + time.Minute)

		v.client().GetJSON("/widget/v1/session").AssertUnauthorized().AssertJSONPath("code", "session_expired")
	})

	t.Run("an unknown or malformed token is invalid", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		for _, token := range []string{"", "nonsense", widget.SessionTokenPrefix + strings.Repeat("a", 43)} {
			tc.Client().WithHeader("Authorization", "Bearer "+token).GetJSON("/widget/v1/session").
				AssertUnauthorized().AssertJSONPath("code", "session_invalid")
		}
	})

	t.Run("signing out ends the session, and signing out again is fine", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		v.client().Delete("/widget/v1/session").AssertNoContent()

		row := v.session(t)
		require.NotNil(t, row.RevokeReason)
		assert.Equal(t, "logout", *row.RevokeReason)
		v.client().GetJSON("/widget/v1/session").AssertUnauthorized().AssertJSONPath("code", "session_revoked")
		v.client().Delete("/widget/v1/session").AssertNoContent()
		tc.Client().WithHeader("Authorization", "Bearer dmi_ws_unknown").Delete("/widget/v1/session").
			AssertUnauthorized()
	})

	t.Run("a session on a widget refusing its site is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		patch(t, tc, testkit.JSON{"allowed_origins": []string{"https://acme.com"}})

		v.client().WithHeader("Origin", "https://evil.example").GetJSON("/widget/v1/session").
			AssertStatus(fhttp.StatusForbidden).AssertJSONPath("code", "origin_not_allowed")
		v.client().WithHeader("Origin", "https://acme.com").GetJSON("/widget/v1/session").AssertOK()
	})
}
