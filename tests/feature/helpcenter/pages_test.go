package helpcenter_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/tests"
)

const manifest = `{
  "resources/js/portal/portal.ts": {
    "file": "assets/portal-4f2a.js",
    "src": "resources/js/portal/portal.ts",
    "isEntry": true
  },
  "resources/js/portal/portal.css": {
    "file": "assets/portal-9c1d.css",
    "src": "resources/js/portal/portal.css",
    "isEntry": true
  }
}`

/*
withBuild gives the application a production build holding the public
pages' stylesheet.
*/
func withBuild() tests.Option {
	return tests.Before(func(a *app.App) {
		a.Frontend = fstest.MapFS{
			".vite/manifest.json":    &fstest.MapFile{Data: []byte(manifest)},
			"assets/portal-4f2a.js":  &fstest.MapFile{Data: []byte("turbo()")},
			"assets/portal-9c1d.css": &fstest.MapFile{Data: []byte("body{}")},
		}
	})
}

/*
get fetches a page and returns its status and body.
*/
func get(t *testing.T, tc *tests.TestCase, path string) (int, string) {
	t.Helper()

	res := tc.Client().WithHeader("Accept", "text/html").Send(fhttp.MethodGet, path, nil)

	return res.Status, string(res.Body)
}

func TestHelpCenterPages(t *testing.T) {
	t.Parallel()

	tc := tests.New(t, withBuild())

	t.Run("shows the home page in the portal's brand with its script and sections", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/acme")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, `style="--brand: #5b5bd6;"`)
		assert.Contains(t, body, `href="/build/assets/portal-9c1d.css"`)
		assert.Contains(t, body, `type="module" src="/build/assets/portal-4f2a.js"`)
		assert.Contains(t, body, `<meta name="view-transition" content="same-origin">`)
		assert.Contains(t, body, "How can we help?")
		assert.Contains(t, body, `href="/hc/acme/collections/getting-started"`)
		assert.Contains(t, body, `href="/hc/acme/feedback"`)
		assert.Contains(t, body, `href="/hc/acme/sign-in"`)
		assert.Contains(t, body, "Shared inboxes for every team")
	})

	t.Run("draws a dark brand's logo from its files", func(t *testing.T) {
		t.Parallel()

		_, body := get(t, tc, "/hc/dimmi")

		assert.Contains(t, body, `src="/hc/dimmi/assets/dimmi-logo-light.svg"`)
		assert.Contains(t, body, `src="/hc/dimmi/assets/dimmi-logo-dark.svg"`)
	})

	t.Run("lists a collection with its sub-collections and who writes them", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/dimmi/collections/messenger")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, "Customize the messenger")
		assert.Contains(t, body, "Installation")
		assert.Contains(t, body, "Install the messenger on Next.js")
		assert.Contains(t, body, "By Ama Owusu and 1 other")
		assert.Contains(t, body, "5 articles")
	})

	t.Run("shows a sub-collection under its parent's trail", func(t *testing.T) {
		t.Parallel()

		_, body := get(t, tc, "/hc/dimmi/collections/messenger-install")

		parent := `<a href="/hc/dimmi/collections/messenger" class="[^"]*">Messenger</a>`
		assert.Regexp(t, parent+`</li><li aria-hidden`, body)
	})

	t.Run("renders an article with its contents, links under the base and neighbors", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/dimmi/articles/install-on-nextjs")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, `<h2 id="install-the-package">Install the package</h2>`)
		assert.Contains(t, body, `href="#add-the-provider"`)
		assert.Contains(t, body, `href="/hc/dimmi/articles/identify-your-users"`)
		assert.Contains(t, body, `class="chroma-chroma"`)
		assert.Contains(t, body, "Install with a script tag")
		assert.Contains(t, body, `type="text/markdown" href="/hc/dimmi/articles/install-on-nextjs.md">`)
		assert.Contains(t, body, "Was this article helpful?")
		assert.Contains(t, body, `data-copy="/hc/dimmi/articles/install-on-nextjs.md"`)
		assert.Contains(t, body, "https://claude.ai/new?q=Read+http%3A%2F%2Fexample.com%2Fhc%2Fdimmi%2Farticles%2Finstall-on-nextjs.md")
	})

	t.Run("serves an article's Markdown twin", func(t *testing.T) {
		t.Parallel()

		res := tc.Client().Send(fhttp.MethodGet, "/hc/dimmi/articles/invite-your-team.md", nil).AssertOK()

		assert.Equal(t, "text/markdown; charset=utf-8", res.Header.Get("Content-Type"))
		assert.Contains(t, string(res.Body), "# Invite your team\n\n> Add teammates")
	})

	t.Run("thanks a yes and asks what was missing after a no", func(t *testing.T) {
		t.Parallel()

		res := tc.Client().Send(fhttp.MethodPost, "/hc/dimmi/articles/invite-your-team/feedback?helpful=no", nil)

		assert.Equal(t, fhttp.StatusSeeOther, res.Status)
		assert.Equal(t, "/hc/dimmi/articles/invite-your-team?helpful=no#feedback", res.Header.Get("Location"))

		_, body := get(t, tc, "/hc/dimmi/articles/invite-your-team?helpful=no")
		assert.Contains(t, body, "What were you looking for?")
		assert.NotContains(t, body, `data-dimmi="open"`)
		_, body = get(t, tc, "/hc/dimmi/articles/invite-your-team?helpful=yes")
		assert.Contains(t, body, "Thanks for letting us know.")
	})

	t.Run("finds articles matching every word of a search, title matches first", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/dimmi/search?q=custom+domain")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, `<meta name="robots" content="noindex">`)
		assert.Contains(t, body, "for ")
		assert.Regexp(t, `(?s)Use a custom domain.*Plans and pricing`, body)
	})

	t.Run("says when a search finds nothing and when nothing was asked", func(t *testing.T) {
		t.Parallel()

		_, none := get(t, tc, "/hc/dimmi/search?q=zebra+crossing")
		_, empty := get(t, tc, "/hc/dimmi/search")

		assert.Contains(t, none, "No answers for “zebra crossing”")
		assert.Contains(t, empty, "Ask a question or type a few words")
	})

	t.Run("answers unknown pages with the help center's not found page", func(t *testing.T) {
		t.Parallel()

		for _, path := range []string{
			"/hc/dimmi/articles/nope", "/hc/dimmi/collections/nope", "/hc/dimmi/elsewhere",
		} {
			status, body := get(t, tc, path)

			assert.Equal(t, fhttp.StatusNotFound, status, path)
			assert.Contains(t, body, "We couldn't find that page", path)
		}
	})

	t.Run("reveals nothing about a help center that does not exist", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/nobody")

		assert.Equal(t, fhttp.StatusNotFound, status)
		assert.Empty(t, body)
	})

	t.Run("serves the sample's images", func(t *testing.T) {
		t.Parallel()

		res := tc.Client().Send(fhttp.MethodGet, "/hc/dimmi/assets/inboxes.svg", nil).AssertOK()

		assert.Equal(t, "image/svg+xml", res.Header.Get("Content-Type"))
	})
}

