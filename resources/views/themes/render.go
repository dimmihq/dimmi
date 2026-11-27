/*
|--------------------------------------------------------------------------
| Rendering Pages
|--------------------------------------------------------------------------
|
| A page renders its template first, then the layout around it: the
| layout reads the page's output as content_for_layout and every
| variable the page assigned, so a page can name its own title or pick
| another layout with {% layout 'name' %}, or none. The help center
| hands the variables over; the environment says what the page may do
| on the host it is served on, such as whether a theme's own scripts
| run, and how it writes its words, addresses and dates.
|
*/

package themes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/dimmihq/dimmi/framework/liquid"
)

/*
Env is what a render needs from the request: T writes the help center's
words in the page's language, Base is the path every link starts with,
AssetURL is the address of a theme's asset, Scripts says a theme's own
scripts may run on this host and Nonce is the one they carry, Date
writes a date as the page's language does, in the long or short style,
and Limits bound the render, liquid's defaults when zero.
*/
type Env struct {
	T        func(key string, args map[string]any) string
	Base     string
	AssetURL func(t *Theme, a Asset) string
	Scripts  bool
	Nonce    string
	Date     func(t time.Time, style string) string
	Limits   liquid.Limits
}

/*
Page is one page to render: the name of its template, the variables
every template and partial reads, and the request's environment.
*/
type Page struct {
	Name    string
	Globals map[string]any
	Env     Env
}

/*
renderData is what the theme's filters read during a render.
*/
type renderData struct {
	theme *Theme
	env   Env
}

/*
Render writes the page in the theme: its template, then its layout.
*/
func (t *Theme) Render(ctx context.Context, w io.Writer, p Page) error {
	opts := liquid.Options{Globals: p.Globals, Data: &renderData{theme: t, env: p.Env}, Limits: p.Env.Limits}
	var content bytes.Buffer
	res, err := t.set.Render(ctx, &content, TemplateDir+p.Name, opts)
	if err != nil {
		return fmt.Errorf("theme page %s: %w", p.Name, err)
	}
	layout := DefaultLayout
	if res.LayoutSet {
		layout = res.Layout
	}
	if layout == "" {
		_, err = content.WriteTo(w)
		return err
	}
	frame, ok := t.set.Lookup(LayoutDir + layout)
	if !ok {
		return fmt.Errorf("theme page %s: no layout named %q", p.Name, layout)
	}
	res.Assigned["content_for_layout"] = liquid.HTML(content.String())
	opts.Locals = res.Assigned
	if _, err = t.set.RenderTemplate(ctx, w, frame, opts); err != nil {
		return fmt.Errorf("theme layout %s: %w", layout, err)
	}
	return nil
}

/*
newEngine returns the Liquid engine themes are parsed with: Liquid's
filters and the help center's.
*/
func newEngine() *liquid.Engine {
	e := liquid.New()
	e.Filter("t", translate)
	e.Filter("url", helpURL)
	e.Filter("asset_url", assetURL)
	e.Filter("stylesheet_tag", stylesheetTag)
	e.Filter("script_tag", scriptTag)
	e.Filter("icon", iconFilter)
	e.Filter("format_date", formatDate)
	return e
}

/*
Filters returns the names of every filter a theme may use.
*/
func Filters() []string { return newEngine().Filters() }

func dataOf(c *liquid.Call) (*renderData, error) {
	d, ok := c.Data.(*renderData)
	if !ok {
		return nil, fmt.Errorf("the page renders outside a theme")
	}
	return d, nil
}
