/*
|--------------------------------------------------------------------------
| Themed Pages
|--------------------------------------------------------------------------
|
| Every page of the portal renders through the help center's theme: its
| team's own once they published one, else the default. A page hands the
| theme its variables; the frame adds what every page reads, such as
| help_center, locale and page, and the tags only the platform writes,
| because they carry the page's nonce or speak for search engines.
|
| A theme's own scripts run only where Scripts allows, on the help
| center's own domain: on the app's /hc/ path the theme gets no nonce
| and its script tags write nothing.
|
*/

package helpcenter

import (
	"context"
	"errors"
	"html"
	"io"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/liquid"
	"github.com/dimmihq/dimmi/framework/view"

	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
Theming is how a help center's pages wear its theme: the theme, its
team's or the default, the team's values for its settings, whether the
theme's own scripts may run on the host serving the page, where its
assets are served, the anti-forgery token every form the page posts
carries, and the platform's own stylesheet and script.
*/
type Theming struct {
	Theme     *themes.Theme
	Settings  map[string]any
	Scripts   bool
	AssetURL  func(t *themes.Theme, a themes.Asset) string
	FormToken string
	Assets    view.Component
}

/*
FormTokenField is the name of the form field holding the anti-forgery
token.
*/
const FormTokenField = "_token"

/*
Render renders the named page of the help center's theme with the
page's own variables, inside the frame every page shares.
*/
func Render(c Center, name string, m Meta, vars map[string]any) view.Component {
	return view.Func(func(ctx context.Context, w io.Writer) error {
		theme := c.Theming.Theme
		if theme == nil {
			return errors.New("help center page " + name + ": no theme to render in")
		}
		globals := c.globals(ctx, name, m)
		for k, v := range vars {
			globals[k] = liquid.Value(v)
		}
		return theme.Render(ctx, w, themes.Page{Name: name, Globals: globals, Env: c.env(ctx)})
	})
}

func (c Center) env(ctx context.Context) themes.Env {
	env := themes.Env{
		Base: c.Base, Scripts: c.Theming.Scripts, AssetURL: c.Theming.AssetURL,
		T: func(key string, args map[string]any) string {
			pairs := make([]any, 0, 2*len(args))
			for k, v := range args {
				pairs = append(pairs, k, v)
			}
			return c.T(key, pairs...)
		},
		Date: func(t time.Time, style string) string {
			if style == "short" {
				return c.ShortDate(t)
			}
			return c.Date(t)
		},
	}
	if c.Theming.Scripts {
		env.Nonce = view.Nonce(ctx)
	}
	return env
}

func (c Center) globals(ctx context.Context, name string, m Meta) map[string]any {
	nonce := view.Nonce(ctx)
	platform := strings.Builder{}
	platform.WriteString(`<meta name="csp-nonce" content="` + html.EscapeString(nonce) + `">`)
	if c.Theming.FormToken != "" {
		platform.WriteString(`<meta name="form-token" content="` + html.EscapeString(c.Theming.FormToken) + `">`)
	}
	platform.WriteString(applyTheme(nonce))
	if c.Theming.Assets != nil {
		_ = c.Theming.Assets.Render(ctx, &platform)
	}
	token := ""
	if c.Theming.FormToken != "" {
		token = `<input type="hidden" name="` + FormTokenField + `" value="` +
			html.EscapeString(c.Theming.FormToken) + `">`
	}
	scriptNonce := ""
	if c.Theming.Scripts {
		scriptNonce = nonce
	}
	page := c.pageVars(name, m)
	return map[string]any{
		"help_center":       liquid.Value(c.helpCenterVars()),
		"reader":            liquid.Value(c.readerVars()),
		"locale":            liquid.Value(c.localeVars()),
		"page":              liquid.Value(page),
		"translations":      c.translations(),
		"settings":          liquid.Value(c.Theming.Theme.Manifest.SettingValues(c.Theming.Settings)),
		"platform_assets":   liquid.HTML(platform.String()),
		"seo_tags":          liquid.HTML(c.seoTags(page)),
		"messenger_snippet": liquid.HTML(c.messengerSnippet(nonce)),
		"form_token":        c.Theming.FormToken,
		"form_token_field":  liquid.HTML(token),
		"script_nonce":      scriptNonce,
	}
}

/*
applyTheme sets the page's theme before it paints: the team's fixed
one, or the reader's pick kept in this browser, or their system's,
which it then keeps following.
*/
func applyTheme(nonce string) string {
	return `<script nonce="` + html.EscapeString(nonce) + `">` +
		`(() => { const root = document.documentElement; ` +
		`const dark = matchMedia("(prefers-color-scheme: dark)"); ` +
		`const pick = () => { const fixed = root.dataset.appearance; if (fixed) return fixed; ` +
		`try { return localStorage.getItem("dimmi-portal-theme") || "system" } catch { return "system" } }; ` +
		`const apply = () => { const p = pick(); root.dataset.themePick = p; ` +
		`root.classList.toggle("dark", p === "dark" || (p === "system" && dark.matches)) }; ` +
		`apply(); dark.addEventListener("change", apply); window.dimmiTheme = apply })()</script>`
}

/*
seoTags are what search engines and link previews read: the page's
description, its title and site for previews, whether to skip it, its
canonical address, its other languages, its Markdown twin and the
changelog's feed.
*/
func (c Center) seoTags(p PageVars) string {
	var b strings.Builder
	tag := func(format string, values ...string) {
		for i, v := range values {
			format = strings.Replace(format, "%"+string(rune('a'+i)), html.EscapeString(v), 1)
		}
		b.WriteString(format)
	}
	if p.Description != "" {
		tag(`<meta name="description" content="%a"><meta property="og:description" content="%b">`,
			p.Description, p.Description)
	}
	tag(`<meta property="og:title" content="%a"><meta property="og:site_name" content="%b">`, p.Title, c.Name)
	if p.NoIndex {
		b.WriteString(`<meta name="robots" content="noindex">`)
	}
	if c.Canonical != "" && !p.NoIndex {
		tag(`<link rel="canonical" href="%a">`, c.Canonical)
	}
	for _, a := range c.Alternates {
		tag(`<link rel="alternate" hreflang="%a" href="%b">`, a.Lang, a.URL)
	}
	if p.MarkdownURL != "" {
		tag(`<link rel="alternate" type="text/markdown" href="%a">`, p.MarkdownURL)
	}
	tag(`<link rel="alternate" type="application/rss+xml" title="%a" href="%b">`,
		c.Product+" changelog", c.url("/changelog/feed.xml"))
	return b.String()
}

/*
messengerSnippet loads the help center's messenger, nothing when it
shows none.
*/
func (c Center) messengerSnippet(nonce string) string {
	if c.Messenger == "" {
		return ""
	}
	return `<script async nonce="` + html.EscapeString(nonce) + `" src="` + html.EscapeString(c.MessengerSrc) +
		`" data-app-id="` + html.EscapeString(c.Messenger) +
		`" data-identity-url="` + html.EscapeString(c.MessengerIdentity) + `"></script>`
}

/*
translations is every word of the help center in the page's language,
for scripts and for words a theme prints as they are.
*/
func (c Center) translations() map[string]any {
	if c.Lang == nil {
		return map[string]any{}
	}
	keys := c.Lang.Keys("helpcenter.")
	out := make(map[string]any, len(keys))
	for _, k := range keys {
		word := strings.TrimPrefix(k, "helpcenter.")
		out[word] = c.T(word)
	}
	return out
}
