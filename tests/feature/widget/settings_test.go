package widget_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/entitlements"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestDefaultWidget(t *testing.T) {
	t.Parallel()

	t.Run("a new workspace has one widget on its default inbox", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		widget := defaultWidget(t, tc)

		res := owner(tc).GetJSON(settingsPath(t, tc)).AssertOK()
		res.AssertJSONPath("data.object", "widget_settings")
		res.AssertJSONPath("data.name", "Website")
		res.AssertJSONPath("data.inbox.object", "inbox")
		res.AssertJSONPath("data.config_version", float64(widget.ConfigVersion))
	})

	t.Run("repairing a workspace that has a widget changes nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		code, out := tc.Command("workspace:repair", tc.Workspace.Workspace.Slug)
		require.Equal(t, 0, code, out)

		assert.Len(t, listWidgets(t, tc), 1)
	})

	t.Run("repairing restores a default after the last was deleted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner(tc).Delete(settingsPath(t, tc)).AssertNoContent()
		assert.Empty(t, listWidgets(t, tc))

		code, out := tc.Command("workspace:repair", tc.Workspace.Workspace.Slug)
		require.Equal(t, 0, code, out)
		assert.Len(t, listWidgets(t, tc), 1)
	})
}

/*
listWidgets returns the workspace's live widgets.
*/
func listWidgets(t *testing.T, tc *tests.TestCase) []models.WidgetSetting {
	t.Helper()

	var out []models.WidgetSetting

	inWorkspace(t, tc, tc.Workspace.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.ListWidgetSettings(ctx, tc.Workspace.Workspace.ID)

		return err
	})

	return out
}

func TestWidgetSettingsCreate(t *testing.T) {
	t.Parallel()

	t.Run("makes a widget with its own channel on the inbox named", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		inbox, err := factories.Inbox(t.Context(), tc.App, tc.Workspace.Workspace, "Sales")
		require.NoError(t, err)

		res := owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{
			"name": "Docs", "inbox_id": models.PublicID(models.PrefixInbox, inbox.Inbox.ID),
			"appearance": testkit.JSON{"primary_color": "#5B5BD6"},
		}).AssertCreated()

		res.AssertJSONPath("data.name", "Docs")
		res.AssertJSONPath("data.inbox.name", "Sales")
		res.AssertJSONPath("data.appearance.primary_color", "#5B5BD6")
		assert.Len(t, listWidgets(t, tc), 2)
	})

	t.Run("keeps a workspace within ten widgets and each has its own id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		seen := map[string]bool{appID(t, tc): true}

		for range 9 {
			res := owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{}).AssertCreated()
			seen[res.String("data.id")] = true
		}

		assert.Len(t, seen, 10)

		refused := owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		refused.AssertJSONPath("code", "entitlement_limit_reached")
		refused.AssertJSONPath("key", entitlements.KeyWidgetMaxPerWorkspace)
		refused.AssertJSONPath("limit", float64(10))
		refused.AssertJSONPath("used", float64(10))
	})

	t.Run("a raised limit lets more through and a lowered one deletes nothing", func(t *testing.T) {
		t.Parallel()

		driver := &entitlements.Driver{Catalog: entitlements.Catalog{
			entitlements.KeyWidgetMaxPerWorkspace: 2, entitlements.KeyBillingTestModeCap: entitlements.Unlimited,
		}}
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Entitlements = driver }))

		owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{}).AssertCreated()
		owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{}).AssertStatus(fhttp.StatusUnprocessableEntity)

		driver.Catalog[entitlements.KeyWidgetMaxPerWorkspace] = 1

		assert.Len(t, listWidgets(t, tc), 2)
	})

	t.Run("two creates at the limit let only one through", func(t *testing.T) {
		t.Parallel()

		driver := &entitlements.Driver{Catalog: entitlements.Catalog{
			entitlements.KeyWidgetMaxPerWorkspace: 2, entitlements.KeyBillingTestModeCap: entitlements.Unlimited,
		}}
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Entitlements = driver }))
		first, second := owner(tc), owner(tc)
		done := make(chan int, 2)

		for _, client := range []*testkit.Client{first, second} {
			go func() {
				done <- client.PostJSON("/api/v1/widget-settings", testkit.JSON{}).Status
			}()
		}

		statuses := []int{<-done, <-done}
		assert.ElementsMatch(t, []int{fhttp.StatusCreated, fhttp.StatusUnprocessableEntity}, statuses)
	})

	t.Run("replays a post with the same idempotency key", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		client := owner(tc).WithHeader("Idempotency-Key", "widget-one")

		first := client.PostJSON("/api/v1/widget-settings", testkit.JSON{"name": "One"}).AssertCreated()
		second := client.PostJSON("/api/v1/widget-settings", testkit.JSON{"name": "One"}).AssertCreated()

		assert.Equal(t, first.String("data.id"), second.String("data.id"))
		assert.Len(t, listWidgets(t, tc), 2)
	})

	t.Run("refuses an inbox of no one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{
			"inbox_id": models.PublicID(models.PrefixInbox, ids.New()),
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "inbox_not_found")
		owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{"inbox_id": "nope"}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
		assert.Len(t, listWidgets(t, tc), 1)
	})

	t.Run("a failing write leaves nothing behind", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.FailWrites("widget_settings", "INSERT")

		owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{}).AssertStatus(fhttp.StatusInternalServerError)

		var channels int64

		inWorkspace(t, tc, tc.Workspace.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
			count, err := q.CountWidgetSettings(ctx, tc.Workspace.Workspace.ID)
			channels = count

			return err
		})
		assert.Equal(t, int64(1), channels)
		tc.Table("channel_connections").AssertCount(1, testkit.Row{"type": "widget"})
	})
}

