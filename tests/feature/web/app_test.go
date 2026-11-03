package web_test

import (
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/tests"
)

const manifest = `{
  "resources/js/app.tsx": {
    "file": "assets/app-4f2a.js",
    "src": "resources/js/app.tsx",
    "isEntry": true,
    "css": ["assets/app-77aa.css"]
  }
}`

/*
withBuild gives the application a production build of the browser app.
*/
func withBuild() tests.Option {
	return tests.Before(func(a *app.App) {
		a.Frontend = fstest.MapFS{
			".vite/manifest.json": &fstest.MapFile{Data: []byte(manifest)},
			"assets/app-4f2a.js":  &fstest.MapFile{Data: []byte("console.log('app')")},
			"assets/app-77aa.css": &fstest.MapFile{Data: []byte("body{}")},
		}
	})
}

/*
withDevServer points the application at a running Vite dev server.
*/
func withDevServer(t *testing.T) tests.Option {
	t.Helper()

	return tests.WithEnv("DIMMI_FRONTEND__HOT_PATH=" + testkit.TempFile(t, "hot", "http://localhost:5173"))
}

/*
nonceOf returns the nonce the policy allows scripts with.
*/
func nonceOf(t *testing.T, policy string) string {
	t.Helper()

	match := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(policy)
	require.Len(t, match, 2, policy)

	return match[1]
}

func TestAppPage(t *testing.T) {
	t.Parallel()

	t.Run("serves the app page with the build's tags under a strict policy", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withBuild())

		res := tc.Client().WithHeader("Accept", "text/html").Send(fhttp.MethodGet, "/d", nil).AssertOK()

		policy := res.Header.Get("Content-Security-Policy")
		nonce := nonceOf(t, policy)
		body := string(res.Body)
		assert.Contains(t, body, `<script nonce="`+nonce+`" type="module" src="/build/assets/app-4f2a.js"></script>`)
		assert.Contains(t, body, `<link nonce="`+nonce+`" rel="stylesheet" href="/build/assets/app-77aa.css">`)
		assert.Contains(t, body, `<div id="root"></div>`)
		assert.Contains(t, body, `<meta name="robots" content="noindex">`)
		assert.Contains(t, body, `<html lang="en">`)
		assert.NotContains(t, policy, "localhost:5173")
		assert.NotContains(t, policy, "unsafe-inline")
		assert.Contains(t, policy, "frame-ancestors 'none'")
		assert.Equal(t, "no-cache", res.Header.Get("Cache-Control"))
	})

	t.Run("serves the same page for every path of the app, with a fresh nonce each time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withBuild())

		first := tc.Client().Send(fhttp.MethodGet, "/d/inbox/conversations/42", nil).AssertOK()
		second := tc.Client().Send(fhttp.MethodGet, "/d/settings", nil).AssertOK()

		assert.Contains(t, string(first.Body), `<div id="root"></div>`)
		assert.NotEqual(t, nonceOf(t, first.Header.Get("Content-Security-Policy")),
			nonceOf(t, second.Header.Get("Content-Security-Policy")))
	})

	t.Run("loads the app from the dev server while it runs, with hot updates allowed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withBuild(), withDevServer(t))

		res := tc.Client().Send(fhttp.MethodGet, "/d", nil).AssertOK()

		body := string(res.Body)
		assert.Contains(t, body, `src="http://localhost:5173/@vite/client"`)
		assert.Contains(t, body, `src="http://localhost:5173/resources/js/app.tsx"`)
		assert.Contains(t, body, "@react-refresh")
		assert.NotContains(t, body, "/build/assets")
		policy := res.Header.Get("Content-Security-Policy")
		assert.Contains(t, policy, "script-src 'self' 'nonce-")
		assert.Contains(t, policy, "connect-src 'self' http://localhost:5173 ws://localhost:5173")
	})

	t.Run("fails the page of a binary built without the app", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Frontend = fstest.MapFS{} }))

		tc.Client().Send(fhttp.MethodGet, "/d", nil).AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("sends visitors of the home page to the app", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withBuild())

		res := tc.Client().Send(fhttp.MethodGet, "/", nil).AssertStatus(fhttp.StatusFound)

		assert.Equal(t, "/d", res.Header.Get("Location"))
	})
}

func TestBuildFiles(t *testing.T) {
	t.Parallel()

	t.Run("serves built files to be cached for good", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withBuild())

		res := tc.Client().Send(fhttp.MethodGet, "/build/assets/app-4f2a.js", nil).AssertOK()

		assert.Equal(t, "console.log('app')", string(res.Body))
		assert.Equal(t, "public, max-age=31536000, immutable", res.Header.Get("Cache-Control"))
	})

	t.Run("answers not found for a file the build does not have", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withBuild())

		tc.Client().Send(fhttp.MethodGet, "/build/assets/missing.js", nil).AssertNotFound()
	})
}
