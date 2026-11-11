package knowledgesources_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/notion"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/website"
	"github.com/dimmihq/dimmi/tests/sourcetest"
)

func TestPatterns(t *testing.T) {
	t.Parallel()

	t.Run("a single star stays in a segment and a double star crosses them", func(t *testing.T) {
		t.Parallel()

		assert.True(t, sources.Match("/docs/**", "/docs/a/b"))
		assert.True(t, sources.Match("docs/**", "/docs"))
		assert.True(t, sources.Match("**/*.pdf", "/a/b/c.pdf"))
		assert.False(t, sources.Match("/docs/*", "/docs/a/b"))
		assert.True(t, sources.Match("/docs/*", "/docs/a"))
	})

	t.Run("exclusions win and no includes keeps everything", func(t *testing.T) {
		t.Parallel()

		assert.True(t, sources.Allowed(nil, nil, "/x"))
		assert.False(t, sources.Allowed([]string{"/docs/**"}, nil, "/blog/x"))
		assert.False(t, sources.Allowed([]string{"/docs/**"}, []string{"/docs/old/**"}, "/docs/old/x"))
	})
}

func TestMarkdownFiles(t *testing.T) {
	t.Parallel()

	t.Run("front matter names the file and marks drafts", func(t *testing.T) {
		t.Parallel()

		doc := sources.MarkdownFile("guides/setup.md", []byte("---\ntitle: Setup\ndraft: true\n---\nBody\n"))
		assert.Equal(t, "Setup", doc.Title)
		assert.True(t, doc.Draft)
		assert.Equal(t, "Body", doc.Markdown)
	})

	t.Run("a top heading or the file name is the title and mdx keeps only its text", func(t *testing.T) {
		t.Parallel()

		doc := sources.MarkdownFile("a/getting-started.mdx",
			[]byte("import X from 'x'\n\n# Start here\n\n<Note type=\"info\">Read {props.name} first</Note>\n"))
		assert.Equal(t, "Start here", doc.Title)
		assert.Equal(t, "Read  first", doc.Markdown)
		assert.Equal(t, "Getting started", sources.MarkdownFile("getting-started.md", []byte("text")).Title)
		assert.Equal(t, []sources.Collection{{ExternalID: "a", Name: "A"}}, sources.Folders("a/x.md"))
	})

	t.Run("html keeps its structure and lists its pictures", func(t *testing.T) {
		t.Parallel()

		md, assets, err := sources.HTMLToMarkdown(`<h2>Pay</h2><p>Use <b>card</b>.</p><img src="https://x.test/a.png" alt="A">` +
			`<img src="data:image/png;base64,xx">`)
		require.NoError(t, err)
		assert.Contains(t, md, "## Pay")
		assert.Contains(t, md, "**card**")
		assert.Equal(t, []sources.Asset{{URL: "https://x.test/a.png", Name: "A"}}, assets)
	})
}

func TestDriverErrors(t *testing.T) {
	t.Parallel()

	t.Run("answers map to the shared errors", func(t *testing.T) {
		t.Parallel()

		assert.ErrorIs(t, sources.FromResponse(&httpclient.Response{Status: 401}, "x"), sources.ErrUnauthorized)
		assert.ErrorIs(t, sources.FromResponse(&httpclient.Response{Status: 404}, "x"), sources.ErrNotFound)
		assert.ErrorIs(t, sources.FromResponse(&httpclient.Response{Status: 503}, "x"), sources.ErrUnavailable)
		assert.NoError(t, sources.FromResponse(&httpclient.Response{Status: 204}, "x"))

		var limited *sources.RateLimitError
		answer := &httpclient.Response{Status: 429, Header: map[string][]string{"Retry-After": {"7200"}}}
		err := sources.FromResponse(answer, "x")
		require.ErrorAs(t, err, &limited)
		assert.Equal(t, time.Hour, limited.RetryAfter, "a long wait is capped")
		assert.Equal(t, time.Minute, sources.RetryAfter("soon"))
		assert.Contains(t, sources.Excluded("robots").Error(), "robots")
	})
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("types are unique, well formed and fully described", func(t *testing.T) {
		t.Parallel()

		r := sources.NewRegistry()
		client := sourcetest.Client(t)
		require.NoError(t, r.Register(notion.New(client)))
		assert.ErrorIs(t, r.Register(notion.New(client)), sources.ErrDuplicate)
		assert.ErrorIs(t, r.Register(stub{sources.Description{Type: "Bad Type"}}), sources.ErrInvalidType)
		assert.ErrorIs(t, r.Register(stub{sources.Description{Type: "half_done"}}), sources.ErrIncomplete)

		d, ok := r.Driver("notion")
		require.True(t, ok)
		assert.True(t, d.Describe().Supports(sources.ModeImport))
		assert.True(t, d.Describe().Feeds(sources.TargetOperator))
		assert.Len(t, r.Descriptions(), 1)
	})
}