func TestChangelogPages(t *testing.T) {
	t.Parallel()

	tc := tests.New(t, withBuild())

	t.Run("lists the newest entries in full and the rest as summaries", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/dimmi/changelog")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, "The latest dimmi releases and improvements.")
		assert.Contains(t, body, `aria-label="Releases in 2026"`)
		assert.Contains(t, body, `title="Voice notes in the messenger"`)
		assert.Contains(t, body, `<time datetime="2026-10-30"`)
		assert.Contains(t, body, "v1.6.0")
		assert.Contains(t, body, `<h2 id="office-hours-per-inbox">`)
		assert.Contains(t, body, "Read more")
		assert.Contains(t, body, `href="/hc/dimmi/changelog/page/2"`)
		assert.NotContains(t, body, "Newer updates")
	})

	t.Run("pages through older entries", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/dimmi/changelog/page/2")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, "Snooze until the customer replies")
		assert.Contains(t, body, `href="/hc/dimmi/changelog"`)
		assert.NotContains(t, body, "Older updates")
	})

	t.Run("filters entries to a label", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/dimmi/changelog/labels/fixed")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, "<title>Fixed · Changelog | dimmi</title>")
		assert.Regexp(t, `<a href="/hc/dimmi/changelog/labels/fixed" aria-current="page"`, body)
		assert.Contains(t, body, "Fixes and polish")
		assert.NotContains(t, body, "Voice notes in the messenger")
	})

	t.Run("does not page past the end, before the start or into an unknown label", func(t *testing.T) {
		t.Parallel()

		for _, path := range []string{
			"/hc/dimmi/changelog/page/9", "/hc/dimmi/changelog/page/1", "/hc/dimmi/changelog/page/two",
			"/hc/dimmi/changelog/labels/nope", "/hc/dimmi/changelog/nope",
		} {
			status, _ := get(t, tc, path)

			assert.Equal(t, fhttp.StatusNotFound, status, path)
		}
	})

	t.Run("shows an entry with its related articles and the entries around it", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/dimmi/changelog/agent-cites-sources")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, "dimmi agent now cites its sources")
		assert.Contains(t, body, "How dimmi agent answers questions")
		assert.Contains(t, body, "Newer · Oct 30")
		assert.Contains(t, body, "Older · Oct 16")
		assert.Contains(t, body, `aria-label="Thumbs up, 28"`)
	})

	t.Run("serves an entry's Markdown twin", func(t *testing.T) {
		t.Parallel()

		res := tc.Client().Send(fhttp.MethodGet, "/hc/dimmi/changelog/voice-notes.md", nil).AssertOK()

		assert.Contains(t, string(res.Body), "# Voice notes in the messenger")
	})

	t.Run("asks a new subscriber to confirm from their inbox", func(t *testing.T) {
		t.Parallel()

		res := tc.Client().Send(fhttp.MethodPost, "/hc/dimmi/changelog/subscriptions", nil)

		assert.Equal(t, fhttp.StatusSeeOther, res.Status)
		assert.Equal(t, "/hc/dimmi/changelog?subscribed#main", res.Header.Get("Location"))

		_, body := get(t, tc, "/hc/dimmi/changelog?subscribed")
		assert.Contains(t, body, "Check your inbox to confirm your subscription.")
	})

	t.Run("returns to the entry after a reaction", func(t *testing.T) {
		t.Parallel()

		res := tc.Client().Send(fhttp.MethodPost, "/hc/dimmi/changelog/voice-notes/reactions", nil)

		assert.Equal(t, fhttp.StatusSeeOther, res.Status)
		assert.Equal(t, "/hc/dimmi/changelog/voice-notes", res.Header.Get("Location"))
	})
}

