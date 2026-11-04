package widget_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
configPath returns the config path of the workspace's first widget.
*/
func configPath(t *testing.T, tc *tests.TestCase) string {
	t.Helper()

	return "/widget/v1/apps/" + appID(t, tc) + "/config"
}

func TestWidgetConfig(t *testing.T) {
	t.Parallel()

	t.Run("answers with what the browser needs and nothing of anyone's", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().GetJSON(configPath(t, tc)).AssertOK()

		res.AssertJSONPath("data.object", "widget_config")
		res.AssertJSONPath("data.app_id", appID(t, tc))
		res.AssertJSONPath("data.company.name", tc.Workspace.Workspace.Name)
		res.AssertJSONPath("data.appearance.position", "right")
		res.AssertJSONPath("data.appearance.header_ink", "light")
		res.AssertJSONPath("data.texts.greeting", "How can we help?")
		res.AssertJSONPath("data.locale", "en")
		res.AssertJSONPath("data.dir", "ltr")
		res.AssertJSONPath("data.tabs", []any{"home", "messages"})
		res.AssertJSONPath("data.honeypot_field", "website")
		res.AssertJSONPath("data.branding.show", config.BuiltEdition == config.EditionCloud)
		res.AssertJSONMissing("data.surface")
		assert.NotContains(t, string(res.Body), tc.Owner.Email)
		assert.Contains(t, res.Header.Values("Vary"), "Origin")
	})

	t.Run("is cacheable with an entity tag that a revalidation matches", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().GetJSON(configPath(t, tc)).AssertOK()
		res.AssertHeader("Cache-Control", "public, max-age=60, s-maxage=60, stale-while-revalidate=600")

		tag := res.Header.Get("ETag")
		assert.Contains(t, tag, appID(t, tc)+":")

		tc.Client().WithHeader("If-None-Match", tag).GetJSON(configPath(t, tc)).AssertStatus(fhttp.StatusNotModified)
	})

	t.Run("a change gives a new entity tag at once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tag := tc.Client().GetJSON(configPath(t, tc)).AssertOK().Header.Get("ETag")

		patch(t, tc, testkit.JSON{"appearance": testkit.JSON{"primary_color": "#000000"}})

		res := tc.Client().WithHeader("If-None-Match", tag).GetJSON(configPath(t, tc)).AssertOK()
		res.AssertJSONPath("data.appearance.primary_color", "#000000")
	})

	t.Run("a change on one replica reaches another", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id := defaultWidget(t, tc).ID
		replica := widget.NewDirectory(tc.App.DB, tc.App.PubSub, time.Minute)
		stop, err := replica.Listen(t.Context())
		require.NoError(t, err)
		t.Cleanup(stop)

		first, err := replica.Find(t.Context(), id)
		require.NoError(t, err)

		patch(t, tc, testkit.JSON{"name": "Renamed"})

		require.Eventually(t, func() bool {
			found, err := replica.Find(t.Context(), id)

			return err == nil && found.Widget.ConfigVersion > first.Widget.ConfigVersion
		}, 5*time.Second, 20*time.Millisecond)
	})

	t.Run("an unknown, malformed or deleted app answers the same", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		for _, app := range []string{"wgt_nonsense", "nope", models.PublicID(models.PrefixWidget, ids.New())} {
			tc.Client().GetJSON("/widget/v1/apps/"+app+"/config").AssertNotFound().
				AssertJSONPath("code", "app_not_found")
		}
	})

	t.Run("a disabled widget says so and comes back", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		patch(t, tc, testkit.JSON{"enabled": false})
		tc.Client().GetJSON(configPath(t, tc)).AssertForbidden().AssertJSONPath("code", "widget_disabled")

		patch(t, tc, testkit.JSON{"enabled": true})
		tc.Client().GetJSON(configPath(t, tc)).AssertOK()
	})

	t.Run("a suspended workspace says so", func(t *testing.T) {
		t.Parallel()

		tc := tests.New(t)
		_, err := tc.Operator().Exec(t.Context(), "UPDATE workspaces SET suspended_at = now() WHERE id = $1",
			tc.Workspace.Workspace.ID)
		require.NoError(t, err)

		tc.Client().GetJSON(configPath(t, tc)).AssertForbidden().AssertJSONPath("code", "workspace_suspended")
	})

	t.Run("turned off on the install answers unavailable", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_WIDGET__ENABLED=false"))

		tc.Client().GetJSON(configPath(t, tc)).AssertStatus(fhttp.StatusServiceUnavailable).
			AssertJSONPath("code", "widget_unavailable")
	})

	t.Run("resolver functions work under row level security and reveal only ids", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id := defaultWidget(t, tc).ID
		q := models.New(tc.ApplicationRole())

		row, err := q.ResolveWidgetApp(t.Context(), id)
		require.NoError(t, err)
		assert.Equal(t, tc.Workspace.Workspace.ID, row.WorkspaceID)

		_, err = q.ListWidgetSettings(t.Context(), tc.Workspace.Workspace.ID)
		if err == nil {
			rows, _ := q.ListWidgetSettings(t.Context(), tc.Workspace.Workspace.ID)
			assert.Empty(t, rows)
		}
	})
}