func TestWidgetSettingsPermissions(t *testing.T) {
	t.Parallel()

	t.Run("members read but do not change", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := memberClient(t, tc, enums.RoleMember)

		member.GetJSON("/api/v1/widget-settings").AssertOK()
		member.GetJSON(settingsPath(t, tc)).AssertOK()
		member.PatchJSON(settingsPath(t, tc), testkit.JSON{"name": "Mine"}).AssertForbidden()
		member.PostJSON("/api/v1/widget-settings", testkit.JSON{}).AssertForbidden()
		member.Delete(settingsPath(t, tc)).AssertForbidden()
	})

	t.Run("admins change", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, admin := memberClient(t, tc, enums.RoleAdmin)

		admin.PatchJSON(settingsPath(t, tc), testkit.JSON{"name": "Ours"}).AssertOK()
	})

	t.Run("a member's token with the settings write ability still cannot manage", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := memberClient(t, tc, enums.RoleMember)

		tc.ActingAsToken(member, "settings:write").PatchJSON(settingsPath(t, tc), testkit.JSON{"name": "Mine"}).
			AssertForbidden()
	})

	t.Run("an owner's token needs the ability", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		path := settingsPath(t, tc)

		tc.ActingAsToken(tc.Workspace.Owner, "settings:read").PatchJSON(path, testkit.JSON{"name": "A"}).
			AssertForbidden()
		tc.ActingAsToken(tc.Workspace.Owner, "settings:write").PatchJSON(path, testkit.JSON{"name": "A"}).AssertOK()
	})

	t.Run("another workspace's widget is not found", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tc.CreateUser()
		otherWorkspace, err := factories.Workspace(t.Context(), tc.App, other)
		require.NoError(t, err)

		client := tc.ActingAs(other).WithHeader(workspaceHeader,
			models.PublicID(models.PrefixWorkspace, otherWorkspace.Workspace.ID))

		client.GetJSON(settingsPath(t, tc)).AssertNotFound()
		client.PatchJSON(settingsPath(t, tc), testkit.JSON{"name": "A"}).AssertNotFound()
		client.Delete(settingsPath(t, tc)).AssertNotFound()
		client.GetJSON("/api/v1/widget-settings/nonsense").AssertNotFound()
	})
}