func TestPortalPages(t *testing.T) {
	t.Parallel()

	tc := tests.New(t, withBuild())

	t.Run("lists feature requests by status with their votes", func(t *testing.T) {
		t.Parallel()

		status, body := get(t, tc, "/hc/dimmi/feedback")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, body, "In progress")
		assert.Contains(t, body, `aria-label="Vote for WhatsApp channel, 58 votes"`)
		assert.Contains(t, body, `aria-pressed="true"`)
	})

	t.Run("takes an idea and a vote and returns to the list", func(t *testing.T) {
		t.Parallel()

		status, form := get(t, tc, "/hc/dimmi/feedback/new")
		idea := tc.Client().Send(fhttp.MethodPost, "/hc/dimmi/feedback", nil)
		vote := tc.Client().Send(fhttp.MethodPost, "/hc/dimmi/feedback/jira/votes", nil)
		_, thanked := get(t, tc, "/hc/dimmi/feedback?submitted")

		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, form, `action="/hc/dimmi/feedback"`)
		assert.Equal(t, "/hc/dimmi/feedback?submitted", idea.Header.Get("Location"))
		assert.Equal(t, "/hc/dimmi/feedback", vote.Header.Get("Location"))
		assert.Contains(t, thanked, "We read every idea")
	})

	t.Run("sends a visitor who is not signed in to sign in", func(t *testing.T) {
		t.Parallel()

		res := tc.Client().Send(fhttp.MethodGet, "/hc/dimmi/conversations", nil)
		status, page := get(t, tc, "/hc/dimmi/sign-in")
		_, ask := get(t, tc, "/hc/dimmi/conversations/new")

		assert.Equal(t, fhttp.StatusSeeOther, res.Status)
		assert.Equal(t, "/hc/dimmi/sign-in", res.Header.Get("Location"))
		assert.Equal(t, fhttp.StatusOK, status)
		assert.Contains(t, page, "Email me a sign-in link")
		assert.Contains(t, ask, `name="email"`)
	})

	t.Run("signs a visitor in to their conversations and out again", func(t *testing.T) {
		t.Parallel()

		res := tc.Client().Send(fhttp.MethodPost, "/hc/dimmi/sign-in", nil)
		cookie := res.Header.Get("Set-Cookie")
		signedIn := tc.Client().WithHeader("Cookie", "dimmi_portal_visitor=1")

		list := signedIn.Send(fhttp.MethodGet, "/hc/dimmi/conversations", nil).AssertOK()
		thread := signedIn.Send(fhttp.MethodGet, "/hc/dimmi/conversations/c-1987", nil).AssertOK()
		missing := signedIn.Send(fhttp.MethodGet, "/hc/dimmi/conversations/c-0", nil)
		again := signedIn.Send(fhttp.MethodGet, "/hc/dimmi/sign-in", nil)
		ask := signedIn.Send(fhttp.MethodGet, "/hc/dimmi/conversations/new", nil).AssertOK()
		out := signedIn.Send(fhttp.MethodPost, "/hc/dimmi/sign-out", nil)

		assert.Equal(t, "/hc/dimmi/conversations", res.Header.Get("Location"))
		assert.Contains(t, cookie, "dimmi_portal_visitor=1; Path=/hc/dimmi; HttpOnly; SameSite=Lax")
		assert.Contains(t, string(list.Body), "Webhook retries after a timeout")
		assert.Contains(t, string(list.Body), "Waiting on you")
		assert.Contains(t, string(thread.Body), "certificate provider")
		assert.Equal(t, fhttp.StatusNotFound, missing.Status)
		assert.Equal(t, "/hc/dimmi/conversations", again.Header.Get("Location"))
		assert.NotContains(t, string(ask.Body), `name="email"`)
		assert.Equal(t, "/hc/dimmi", out.Header.Get("Location"))
		assert.Contains(t, out.Header.Get("Set-Cookie"), "Max-Age=0")
	})

	t.Run("returns to the conversation after a reply or a new message", func(t *testing.T) {
		t.Parallel()

		reply := tc.Client().Send(fhttp.MethodPost, "/hc/dimmi/conversations/c-1987/messages", nil)
		start := tc.Client().Send(fhttp.MethodPost, "/hc/dimmi/conversations", nil)

		assert.Equal(t, "/hc/dimmi/conversations/c-1987", reply.Header.Get("Location"))
		assert.Equal(t, "/hc/dimmi/conversations", start.Header.Get("Location"))
	})
}