func TestWidgetOrigin(t *testing.T) {
	t.Parallel()

	allowed := func(t *testing.T, tc *tests.TestCase, origins ...string) {
		t.Helper()
		patch(t, tc, testkit.JSON{"allowed_origins": origins})
	}

	t.Run("an empty list allows any origin, a sandboxed page and no origin at all", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		for _, origin := range []string{"https://any.example", "null"} {
			tc.Client().WithHeader("Origin", origin).GetJSON(configPath(t, tc)).AssertOK().
				AssertHeader("Access-Control-Allow-Origin", origin)
		}

		tc.Client().GetJSON(configPath(t, tc)).AssertOK()
	})

	t.Run("a list allows its sites and refuses others at once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		allowed(t, tc, "https://acme.com", "https://*.acme.com")

		for _, origin := range []string{"https://acme.com", "https://ACME.com:443", "https://app.acme.com"} {
			tc.Client().WithHeader("Origin", origin).GetJSON(configPath(t, tc)).AssertOK()
		}

		for _, origin := range []string{"https://evil.example", "null", "https://a.b.acme.com", "http://acme.com"} {
			res := tc.Client().WithHeader("Origin", origin).GetJSON(configPath(t, tc)).AssertForbidden()
			res.AssertJSONPath("code", "origin_not_allowed")
			res.AssertHeader("Cache-Control", "no-store")
		}
	})

	t.Run("a wildcard does not cover its bare domain", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		allowed(t, tc, "https://*.acme.com")

		tc.Client().WithHeader("Origin", "https://acme.com").GetJSON(configPath(t, tc)).AssertForbidden()
	})

	t.Run("a request with no origin passes only when apps are accepted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		allowed(t, tc, "https://acme.com")

		tc.Client().GetJSON(configPath(t, tc)).AssertForbidden().AssertJSONPath("code", "origin_not_allowed")

		patch(t, tc, testkit.JSON{"native_apps_enabled": true})
		tc.Client().GetJSON(configPath(t, tc)).AssertOK()
	})

	t.Run("this install's own origin always passes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_APP__URL=https://install.example"))
		allowed(t, tc, "https://acme.com")

		tc.Client().WithHeader("Origin", "https://install.example").GetJSON(configPath(t, tc)).AssertOK()
	})

	t.Run("a preflight answers for allowed sites and refuses others", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		allowed(t, tc, "https://acme.com")

		ok := tc.Client().WithHeader("Origin", "https://acme.com").
			WithHeader("Access-Control-Request-Method", "GET").Send(fhttp.MethodOptions, configPath(t, tc), nil)
		ok.AssertNoContent()
		ok.AssertHeader("Access-Control-Allow-Origin", "https://acme.com")
		ok.AssertHeader("Access-Control-Max-Age", "7200")
		assert.Contains(t, ok.Header.Get("Access-Control-Allow-Headers"), "Dimmi-Feedback-Session")
		assert.Empty(t, ok.Header.Get("Access-Control-Allow-Credentials"))

		no := tc.Client().WithHeader("Origin", "https://evil.example").Send(fhttp.MethodOptions, configPath(t, tc), nil)
		no.AssertForbidden()
		assert.Empty(t, no.Header.Get("Access-Control-Allow-Origin"))
	})

	t.Run("a preflight on a route naming no app or a missing one passes on to the real request", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().WithHeader("Origin", "https://acme.com").Send(fhttp.MethodOptions, "/widget/v1/sessions", nil).
			AssertNoContent()
		tc.Client().WithHeader("Origin", "https://acme.com").Send(fhttp.MethodOptions, "/widget/v1/apps/nope/config", nil).
			AssertNoContent()
		tc.Client().WithHeader("Origin", "https://acme.com").
			Send(fhttp.MethodOptions,
				"/widget/v1/apps/"+models.PublicID(models.PrefixWidget, ids.New())+"/config", nil).
			AssertNoContent()
	})
}

func TestWidgetAvailability(t *testing.T) {
	t.Parallel()

	path := func(t *testing.T, tc *tests.TestCase) string {
		t.Helper()

		return "/widget/v1/apps/" + appID(t, tc) + "/availability"
	}

	t.Run("nobody with a connection means offline, and no email ever shows", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().GetJSON(path(t, tc)).AssertOK()

		res.AssertJSONPath("data.object", "widget_availability")
		res.AssertJSONPath("data.online", false)
		res.AssertJSONPath("data.within_hours", true)
		res.AssertJSONPath("data.reply_time", "hours")
		res.AssertJSONPath("data.reply_time_text", "Usually replies in a few hours")
		res.AssertHeader("Cache-Control", "public, max-age=30")
		assert.NotContains(t, string(res.Body), tc.Owner.Email)
	})

	t.Run("a connected available teammate means online, away does not", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		base := tc.RealtimeURL()
		listener := tc.ListenAs(tc.Owner, base)
		require.Nil(t, listener.Subscribe("workspace:"+workspaceOf(tc)).Error)

		require.Eventually(t, func() bool {
			return tc.Client().GetJSON(path(t, tc)).AssertOK().String("data.reply_time") == "minutes"
		}, 5*time.Second, 50*time.Millisecond)

		_, err := tc.Operator().Exec(t.Context(),
			"UPDATE workspace_members SET availability = 'away' WHERE workspace_id = $1",
			tc.Workspace.Workspace.ID)
		require.NoError(t, err)

		tc.Client().GetJSON(path(t, tc)).AssertOK().AssertJSONPath("data.online", false)
	})

	t.Run("shows at most three faces with avatars", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(*app.App) {}))

		for range 4 {
			user := tc.CreateUser()
			_, err := factories.Member(t.Context(), tc.App, factories.Membership{
				Workspace: tc.Workspace.Workspace, User: user, Role: "member",
			})
			require.NoError(t, err)

			_, err = tc.Operator().Exec(t.Context(),
				"UPDATE users SET avatar_path = 'avatars/x.png' WHERE id = $1", user.ID)
			require.NoError(t, err)
		}

		var got struct {
			Data struct {
				Team []widget.TeamFace `json:"team"`
			} `json:"data"`
		}

		tc.Client().GetJSON(path(t, tc)).AssertOK().Decode(&got)
		assert.LessOrEqual(t, len(got.Data.Team), 3)
	})
}
