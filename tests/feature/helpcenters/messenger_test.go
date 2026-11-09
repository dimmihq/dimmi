package helpcenters_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/tests"
)

func TestHelpCenterMessenger(t *testing.T) {
	t.Parallel()

	t.Run("shows the workspace's first messenger on every page until turned off", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		first := c.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
		rule := testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "pro"}
		audience := c.d.Owner.PostJSON("/api/v1/audiences", testkit.JSON{
			"name": "Pro", "rules": testkit.JSON{"rules": rule},
		}).AssertCreated().String("data.id")
		kept := c.article(t, testkit.JSON{"title": "Priority support", "body_markdown": "Call us."}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+kept+"/publications", nil).AssertOK()
		c.d.Owner.PatchJSON("/api/v1/articles/"+kept+"/audience", testkit.JSON{"audience_id": audience}).AssertOK()

		for _, path := range []string{"", "/search?q=x", "/changelog"} {
			body := string(c.page(path).Body)
			assert.Contains(t, body, `/widget/v1/loader.js" data-app-id="`+first+`"`, path)
		}
		assert.Contains(t, string(c.page("/search?q=nothing-matches").Body), `data-dimmi="open"`)

		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{"settings": testkit.JSON{"messenger": false}}).
			AssertOK().AssertJSONPath("data.settings.messenger", false)
		body := string(c.page("/search?q=nothing-matches").Body)
		assert.NotContains(t, body, "/widget/v1/loader.js")
		assert.NotContains(t, body, `data-dimmi="open"`)
		assert.NotContains(t, body, "/conversations/new")
	})

	t.Run("offers Ask AI where the agent answers the messenger and Contact us where the team does", func(t *testing.T) {
		t.Parallel()
		team, ai, elsewhere := newCenter(t), newCenter(t), newCenter(t)
		for _, c := range []center{ai, elsewhere} {
			c.d.Owner.GetJSON("/api/v1/ai/agent").AssertOK()
		}
		agent := func(c center, sql string) {
			_, err := c.d.TC.Operator().Exec(t.Context(), sql, c.d.TC.Workspace.Workspace.ID)
			require.NoError(t, err)
		}
		agent(ai, "UPDATE ai_agents SET enabled = true WHERE workspace_id = $1")
		agent(elsewhere, "UPDATE ai_agents SET enabled = true, enabled_channels = '{email}' WHERE workspace_id = $1")

		home := string(team.page("").Body)
		assert.Contains(t, home, "<span>Contact us</span>")
		assert.NotContains(t, home, "data-ask-ai")
		assert.Contains(t, home, `placeholder="Search for articles…"`)

		home = string(ai.page("").Body)
		assert.Contains(t, home, "<span>Ask AI</span>")
		assert.Contains(t, home, "data-ask-ai")
		assert.NotContains(t, string(ai.page("/search?q=x").Body), "data-ask-ai")

		assert.Contains(t, string(elsewhere.page("").Body), "<span>Contact us</span>")
	})

	t.Run("shows no way to get help while the workspace's messenger is switched off", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		widget := c.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
		c.d.Owner.PatchJSON("/api/v1/widget-settings/"+widget, testkit.JSON{"enabled": false}).AssertOK()

		body := string(c.page("").Body)
		assert.NotContains(t, body, "/widget/v1/loader.js")
		assert.NotContains(t, body, `data-dimmi="open"`)
	})

	t.Run("shows the picked messenger and refuses one the workspace does not have", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		second := c.d.Owner.PostJSON("/api/v1/widget-settings", testkit.JSON{"name": "Sales"}).AssertCreated().
			String("data.id")

		pick := testkit.JSON{"settings": testkit.JSON{"messenger_id": second}}
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, pick).AssertOK()
		assert.Contains(t, string(c.page("").Body), `data-app-id="`+second+`"`)

		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id,
			testkit.JSON{"settings": testkit.JSON{"messenger_id": "wgt_0000000000000000000000000"}}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "messenger_unknown")
	})

	t.Run("lets a messenger kept to listed sites answer its own workspace's help center only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Config.HelpCenters.BaseDomain = "help.test" }))
		owner := tc.In(tc.Owner)
		widget := owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
		sub := owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.subdomain")
		require.NotEmpty(t, sub)
		owner.PatchJSON("/api/v1/widget-settings/"+widget, testkit.JSON{"allowed_origins": []string{"https://acme.com"}}).
			AssertOK()
		config := "/widget/v1/apps/" + widget + "/config"

		tc.Client().WithHeader("Origin", "https://"+sub+".help.test").GetJSON(config).AssertOK()
		tc.Client().WithHeader("Origin", "https://someone-else.help.test").GetJSON(config).AssertForbidden()
		tc.Client().WithHeader("Origin", "http://"+sub+".help.test").GetJSON(config).AssertForbidden()
	})
}
