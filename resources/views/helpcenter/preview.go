/*
|--------------------------------------------------------------------------
| Theme Previews
|--------------------------------------------------------------------------
|
| A theme is previewed on its author's own computer with the help
| center's real pages: the server builds a page as it would for a
| reader, but hands back the template's name and variables instead of
| rendering them, and the preview renders them with the theme being
| written. The words, addresses and dates then come from the same code
| on both sides.
|
*/

package helpcenter

import (
	"context"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/resources/views/themes"
)

/*
Capture is a page caught instead of rendered: its template and the
variables it would have read.
*/
type Capture struct {
	Name    string
	Globals map[string]any
}

type captureKey struct{}

/*
WithCapture returns a context in which pages are caught in the capture
rather than rendered.
*/
func WithCapture(ctx context.Context) (context.Context, *Capture) {
	c := &Capture{}
	return context.WithValue(ctx, captureKey{}, c), c
}

/*
PreviewEnv is the environment a preview renders a caught page in: the
help center's words in the page's language, from lang, and its links
under base.
*/
func PreviewEnv(ctx context.Context, lang *i18n.Translator, locale, base string) themes.Env {
	c := Center{Lang: lang, Base: base}
	if locale != "en" {
		c.Locale = locale
	}
	return c.env(ctx)
}
