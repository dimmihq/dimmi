package themepreview_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/liquid"
	"github.com/dimmihq/dimmi/framework/view"

	"github.com/dimmihq/dimmi/cli/dimmi"
	"github.com/dimmihq/dimmi/cli/themepreview"
	"github.com/dimmihq/dimmi/lang"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
pages answers every path with a help center's home, caught and sent as
JSON the way dimmi sends it.
*/
type pages struct{ preview dimmi.Preview }

func (p pages) ThemePreview(context.Context, string, string) (dimmi.Preview, error) {
	return p.preview, nil
}

func caughtHome(t *testing.T, tr *i18n.Translator, theme *themes.Theme) dimmi.Preview {
	t.Helper()
	c := helpcenter.Center{Name: "Acme Help", Product: "Acme", Brand: "#5b5bd6", Base: "/hc/acme", Lang: tr}
	c.Theming.Theme = theme
	all := []helpcenter.Collection{{
		Slug: "start", Title: "Getting started", Count: 1,
		Articles: []helpcenter.Summary{{Slug: "install", Title: "Install the widget"}},
	}}
	ctx, capture := helpcenter.WithCapture(context.Background())
	if _, err := view.HTML(ctx, helpcenter.Home(c, all, all[0].Articles, nil)); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(liquid.Encode(capture.Globals))
	if err != nil {
		t.Fatal(err)
	}
	var vars map[string]any
	if err := json.Unmarshal(raw, &vars); err != nil {
		t.Fatal(err)
	}
	return dimmi.Preview{Status: http.StatusOK, Template: capture.Name, Variables: vars}
}

func TestPreviewRendersLivePagesInTheFolderTheme(t *testing.T) {
	tr, err := i18n.New(i18n.Config{Locale: i18n.Source}, lang.Files())
	if err != nil {
		t.Fatal(err)
	}
	def, err := themes.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("theme.json", `{"api_version": 1, "name": "Preview"}`)
	write("partials/logo.liquid",
		`<b class="my-logo">{{ help_center.product }}</b>{{ 'x.css' | asset_url | stylesheet_tag }}`)
	write("assets/x.css", "b{color:red}")
	folder := &themepreview.Folder{Dir: dir, Fallback: def}
	if _, err := folder.Load(); err != nil {
		t.Fatal(err)
	}
	srv := &themepreview.Server{
		Folder: folder, Pages: pages{caughtHome(t, tr, def)}, Lang: tr,
		HelpCenter: dimmi.HelpCenter{ID: "hc_1", Subdomain: "acme"},
	}

	res := httptest.NewRecorder()
	srv.ServeHTTP(res, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hc/acme", http.NoBody))
	page := res.Body.String()
	logo := strings.Contains(page, `<b class="my-logo">Acme</b>`)
	if res.Code != http.StatusOK || !logo || !strings.Contains(page, "__dimmi/reload") {
		t.Fatalf("status %d, page:\n%s", res.Code, page)
	}
	_, after, _ := strings.Cut(page, `<link rel="stylesheet" href="`)
	css, _, _ := strings.Cut(after, `"`)
	asset := httptest.NewRecorder()
	srv.ServeHTTP(asset, httptest.NewRequestWithContext(t.Context(), http.MethodGet, css, http.NoBody))
	if asset.Body.String() != "b{color:red}" {
		t.Fatalf("asset %s: %q", css, asset.Body.String())
	}

	write("partials/logo.liquid", `{% if %}`)
	if changed, err := folder.Load(); err != nil || !changed {
		t.Fatalf("reload: %v %v", changed, err)
	}
	broken := httptest.NewRecorder()
	srv.ServeHTTP(broken, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hc/acme", http.NoBody))
	if !strings.Contains(broken.Body.String(), "partials/logo.liquid:1") {
		t.Fatalf("problems page:\n%s", broken.Body.String())
	}
}