func TestWidgetSettingsUpdate(t *testing.T) {
	t.Parallel()

	t.Run("moves the version on and merges appearance key by key", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		before := defaultWidget(t, tc).ConfigVersion

		res := patch(t, tc, testkit.JSON{"appearance": testkit.JSON{"primary_color": "#5B5BD6", "position": "left"}})
		res.AssertJSONPath("data.config_version", float64(before+1))
		res.AssertJSONPath("data.appearance.theme", "auto")

		res = patch(t, tc, testkit.JSON{"appearance": testkit.JSON{
			"position": nil, "launcher": testkit.JSON{"style": "tab"},
		}})
		res.AssertJSONPath("data.appearance.primary_color", "#5B5BD6")
		res.AssertJSONPath("data.appearance.position", "right")
		res.AssertJSONPath("data.appearance.launcher.style", "tab")
		res.AssertJSONPath("data.appearance.launcher.shape", "circle")
	})

	t.Run("a change to nothing does not move the version", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		before := defaultWidget(t, tc).ConfigVersion

		patch(t, tc, testkit.JSON{"name": "Website"})

		assert.Equal(t, before, defaultWidget(t, tc).ConfigVersion)
	})

	for name, c := range map[string]struct {
		body  testkit.JSON
		field string
	}{
		"color":       {testkit.JSON{"appearance": testkit.JSON{"primary_color": "red"}}, "appearance.primary_color"},
		"theme":       {testkit.JSON{"appearance": testkit.JSON{"theme": "neon"}}, "appearance.theme"},
		"unknown key": {testkit.JSON{"appearance": testkit.JSON{"nope": 1}}, "appearance.nope"},
		"padding":     {testkit.JSON{"appearance": testkit.JSON{"padding_x": 500}}, "appearance.padding_x"},
		"launcher": {
			testkit.JSON{"appearance": testkit.JSON{"launcher": testkit.JSON{"style": "x"}}},
			"appearance.launcher.style",
		},
		"background": {
			testkit.JSON{"appearance": testkit.JSON{"background": testkit.JSON{"type": "x"}}},
			"appearance.background.type",
		},
		"text key":      {testkit.JSON{"texts": testkit.JSON{"nope": "x"}}, "texts.nope"},
		"long text":     {testkit.JSON{"texts": testkit.JSON{"greeting_intro": strings.Repeat("a", 61)}}, ""},
		"empty name":    {testkit.JSON{"name": " "}, "name"},
		"reply time":    {testkit.JSON{"reply_time": "soon"}, "reply_time"},
		"collect email": {testkit.JSON{"collect_email": "x"}, "collect_email"},
		"attachment":    {testkit.JSON{"max_attachment_mb": 26}, "max_attachment_mb"},
		"locale":        {testkit.JSON{"locale": "not a tag"}, "locale"},
		"help tab":      {testkit.JSON{"help_tab_enabled": true}, "help_tab_enabled"},
		"changelog tab": {testkit.JSON{"changelog_tab_enabled": true}, "changelog_tab_enabled"},
	} {
		t.Run("refuses a bad "+name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)

			res := owner(tc).PatchJSON(settingsPath(t, tc), c.body).AssertStatus(fhttp.StatusUnprocessableEntity)
			if c.field != "" {
				res.AssertValidationError(c.field)
			}
		})
	}

	t.Run("refuses each bad appearance value on its own field", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		as, path := owner(tc), settingsPath(t, tc)
		look := func(key string, value any) testkit.JSON {
			return testkit.JSON{"appearance": testkit.JSON{key: value}}
		}

		for field, body := range map[string]testkit.JSON{
			"appearance.header_color":              look("header_color", "blue"),
			"appearance.sound":                     look("sound", "yes"),
			"appearance.logo_asset_id":             look("logo_asset_id", 7),
			"appearance.launcher":                  look("launcher", "tab"),
			"appearance.launcher.label":            look("launcher", testkit.JSON{"label": strings.Repeat("a", 25)}),
			"appearance.launcher.nope":             look("launcher", testkit.JSON{"nope": true}),
			"appearance.background":                look("background", "solid"),
			"appearance.background.gradient":       look("background", testkit.JSON{"gradient": "red"}),
			"appearance.background.fade":           look("background", testkit.JSON{"fade": 1}),
			"appearance.background.image_asset_id": look("background", testkit.JSON{"image_asset_id": false}),
			"texts.greeting":                       {"texts": testkit.JSON{"greeting": 5}},
		} {
			as.PatchJSON(path, body).
				AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError(field)
		}

		for _, gradient := range []testkit.JSON{
			{"from": "#000000"},
			{"from": "#000000", "to": "#ffffff", "angle": 400},
			{"from": "#000000", "to": "#ffffff", "angle": 1.5},
		} {
			as.PatchJSON(path, look("background", testkit.JSON{"gradient": gradient})).
				AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError("appearance.background.gradient")
		}
	})

	t.Run("takes a full appearance and clears a text with null", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := patch(t, tc, testkit.JSON{
			"appearance": testkit.JSON{
				"header_color": nil, "button_color": "#112233", "sound": false, "logo_asset_id": nil,
				"launcher": testkit.JSON{"label": "Ask us", "image_asset_id": nil},
				"background": testkit.JSON{
					"type": "gradient", "fade": true,
					"gradient": testkit.JSON{"from": "#000000", "to": "#ffffff", "angle": 90},
				},
			},
			"texts": testkit.JSON{"greeting": "Hi there\u0007", "chat_title": "Chat"},
		})
		res.AssertJSONPath("data.appearance.button_color", "#112233")
		res.AssertJSONPath("data.appearance.background.gradient.angle", 90.0)
		res.AssertJSONPath("data.texts.greeting", "Hi there")

		patch(t, tc, testkit.JSON{"texts": testkit.JSON{"greeting": nil, "chat_title": ""}}).
			AssertJSONMissing("data.texts.greeting").AssertJSONMissing("data.texts.chat_title")

		options := patch(t, tc, testkit.JSON{
			"reply_time": "minutes", "collect_email": "always", "locale": "de", "max_attachment_mb": 5,
			"appearance": testkit.JSON{"primary_color": "#FFFFFF", "header_color": "#FAFAFA"},
		})
		options.AssertJSONPath("data.reply_time", "minutes").AssertJSONPath("data.locale", "de")
		options.AssertJSONPath("data.max_attachment_mb", 5.0)

		config := tc.Client().GetJSON(configPath(t, tc)).AssertOK()
		config.AssertJSONPath("data.appearance.header_ink", "dark")
		config.AssertJSONPath("data.appearance.button_color", "#112233")
		config.AssertJSONPath("data.appearance.background.gradient.from", "#000000")
	})

	t.Run("refuses a name too long for the list", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner(tc).PostJSON("/api/v1/widget-settings", testkit.JSON{"name": strings.Repeat("a", 81)}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})

	t.Run("sends new conversations to another inbox", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		inbox, err := factories.Inbox(t.Context(), tc.App, tc.Workspace.Workspace, "Sales")
		require.NoError(t, err)

		res := patch(t, tc, testkit.JSON{"inbox_id": models.PublicID(models.PrefixInbox, inbox.Inbox.ID)})

		res.AssertJSONPath("data.inbox.name", "Sales")
	})

	t.Run("records who changed the allowed sites but not the look", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		patch(t, tc, testkit.JSON{"name": "Renamed"})
		tc.Table("audit_logs").AssertCount(0, testkit.Row{"event": "widget.settings_updated"})

		patch(t, tc, testkit.JSON{"allowed_origins": []string{"https://acme.com"}})
		tc.Table("audit_logs").AssertCount(1, testkit.Row{"event": "widget.settings_updated"})
	})

	t.Run("deleting records it and leaves the channel disconnected", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		widget := defaultWidget(t, tc)

		owner(tc).Delete(settingsPath(t, tc)).AssertNoContent()

		tc.Table("audit_logs").AssertCount(1, testkit.Row{"event": "widget.deleted"})
		tc.Table("channel_connections").
			AssertCount(1, testkit.Row{"id": widget.ChannelConnectionID, "status": "disconnected"})
		tc.Client().GetJSON("/widget/v1/apps/"+models.PublicID(models.PrefixWidget, widget.ID)+"/config").
			AssertNotFound().AssertJSONPath("code", "app_not_found")
	})
}

