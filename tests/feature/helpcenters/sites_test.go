package helpcenters_test

import (
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/dns"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
browser is a reader's browser, which counts as a person reading.
*/
const browser = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 Chrome/130.0 Safari/537.36"

/*
checkDomains runs the checks of every domain due, as the queue would.
*/
func (c center) checkDomains(t *testing.T) {
	t.Helper()
	app := c.d.TC.App
	due, err := models.New(app.DB).ListHelpCenterDomainsDue(t.Context(), models.ListHelpCenterDomainsDueParams{
		DueAt: c.d.TC.Clock.Now().Add(time.Hour), RowLimit: 100,
	})
	require.NoError(t, err)
	for _, d := range due {
		if d.WorkspaceID == c.d.TC.Workspace.Workspace.ID {
			require.NoError(t, app.Actions.HelpDomains.Handle(t.Context(), d.WorkspaceID, d.ID))
		}
	}
}

func TestHelpCenterSites(t *testing.T) {
	t.Parallel()

	t.Run("adds a help center named into its subdomain and deletes it once its name is typed", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		res := c.d.Owner.PostJSON("/api/v1/help-centers", testkit.JSON{"name": "Acme API Docs", "locale": "fr"}).
			AssertCreated().AssertJSONPath("data.is_default", false).AssertJSONPath("data.locale", "fr")
		id := res.String("data.id")
		assert.True(t, strings.HasPrefix(res.String("data.subdomain"), "acme-api-docs"))
		c.d.Owner.GetJSON("/api/v1/help-centers").AssertOK().AssertJSONPath("data.1.id", id)

		c.d.Owner.PostJSON("/api/v1/help-centers", testkit.JSON{"name": "Other", "subdomain": "admin"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "subdomain_reserved")
		admin, _ := c.d.Join(t, enums.RoleAdmin)
		admin.Delete("/api/v1/help-centers/" + id + "?confirm=Acme+API+Docs").AssertStatus(fhttp.StatusForbidden)
		c.d.Owner.Delete("/api/v1/help-centers/"+id+"?confirm=Acme").
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "confirmation_mismatch")
		c.d.Owner.Delete("/api/v1/help-centers/"+c.id+"?confirm=x").
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "default_help_center")
		confirm := "?confirm=" + url.QueryEscape("Acme API Docs")
		c.d.Owner.Delete("/api/v1/help-centers/" + id + confirm).AssertNoContent()
		c.d.Owner.GetJSON("/api/v1/help-centers/" + id).AssertNotFound()
	})

	t.Run("serves a help center on its own domain once its records verify", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		domain := "/api/v1/help-centers/" + c.id + "/domain"
		c.d.Owner.GetJSON(domain).AssertOK().AssertJSONPath("data", nil)
		c.d.Owner.PatchJSON(domain, testkit.JSON{"hostname": "acme.test"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "domain_apex")

		res := c.d.Owner.PatchJSON(domain, testkit.JSON{"hostname": "https://Help.Acme.test/"}).AssertOK().
			AssertJSONPath("data.hostname", "help.acme.test").AssertJSONPath("data.status", "pending").
			AssertJSONPath("data.records.0.type", "CNAME").
			AssertJSONPath("data.records.1.name", "_dimmi.help.acme.test")
		target, txt := res.String("data.records.0.value"), res.String("data.records.1.value")
		require.True(t, strings.HasPrefix(txt, "dimmi-verification="))

		c.checkDomains(t)
		pending := c.d.Owner.GetJSON(domain).AssertOK().AssertJSONPath("data.status", "pending")
		assert.Contains(t, pending.String("data.failure_reason"), "No TXT record")
		require.ErrorIs(t, c.d.TC.App.AuthorizeDomain(t.Context(), "help.acme.test"), customdomain.ErrNotAuthorized)

		c.d.TC.App.DNS.Fake().Set("_dimmi.help.acme.test", dns.Records{TXT: []string{txt}}).
			Set("help.acme.test", dns.Records{CNAME: target + "."}).
			Set(target, dns.Records{Addrs: []netip.Addr{netip.MustParseAddr("203.0.113.7")}})
		c.d.Owner.PostJSON(domain+"/checks", nil).AssertOK()
		c.checkDomains(t)
		c.d.Owner.GetJSON(domain).AssertOK().AssertJSONPath("data.status", "verified")
		c.d.Owner.GetJSON("/api/v1/help-centers/"+c.id).AssertOK().
			AssertJSONPath("data.url", "https://help.acme.test").AssertJSONPath("data.custom_domain", "help.acme.test")
		require.NoError(t, c.d.TC.App.AuthorizeDomain(t.Context(), "help.acme.test"))

		page := c.d.TC.Client().WithHeader("Accept", "text/html").Send("GET", "https://help.acme.test/", nil).AssertOK()
		assert.Contains(t, string(page.Body), `<link rel="canonical" href="https://help.acme.test`)
		c.page("").AssertOK()

		c.d.Owner.Delete(domain).AssertNoContent()
		require.ErrorIs(t, c.d.TC.App.AuthorizeDomain(t.Context(), "help.acme.test"), customdomain.ErrNotAuthorized)
	})

	t.Run("refuses a domain another help center claimed", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		other := c.d.Owner.PostJSON("/api/v1/help-centers", testkit.JSON{"name": "Partner docs"}).AssertCreated().
			String("data.id")
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id+"/domain", testkit.JSON{"hostname": "docs.shared.test"}).
			AssertOK()
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+other+"/domain", testkit.JSON{"hostname": "docs.shared.test"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "domain_taken")
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id+"/domain", testkit.JSON{"hostname": "docs.other.test"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "domain_exists")
	})

	t.Run("lists published articles for crawlers and language models", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id := c.article(t, testkit.JSON{
			"title": "Reset your password", "description": "Get back in.", "body_markdown": "Open **Sign in**.",
		}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		c.article(t, testkit.JSON{"title": "Unreleased draft", "body_markdown": "Secret."})

		sitemap := string(c.page("/sitemap.xml").AssertOK().Body)
		assert.Contains(t, sitemap, "/articles/reset-your-password</loc>")
		assert.NotContains(t, sitemap, "unreleased-draft")
		assert.Contains(t, string(c.page("/robots.txt").AssertOK().Body), "/sitemap.xml")
		llms := string(c.page("/llms.txt").AssertOK().Body)
		assert.Contains(t, llms, "/articles/reset-your-password.md): Get back in.")
		assert.Contains(t, string(c.page("/llms-full.txt").AssertOK().Body), "Open **Sign in**.")

		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{"indexing_enabled": false}).AssertOK()
		assert.Contains(t, string(c.page("/robots.txt").AssertOK().Body), "Disallow: /\n")
		c.page("/sitemap.xml").AssertNotFound()
		assert.Contains(t, string(c.page("/articles/reset-your-password").AssertOK().Body), `content="noindex"`)
	})

	t.Run("counts views and searches that found nothing, never crawlers", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id := c.article(t, testkit.JSON{"title": "Export data", "body_markdown": "Use the export button."}).
			String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		reader := c.d.TC.Client().WithHeader("Accept", "text/html").WithHeader("User-Agent", browser)
		reader.Send("GET", c.address("/articles/export-data"), nil).AssertOK()
		reader.Send("GET", c.address("/articles/export-data"), nil).AssertOK()
		reader.Send("GET", c.address("/search?q=Refund+policy"), nil).AssertOK()
		c.d.TC.Client().WithHeader("User-Agent", "Googlebot/2.1").Send("GET", c.address("/articles/export-data"), nil).
			AssertOK()

		res := c.d.Owner.GetJSON("/api/v1/help-centers/" + c.id + "/analytics?days=7").AssertOK()
		res.AssertJSONPath("data.views", float64(2)).AssertJSONPath("data.top_articles.0.title", "Export data").
			AssertJSONPath("data.content_gaps.0.query", "refund policy").
			AssertJSONPath("data.empty_searches", float64(1))
		res.AssertJSONPath("data.daily.6.views", float64(2)).AssertJSONPath("data.helpful_percent", nil)
	})
}
