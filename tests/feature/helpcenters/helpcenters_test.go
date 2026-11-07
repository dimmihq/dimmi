package helpcenters_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/tests"
)

/*
center is a workspace's default help center as the owner reaches it.
*/
type center struct {
	d         tests.Desk
	id        string
	subdomain string
}

/*
newCenter boots a workspace and reads its default help center.
*/
func newCenter(t *testing.T) center {
	t.Helper()
	d := tests.NewDesk(t)
	res := d.Owner.GetJSON("/api/v1/help-centers").AssertOK().AssertJSONPath("data.0.is_default", true)
	return center{d: d, id: res.String("data.0.id"), subdomain: res.String("data.0.subdomain")}
}

/*
article creates and returns a draft through the API.
*/
func (c center) article(t *testing.T, body testkit.JSON) *testkit.Response {
	t.Helper()
	return c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/articles", body).AssertCreated()
}

/*
page opens a public page of the help center.
*/
func (c center) page(path string) *testkit.Response {
	return c.d.TC.Client().WithHeader("Accept", "text/html").Send("GET", "/hc/"+c.subdomain+path, nil)
}

func TestHelpCenters(t *testing.T) {
	t.Parallel()

	t.Run("gives every workspace a default help center at its slug", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		assert.Equal(t, c.d.TC.Workspace.Workspace.Slug, c.subdomain)
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{
			"name": "Acme Docs", "settings": testkit.JSON{"primary_color": "#123456", "home_title": "How can we help?"},
		}).AssertOK().AssertJSONPath("data.name", "Acme Docs").AssertJSONPath("data.settings.primary_color", "#123456")
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{"subdomain": "help"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "subdomain_reserved")
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id,
			testkit.JSON{"settings": testkit.JSON{"primary_color": "red"}}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})

	t.Run("publishes an article readers can open, search and follow after a rename", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		col := c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", testkit.JSON{"name": "Getting started"}).
			AssertCreated().AssertJSONPath("data.slug", "getting-started").String("data.id")
		art := c.article(t, testkit.JSON{
			"title": "Invite your team", "collection_id": col, "description": "Bring teammates in.",
			"body_markdown": "## Send invitations\n\nOpen **Settings** and invite by email.",
		})
		id := art.String("data.id")
		art.AssertJSONPath("data.slug", "invite-your-team").AssertJSONPath("data.status", "draft")
		c.page("/articles/invite-your-team").AssertNotFound()

		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK().
			AssertJSONPath("data.status", "published")
		page := c.page("/articles/invite-your-team").AssertOK()
		assert.Contains(t, string(page.Body), "Send invitations")
		assert.Contains(t, string(page.Body), "<strong>Settings</strong>")
		md := c.page("/articles/invite-your-team.md").AssertOK()
		assert.True(t, strings.HasPrefix(string(md.Body), "# Invite your team"))
		assert.Contains(t, string(c.page("/search?q=invitations").AssertOK().Body), "Invite your team")
		assert.Contains(t, string(c.page("").AssertOK().Body), "Getting started")

		c.d.Owner.PatchJSON("/api/v1/articles/"+id, testkit.JSON{"slug": "invite-teammates", "lock_version": 1}).
			AssertOK().AssertJSONPath("data.has_unpublished_changes", true)
		c.page("/articles/invite-your-team").AssertOK()
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		c.page("/articles/invite-your-team").AssertStatus(fhttp.StatusMovedPermanently).
			AssertHeader("Location", "/hc/"+c.subdomain+"/articles/invite-teammates")
		c.d.Owner.GetJSON("/api/v1/articles/"+id+"/versions").AssertOK().AssertJSONPath("data.0.number", float64(2))
	})

	t.Run("keeps drafts, archived and agent-only articles away from readers", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		secret := c.article(t, testkit.JSON{
			"title": "Internal pricing rules", "visibility": "agent_only",
			"body_markdown": "Discounts above 20% need approval.",
		}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+secret+"/publications", nil).AssertOK()
		c.page("/articles/internal-pricing-rules").AssertNotFound()
		assert.NotContains(t, string(c.page("/search?q=discounts").AssertOK().Body), "Internal pricing")

		gone := c.article(t, testkit.JSON{"title": "Old guide", "body_markdown": "Old."}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+gone+"/publications", nil).AssertOK()
		c.page("/articles/old-guide").AssertOK()
		c.d.Owner.PatchJSON("/api/v1/articles/"+gone, testkit.JSON{"status": "archived"}).AssertOK()
		c.page("/articles/old-guide").AssertNotFound()
	})

	t.Run("refuses a save over someone else's and a slug already taken", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id := c.article(t, testkit.JSON{"title": "Billing"}).String("data.id")
		c.d.Owner.PatchJSON("/api/v1/articles/"+id, testkit.JSON{"body_markdown": "One", "lock_version": 1}).AssertOK()
		c.d.Owner.PatchJSON("/api/v1/articles/"+id, testkit.JSON{"body_markdown": "Two", "lock_version": 1}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "article_changed")
		c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/articles", testkit.JSON{"title": "Billing"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "slug_taken")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "nothing_to_publish")
		require.Len(t, c.d.Owner.GetJSON("/api/v1/help-centers/"+c.id+"/articles?status=published").AssertOK().
			String("data.0.id"), len(id))
	})

	t.Run("feeds the messenger's Help tab with plain blocks", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		col := c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", testkit.JSON{"name": "Billing"}).
			AssertCreated().String("data.id")
		id := c.article(t, testkit.JSON{
			"title": "Change your plan", "collection_id": col,
			"body_markdown": "## Steps\n\n1. Open **Billing**\n2. Pick a plan\n\n> [!NOTE]\n> Prorated.\n",
		}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		app := c.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")

		res := c.d.TC.Client().GetJSON("/widget/v1/apps/" + app + "/help").AssertOK()
		res.AssertJSONPath("data.collections.0.title", "Billing").
			AssertJSONPath("data.collections.0.articles.0", "change-your-plan").
			AssertJSONPath("data.articles.0.body.0.kind", "h2").
			AssertJSONPath("data.articles.0.body.1.kind", "steps").
			AssertJSONPath("data.articles.0.body.1.items.0", "Open Billing").
			AssertJSONPath("data.articles.0.body.2.kind", "note")
		assert.NotContains(t, string(res.Body), "<strong>")
	})

	t.Run("lets the agent retrieve published articles, agent-only ones only for itself", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		for _, a := range []testkit.JSON{
			{"title": "Refund policy", "body_markdown": "## Refunds\n\nAll refunds take five business days."},
			{
				"title": "Refund approvals", "visibility": "agent_only",
				"body_markdown": "Refunds above 500 need a manager.",
			},
			{"title": "Draft refund notes", "body_markdown": "Refunds secret draft."},
		} {
			id := c.article(t, a).String("data.id")
			if a["title"] != "Draft refund notes" {
				c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
			}
		}
		ws := c.d.TC.Workspace.Workspace.ID
		res, err := c.d.TC.App.Knowledge.Search(t.Context(), ws, "how long do refunds take",
			contracts.KnowledgeSearchOptions{Visibility: []contracts.KnowledgeVisibility{"public", "agent_only"}})
		require.NoError(t, err)
		require.NotEmpty(t, res.Chunks)
		titles := []string{}
		for _, ch := range res.Chunks {
			titles = append(titles, ch.Title)
			assert.NotEqual(t, "Draft refund notes", ch.Title)
		}
		assert.Contains(t, titles, "Refund approvals")
		assert.Equal(t, "Refund policy", res.Chunks[0].Title)
		assert.Contains(t, res.Chunks[0].URL, "/hc/"+c.subdomain+"/articles/refund-policy")

		public, err := c.d.TC.App.Knowledge.Search(t.Context(), ws, "refunds manager",
			contracts.KnowledgeSearchOptions{Visibility: []contracts.KnowledgeVisibility{"public"}})
		require.NoError(t, err)
		for _, ch := range public.Chunks {
			assert.NotEqual(t, "Refund approvals", ch.Title)
		}
	})

	t.Run("answers on its own subdomain of the help domain", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_HELP_CENTERS__BASE_DOMAIN=help.example.test"))
		owner := tc.In(tc.Owner)
		hc := owner.GetJSON("/api/v1/help-centers").AssertOK()
		sub := hc.String("data.0.subdomain")
		hc.AssertJSONPath("data.0.url", "https://"+sub+".help.example.test")
		id := owner.PostJSON("/api/v1/help-centers/"+hc.String("data.0.id")+"/articles",
			testkit.JSON{"title": "Hosted page", "body_markdown": "Served on its own host."}).AssertCreated().String("data.id")
		owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()

		page := tc.Client().WithHeader("Accept", "text/html").
			Send("GET", "https://"+sub+".help.example.test/articles/hosted-page", nil).AssertOK()
		assert.Contains(t, string(page.Body), "Served on its own host.")
		assert.Contains(t, string(page.Body), `href="/articles/hosted-page.md"`)
	})
}
