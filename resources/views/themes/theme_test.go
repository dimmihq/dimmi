package themes

import (
	"strings"
	"testing"
)

func defaultTheme(t *testing.T) *Theme {
	t.Helper()
	d, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDefaultThemeHasEveryPage(t *testing.T) {
	d := defaultTheme(t)
	for _, page := range Pages() {
		if !d.HasTemplate(TemplateDir + page) {
			t.Errorf("the default theme has no %s template", page)
		}
	}
	if d.Custom() {
		t.Error("the default theme is not a team's own")
	}
}

func TestLoadListsEveryProblem(t *testing.T) {
	_, problems := Load(map[string][]byte{
		"theme.json": []byte(`{"api_version": 2, "name": "",
			"settings": [{"id": "Bad", "type": "text"}]}`),
		"templates/home.liquid":   []byte("{% if x %}"),
		"templates/nope.liquid":   []byte("hi"),
		"assets/../../etc/passwd": []byte("x"),
		"assets/run.exe":          []byte("x"),
		"partials/.hidden.liquid": []byte("x"),
	}, defaultTheme(t))
	want := []string{
		"api_version must be 1", "name must be", `setting id "Bad"`, "templates/home.liquid:1: missing {% endif %}",
		"no page is called nope", "not a plain path", "assets may be", "hidden files",
	}
	var all strings.Builder
	for _, p := range problems {
		all.WriteString(p.String() + "\n")
	}
	for _, w := range want {
		if !strings.Contains(all.String(), w) {
			t.Errorf("problems lack %q:\n%s", w, all.String())
		}
	}
}

func TestSmallThemeFallsBackToDefault(t *testing.T) {
	th, problems := Load(map[string][]byte{
		"theme.json": []byte(`{"api_version": 1, "name": "Mine",
			"settings": [{"id": "accent", "type": "color", "default": "#112233"}]}`),
		"assets/theme.css":     []byte("body{}"),
		"partials/logo.liquid": []byte("MY LOGO"),
	}, defaultTheme(t))
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	if th.HasTemplate(TemplateDir+"home") || !th.Custom() {
		t.Fatal("expected the default home")
	}
	a, owner, ok := th.Asset("theme.css")
	if !ok || owner != th || a.Hash == "" || !strings.HasPrefix(a.Type, "text/css") {
		t.Fatalf("asset %+v", a)
	}
	if v := th.Manifest.SettingValues(map[string]any{"accent": "nope"}); v["accent"] != "#112233" {
		t.Fatalf("settings %v", v)
	}
}
