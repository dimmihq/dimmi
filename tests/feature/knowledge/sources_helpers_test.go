package knowledge_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
site is the address of the docs site the tests crawl.
*/
const site = "https://docs.hivy.test"

/*
page is one page of the docs site.
*/
type page struct {
	path  string
	title string
	body  string
}

/*
html renders a server-rendered docs page with navigation around the
main content, the way real docs sites are built.
*/
func (p page) html() string {
	return fmt.Sprintf(`<!doctype html><html lang="en"><head><title>%s | Hivy Docs</title>
<link rel="canonical" href="%s%s"></head><body>
<nav><a href="/">Home</a> <a href="/docs/billing">Billing</a> <a href="https://elsewhere.test/x">Out</a></nav>
<main><article><h1>%s</h1><p>%s</p>
<h2>Steps</h2><ol><li>Open Settings and choose the page you need to change.</li>
<li>Follow the steps on screen and save when you are done.</li></ol>
<p>Every change applies to the whole workspace at once, and teammates see it the next time they load the page.</p>
</article></main><footer>Copyright Hivy</footer></body></html>`, p.title, site, p.path, p.title, p.body)
}

/*
serveSite answers the docs site's robots.txt, sitemap and pages through
the application's HTTP client.
*/
func serveSite(t *testing.T, x index, pages []page) *httpclienttest.Fake {
	t.Helper()

	fake := httpclienttest.New(x.tc.App.HTTP).PreventStrayRequests()
	fake.Stub("GET "+site+"/robots.txt", httpclient.Response{
		Status: 200, Body: []byte("User-agent: *\nDisallow: /private/\n"), Header: textHeader("text/plain"),
	})

	var urls strings.Builder
	for _, p := range pages {
		fmt.Fprintf(&urls, "<url><loc>%s%s</loc></url>", site, p.path)
		fake.Stub("GET "+site+p.path, httpclient.Response{
			Status: 200, Body: []byte(p.html()), Header: textHeader("text/html; charset=utf-8"),
		})
	}

	fake.Stub("GET "+site+"/", httpclient.Response{Status: 404})
	fake.Stub("GET "+site+"/sitemap.xml", httpclient.Response{
		Status: 200, Header: textHeader("application/xml"),
		Body: []byte(`<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
			urls.String() + `</urlset>`),
	})

	return fake
}

func textHeader(kind string) fhttp.Header {
	return fhttp.Header{"Content-Type": {kind}}
}

/*
docsPages are the three pages of the docs site.
*/
func docsPages() []page {
	return []page{
		{
			path: "/docs/refunds", title: "Refund policy",
			body: "Refunds are paid within fourteen days to the original card.",
		},
		{
			path: "/docs/billing", title: "Change your plan",
			body: "Upgrade or downgrade the plan from the billing page.",
		},
		{path: "/docs/time-off", title: "Request time off", body: "Ask your manager for leave with the time off form."},
	}
}

/*
addWebsite adds a website source through the API and syncs it now.
*/
func addWebsite(t *testing.T, x index, extra testkit.JSON) string {
	t.Helper()

	body := testkit.JSON{
		"type": "website", "name": "Hivy docs", "settings": testkit.JSON{"start_url": site + "/docs/refunds"},
		"sync_now": true,
	}
	maps.Copy(body, extra)

	return x.owner.PostJSON("/api/v1/knowledge/sources", body).AssertCreated().String("data.id")
}

/*
settle runs the queue until the source's sync ends, moving the clock on
so paced requests get their turn.
*/
func settle(t *testing.T, x index, source string) *testkit.Response {
	t.Helper()

	for range 30 {
		x.tc.Clock.Advance(2 * time.Second)
		x.tc.DrainLater()

		res := x.owner.GetJSON("/api/v1/knowledge/sources/" + source).AssertOK()
		if res.String("data.status") != "syncing" {
			return res
		}
	}

	require.Fail(t, "the sync never ended")

	return nil
}

/*
newSources boots a workspace with the built-in embedder answered by a
fake and the crawler unpaced, since the test clock stands still.
*/
func newSources(t *testing.T) index {
	t.Helper()

	return boot(t, tests.WithEnv(
		"DIMMI_AI__MANAGED__EMBEDDINGS__API_KEY=test-key", "DIMMI_AI__KNOWLEDGE__CRAWL_INTERVAL_MS=0",
	))
}

/*
markdownZip is a handbook of two Markdown files, one a draft, and a
picture the sync skips.
*/
func markdownZip(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	w := zip.NewWriter(&buf)
	files := map[string]string{
		"people/parental-leave.md": "---\ntitle: Parental leave\n---\nEvery parent gets sixteen weeks of paid leave.\n",
		"people/expenses.md":       "# Expenses\n\nClaim travel within thirty days with receipts.\n",
		"drafts/next-year.md":      "---\ntitle: Next year\ndraft: true\n---\nNothing yet.\n",
		"logo.png":                 "not text",
	}

	for name, body := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(body))
		require.NoError(t, err)
	}

	require.NoError(t, w.Close())

	return buf.Bytes()
}
