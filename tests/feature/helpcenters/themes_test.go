package helpcenters_test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
zipOf packs files into a theme's zip.
*/
func zipOf(tb testing.TB, files map[string]string) []byte {
	tb.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, body := range files {
		f, err := w.Create(name)
		require.NoError(tb, err)
		_, err = f.Write([]byte(body))
		require.NoError(tb, err)
	}
	require.NoError(tb, w.Close())
	return b.Bytes()
}

/*
upload sends a theme's zip for the help center.
*/
func (c center) upload(files map[string]string) *testkit.Response {
	return c.d.Owner.PostForm("/api/v1/help-centers/"+c.id+"/themes",
		testkit.FormFile{Field: "file", Name: "theme.zip", Contents: zipOf(c.d.TC.T, files)}, nil)
}

/*
hosted says whether the help center's pages are served on its own host,
where a team's theme runs its scripts.
*/
func (c center) hosted() bool { return c.d.TC.App.Config.HelpCenters.BaseDomain != "" }

var assetLink = regexp.MustCompile(`href="([^"]*theme-assets/[^"]+theme\.css)"`)

func TestHelpCenterThemes(t *testing.T) {
	t.Parallel()

	t.Run("refuses a theme with problems, naming each with its file and line", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		res := c.upload(map[string]string{
			"theme.json":            `{"api_version": 1, "name": "Broken"}`,
			"templates/home.liquid": "<h1>{{ help_center.name }</h1>\n{% if x %}",
			"templates/blog.liquid": "nope",
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "theme_invalid")
		body := string(res.Body)
		assert.Contains(t, body, "templates/home.liquid")
		assert.Contains(t, body, "no page is called blog")
		c.d.Owner.GetJSON("/api/v1/help-centers/"+c.id+"/themes").AssertOK().AssertJSONPath("data", []any{})

		c.d.Owner.PostForm("/api/v1/help-centers/"+c.id+"/themes",
			testkit.FormFile{Field: "file", Name: "theme.zip", Contents: []byte("not a zip")}, nil).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "theme_archive_invalid")
	})

	t.Run("wears a published theme, serves its assets forever and goes back to the default", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		res := c.upload(map[string]string{
			"acme/theme.json": `{"api_version": 1, "name": "Acme", "version": "2.0.0",
				"settings": [{"id": "hero", "type": "text", "label": "Hero", "default": "Welcome"}]}`,
			"acme/templates/home.liquid": `<h1 class="acme-hero">{{ settings.hero }} to {{ help_center.name }}</h1>` +
				`{{ 'theme.css' | asset_url | stylesheet_tag }}{{ 'app.js' | asset_url | script_tag }}` +
				`<form method="post" action="{{ '/feedback' | url }}">{{ form_token_field }}</form>`,
			"acme/assets/theme.css": ".acme-hero{color:red}",
			"acme/assets/app.js":    "console.log(1)",
			"__MACOSX/acme/._x":     "junk",
		}).AssertCreated()
		res.AssertJSONPath("data.object", "help_center_theme").AssertJSONPath("data.version", float64(1)).
			AssertJSONPath("data.live", false).AssertJSONPath("data.theme_version", "2.0.0").
			AssertJSONPath("data.values.hero", "Welcome")
		theme := res.String("data.id")
		assert.NotContains(t, string(c.page("").Body), "acme-hero")

		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id+"/live-theme", testkit.JSON{"theme_id": theme}).
			AssertOK().AssertJSONPath("data.theme_id", theme)
		c.d.Owner.GetJSON("/api/v1/help-centers/"+c.id+"/themes").AssertOK().AssertJSONPath("data.0.live", true)
		c.d.Owner.PatchJSON(
			"/api/v1/help-centers/"+c.id+"/themes/"+theme,
			testkit.JSON{"settings": testkit.JSON{"hero": "Hello"}},
		).
			AssertOK().AssertJSONPath("data.values.hero", "Hello")
		c.d.Owner.PatchJSON(
			"/api/v1/help-centers/"+c.id+"/themes/"+theme, testkit.JSON{"settings": testkit.JSON{"nope": 1}},
		).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "theme_settings_invalid")

		home := string(c.page("").AssertOK().Body)
		assert.Contains(t, home, `<h1 class="acme-hero">Hello to`)
		assert.Contains(t, home, `name="_token"`)
		assert.Equal(
			t, c.hosted(), strings.Contains(home, "<script src="),
			"a theme's scripts run only on its own host",
		)
		assert.Contains(
			t, string(c.page("/search").AssertOK().Body), "max-w-[46rem]",
			"pages the theme lacks wear the default",
		)

		link := assetLink.FindStringSubmatch(home)
		require.NotNil(t, link, home)
		asset := c.d.TC.Client().Send("GET", c.assetAddress(link[1]), nil).AssertOK()
		assert.Equal(t, ".acme-hero{color:red}", string(asset.Body))
		assert.Equal(t, "public, max-age=31536000, immutable", asset.Header.Get("Cache-Control"))
		assert.Contains(t, asset.Header.Get("Content-Type"), "text/css")
		assert.Contains(t, asset.Header.Get("Content-Security-Policy"), "sandbox")
		stale := strings.Replace(link[1], "/theme.css", "x/theme.css", 1)
		c.d.TC.Client().Send("GET", c.assetAddress(strings.Replace(stale, "/"+theme+"/", "/"+theme+"/0", 1)), nil).
			AssertStatus(fhttp.StatusNotFound)
		script := strings.Replace(link[1], "theme.css", "app.js", 1)
		if !c.hosted() {
			c.d.TC.Client().Send("GET", c.assetAddress(script), nil).AssertStatus(fhttp.StatusNotFound)
		}

		zipped := c.d.Owner.Send("GET", "/api/v1/help-centers/"+c.id+"/themes/"+theme+"/archive", nil).AssertOK()
		assert.Equal(t, "application/zip", zipped.Header.Get("Content-Type"))
		assert.Contains(t, names(t, zipped.Body), "templates/home.liquid")
		starter := c.d.Owner.Send("GET", "/api/v1/help-centers/"+c.id+"/themes/default/archive", nil).AssertOK()
		assert.Contains(t, names(t, starter.Body), "layouts/theme.liquid")

		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id+"/live-theme", testkit.JSON{"theme_id": nil}).
			AssertOK().AssertJSONPath("data.theme_id", nil)
		assert.NotContains(t, string(c.page("").AssertOK().Body), "acme-hero")
	})

	t.Run("refuses a form without its page's token or from another site", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		form := url.Values{"email": {"ada@example.com"}}.Encode()
		c.d.TC.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
			Send("POST", c.address("/changelog/subscriptions"), form).AssertStatus(fhttp.StatusForbidden)
		c.forms(c.d.TC.Client()).WithHeader("Sec-Fetch-Site", "same-site").
			WithHeader("Content-Type", "application/x-www-form-urlencoded").
			Send("POST", c.address("/changelog/subscriptions"), form).AssertStatus(fhttp.StatusForbidden)
		c.forms(c.d.TC.Client()).WithHeader("Content-Type", "application/x-www-form-urlencoded").
			WithHeader("Accept", "text/html").
			Send("POST", c.address("/changelog/subscriptions"), form).AssertStatus(fhttp.StatusSeeOther)
	})
}

/*
assetAddress is where a browser fetches an asset the page links.
*/
func (c center) assetAddress(link string) string {
	if strings.HasPrefix(link, "/hc/") {
		return link
	}
	return c.address(link)
}

func names(t *testing.T, archive []byte) []string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	var out []string
	for _, f := range r.File {
		rc, err := f.Open()
		require.NoError(t, err)
		_, _ = io.CopyN(io.Discard, rc, 1<<20)
		_ = rc.Close()
		out = append(out, f.Name)
	}
	return out
}