func TestWidgetAllowedOrigins(t *testing.T) {
	t.Parallel()

	t.Run("stores sites in their normal form", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := patch(t, tc, testkit.JSON{"allowed_origins": []string{
			"https://ACME.com", "https://acme.com:443", "https://*.acme.com", "http://localhost:3000",
			"https://münchen.de",
		}})

		var got struct {
			Data struct {
				AllowedOrigins []string `json:"allowed_origins"`
			} `json:"data"`
		}

		res.Decode(&got)
		assert.Equal(t, []string{
			"https://acme.com", "https://*.acme.com", "http://localhost:3000", "https://xn--mnchen-3ya.de",
		}, got.Data.AllowedOrigins)
	})

	for name, bad := range map[string]string{
		"a path": "https://acme.com/app", "a trailing slash": "https://acme.com/", "no scheme": "acme.com",
		"a wildcard in the middle": "https://a.*.acme.com", "a public suffix wildcard": "https://*.com",
		"null": "null",
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)

			owner(tc).PatchJSON(settingsPath(t, tc), testkit.JSON{"allowed_origins": []string{"https://ok.com", bad}}).
				AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError("allowed_origins.1")
		})
	}

	t.Run("refuses more than fifty", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		many := make([]string, 51)

		for i := range many {
			many[i] = "https://s" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".com"
		}

		owner(tc).PatchJSON(settingsPath(t, tc), testkit.JSON{"allowed_origins": many}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}
