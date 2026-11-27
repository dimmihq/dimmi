/*
|--------------------------------------------------------------------------
| Theme Filters
|--------------------------------------------------------------------------
|
| The filters a theme has beside Liquid's own: the help center's words
| in the page's language, addresses under the help center's base, the
| addresses and tags of the theme's assets, icons, and dates written as
| the page's language writes them.
|
*/

package themes

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/liquid"
)

/*
translate writes one of the help center's words in the page's language,
such as {{ 'article_count' | t: count: 3 }}. An argument that is HTML,
such as a captured time element, goes in as markup and the rest of the
words are escaped around it.
*/
func translate(c *liquid.Call) (any, error) {
	d, err := dataOf(c)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprint(c.Input)
	if d.env.T == nil {
		return "helpcenter." + key, nil
	}
	args := map[string]any{}
	marks := map[string]string{}
	for name, v := range c.Named {
		if h, ok := v.(liquid.HTML); ok {
			mark := fmt.Sprintf("\x00%d\x00", len(marks))
			marks[mark] = string(h)
			args[name] = mark
			continue
		}
		args[name] = plain(v)
	}
	out := d.env.T(key, args)
	if len(marks) == 0 {
		return out, nil
	}
	out = html.EscapeString(out)
	for mark, markup := range marks {
		out = strings.ReplaceAll(out, mark, markup)
	}
	return liquid.HTML(out), nil
}

func plain(v any) any {
	switch x := v.(type) {
	case nil:
		return ""
	case int, float64, string:
		return x
	}
	return fmt.Sprint(v)
}

/*
helpURL turns a path into the address of that page of the help center,
under its base: {{ '/search' | url }}.
*/
func helpURL(c *liquid.Call) (any, error) {
	d, err := dataOf(c)
	if err != nil {
		return nil, err
	}
	return Href(d.env.Base, fmt.Sprint(c.Input)), nil
}

/*
Href is the address of a path under a help center's base.
*/
func Href(base, path string) string {
	if path == "/" && base != "" {
		return base
	}
	return base + path
}

/*
assetURL is the address of one of the theme's assets, carrying its hash
so it can be cached forever: {{ 'theme.css' | asset_url }}.
*/
func assetURL(c *liquid.Call) (any, error) {
	d, err := dataOf(c)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprint(c.Input)
	a, owner, ok := d.theme.Asset(name)
	if !ok {
		return nil, fmt.Errorf("the theme has no asset %q", name)
	}
	if d.env.AssetURL == nil {
		return name, nil
	}
	return d.env.AssetURL(owner, a), nil
}

func stylesheetTag(c *liquid.Call) (any, error) {
	return liquid.HTML(`<link rel="stylesheet" href="` + html.EscapeString(fmt.Sprint(c.Input)) + `">`), nil
}

/*
scriptTag loads a script where a theme's scripts may run, and writes
nothing where they may not, such as on the app's /hc/ path.
*/
func scriptTag(c *liquid.Call) (any, error) {
	d, err := dataOf(c)
	if err != nil || !d.env.Scripts {
		return liquid.HTML(""), err
	}
	return liquid.HTML(`<script src="` + html.EscapeString(fmt.Sprint(c.Input)) + `" nonce="` +
		html.EscapeString(d.env.Nonce) + `" defer></script>`), nil
}

/*
iconFilter draws one of the help center's icons in the text's color:
{{ 'search' | icon: 'size-4' }}.
*/
func iconFilter(c *liquid.Call) (any, error) {
	class := ""
	if c.Arg(0) != nil {
		class = fmt.Sprint(c.Arg(0))
	}
	return liquid.HTML(Icon(fmt.Sprint(c.Input), class)), nil
}

/*
formatDate writes a time as the page's language writes dates: long,
such as October 28, 2026, short, without the year where the language
does, or iso, as 2026-10-28.
*/
func formatDate(c *liquid.Call) (any, error) {
	t, ok := c.Input.(time.Time)
	if !ok {
		return c.Input, nil
	}
	style := ""
	if c.Arg(0) != nil {
		style = fmt.Sprint(c.Arg(0))
	}
	if style == "iso" {
		return t.Format(time.DateOnly), nil
	}
	d, err := dataOf(c)
	if err != nil {
		return nil, err
	}
	if d.env.Date == nil {
		return t.Format("January 2, 2006"), nil
	}
	return d.env.Date(t, style), nil
}
