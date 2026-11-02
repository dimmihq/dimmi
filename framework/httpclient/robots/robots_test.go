package robots_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/robots"
)

/*
site serves the handler on loopback and returns its origin with a client
allowed to reach it.
*/
func site(t *testing.T, handler http.HandlerFunc) (string, *httpclient.Client) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	client, err := httpclient.New(httpclient.Config{AllowHosts: []string{u.Host}})
	require.NoError(t, err)

	return server.URL, client
}

func fetch(t *testing.T, status int, body string) *robots.Rules {
	t.Helper()

	origin, client := site(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)

			return
		}

		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})

	rules, err := robots.Fetch(t.Context(), client, origin+"/some/page?x=1")
	require.NoError(t, err)

	return rules
}

func TestMatching(t *testing.T) {
	t.Parallel()

	rules := fetch(t, http.StatusOK, strings.Join([]string{
		"User-Agent: *",
		"Disallow: *.gif$",
		"Disallow: /example/",
		"Allow: /publications/",
		"",
		"User-Agent: foobot",
		"Disallow: /",
		"Allow: /example/page.html",
		"Allow: /example/allowed.gif",
		"Crawl-delay: 2",
		"",
		"User-Agent: barbot",
		"User-Agent: bazbot",
		"Disallow: /example/page.html",
		"",
		"User-Agent: quxbot",
		"Disallow:",
	}, "\n"))

	cases := []struct {
		agent, path string
		allowed     bool
	}{
		{"foobot", "/example/page.html", true},
		{"foobot", "/example/allowed.gif", true},
		{"foobot", "/example/other.html", false},
		{"FooBot", "/", false},
		{"barbot", "/example/page.html", false},
		{"bazbot", "/example/page.html", false},
		{"bazbot", "/other", true},
		{"quxbot", "/anything", true},
		{"otherbot", "/example/page.html", false},
		{"otherbot", "/publications/1", true},
		{"otherbot", "/image.gif", false},
		{"otherbot", "/image.gif?size=2", true},
	}

	for _, c := range cases {
		t.Run(c.agent+" on "+c.path, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.allowed, rules.Allowed(c.agent, c.path))
		})
	}

	t.Run("reads the crawl delay of the matching group", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 2*time.Second, rules.CrawlDelay("foobot"))
		assert.Zero(t, rules.CrawlDelay("otherbot"))
	})
}

func TestFetch(t *testing.T) {
	t.Parallel()

	t.Run("allows everything when the file is missing", func(t *testing.T) {
		t.Parallel()

		assert.True(t, fetch(t, http.StatusNotFound, "").Allowed("bot", "/private"))
		assert.True(t, fetch(t, http.StatusForbidden, "").Allowed("bot", "/private"))
	})

	t.Run("allows nothing on a server error", func(t *testing.T) {
		t.Parallel()

		assert.False(t, fetch(t, http.StatusServiceUnavailable, "").Allowed("bot", "/"))
	})

	t.Run("allows nothing on a status the protocol does not cover", func(t *testing.T) {
		t.Parallel()

		assert.False(t, fetch(t, http.StatusNotModified, "").Allowed("bot", "/"))
	})

	t.Run("allows nothing when the file cannot be parsed", func(t *testing.T) {
		t.Parallel()

		assert.False(t, fetch(t, http.StatusOK, "User-agent: *\nCrawl-delay: soon\n").Allowed("bot", "/"))
	})

	t.Run("allows nothing when the host is refused or unreachable", func(t *testing.T) {
		t.Parallel()
		origin, client := site(t, func(http.ResponseWriter, *http.Request) {})
		guarded, err := httpclient.New(httpclient.Config{})
		require.NoError(t, err)

		blocked, err := robots.Fetch(t.Context(), guarded, origin)
		require.NoError(t, err)
		down, err := robots.Fetch(t.Context(), client, "http://127.0.0.1:1")
		require.NoError(t, err)

		assert.False(t, blocked.Allowed("bot", "/"))
		assert.False(t, down.Allowed("bot", "/"))
	})

	t.Run("allows nothing when the body breaks off", func(t *testing.T) {
		t.Parallel()
		origin, client := site(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, "User-agent: *\n")
		})

		rules, err := robots.Fetch(t.Context(), client, origin)

		require.NoError(t, err)
		assert.False(t, rules.Allowed("bot", "/"))
	})

	t.Run("reads only the first part of a huge file", func(t *testing.T) {
		t.Parallel()
		body := "User-agent: *\nDisallow: /private\n#" + strings.Repeat("x", robots.MaxSize) + "\nDisallow: /\n"

		rules := fetch(t, http.StatusOK, body)

		assert.False(t, rules.Allowed("bot", "/private"))
		assert.True(t, rules.Allowed("bot", "/public"))
	})

	t.Run("follows a redirect to the file", func(t *testing.T) {
		t.Parallel()
		origin, client := site(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/robots.txt" {
				http.Redirect(w, r, "/real-robots.txt", http.StatusMovedPermanently)

				return
			}

			_, _ = io.WriteString(w, "User-agent: *\nDisallow: /\n")
		})

		rules, err := robots.Fetch(t.Context(), client, origin)

		require.NoError(t, err)
		assert.False(t, rules.Allowed("bot", "/"))
	})

	t.Run("rejects an origin that is not a web url", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{})
		require.NoError(t, err)

		for _, origin := range []string{"example.com", "ftp://example.com", "https://", "http://bad host"} {
			_, err := robots.Fetch(t.Context(), client, origin)
			require.ErrorIs(t, err, robots.ErrInvalidOrigin, origin)
		}
	})

	t.Run("stops when the context ends", func(t *testing.T) {
		t.Parallel()
		origin, client := site(t, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := robots.Fetch(ctx, client, origin)

		require.ErrorIs(t, err, context.Canceled)
	})
}