type stub struct{ d sources.Description }

func (s stub) Describe() sources.Description { return s.d }
func (s stub) Check(context.Context, sources.Config) (sources.CheckResult, error) {
	return sources.CheckResult{}, nil
}

func (s stub) List(context.Context, sources.Config, string) (sources.Page, error) {
	return sources.Page{}, nil
}

func (s stub) Fetch(context.Context, sources.Config, sources.Ref) (sources.Document, error) {
	return sources.Document{}, nil
}

func TestWebsitePages(t *testing.T) {
	t.Parallel()

	site := func(t *testing.T, robots string, pages map[string]string) *website.Driver {
		t.Helper()

		client := sourcetest.Client(t)
		fake := sourcetestFake(t, client)
		fake.Stub("GET https://docs.acme.test/robots.txt", httpclient.Response{
			Status: 200, Body: []byte(robots), Header: map[string][]string{"Content-Type": {"text/plain"}},
		})
		for path, body := range pages {
			fake.Stub("GET https://docs.acme.test"+path, httpclient.Response{
				Status: 200, Body: []byte(body), Header: map[string][]string{"Content-Type": {"text/html"}},
			})
		}

		return website.New(website.NewBuiltin(client, clock.System(), 0))
	}
	cfg := sources.Config{JSON: json.RawMessage(`{"start_url":"https://docs.acme.test/"}`)}
	fetch := func(d *website.Driver, path string) (sources.Document, error) {
		return d.Fetch(t.Context(), cfg, sources.Ref{URL: "https://docs.acme.test" + path})
	}

	t.Run("main content becomes markdown with a clean title and same-site links", func(t *testing.T) {
		t.Parallel()

		body := strings.Repeat("Teams pay monthly or yearly and can switch at any time from settings. ", 6)
		page := `<html><head><title>Pricing | Acme</title>
<link rel="canonical" href="/pricing"></head><body><nav><a href="/">Home</a><a href="https://x.test">x</a></nav>
<main><h1>Pricing</h1><p>` + body + `</p><p><a href="/pricing?utm_source=x#top">Again</a></p></main></body></html>`
		d := site(t, "User-agent: *\nAllow: /\n", map[string]string{"/pricing": page})

		doc, err := fetch(d, "/pricing")
		require.NoError(t, err)
		assert.Equal(t, "Pricing", doc.Title)
		assert.Contains(t, doc.Markdown, "Teams pay monthly")
		assert.Equal(t, sources.VisibilityPublic, doc.Visibility)
		assert.Contains(t, doc.Links, "https://docs.acme.test/")
		assert.NotContains(t, strings.Join(doc.Links, " "), "x.test")
		assert.Contains(t, doc.Links, "https://docs.acme.test/pricing", "tracking parameters and fragments are dropped")
	})

	t.Run("robots, duplicates and app shells are set aside with their reason", func(t *testing.T) {
		t.Parallel()

		d := site(t, "User-agent: *\nDisallow: /private\n", map[string]string{
			"/copy": `<html><head><link rel="canonical" href="https://docs.acme.test/original"></head><body>x</body></html>`,
			"/app":  `<html><body><div id="root"></div><noscript>Please enable JavaScript</noscript></body></html>`,
		})

		reason := func(path string) string {
			_, err := fetch(d, path)
			if excluded, ok := errors.AsType[*sources.ExcludedError](err); ok {
				return excluded.Reason
			}
			return ""
		}

		assert.Equal(t, "robots", reason("/private/x"))
		assert.Equal(t, "duplicate", reason("/copy"))
		assert.Equal(t, "js_rendered", reason("/app"))
	})

	t.Run("addresses have one canonical form", func(t *testing.T) {
		t.Parallel()

		u, err := url.Parse("HTTPS://Docs.Acme.test:443/a?utm_medium=x&b=2&a=1#frag")
		require.NoError(t, err)
		assert.Equal(t, "https://docs.acme.test/a?a=1&b=2", website.Canonical(u))
	})
}

func sourcetestFake(t *testing.T, client *httpclient.Client) *httpclienttest.Fake {
	t.Helper()

	return httpclienttest.New(client).PreventStrayRequests()
}
