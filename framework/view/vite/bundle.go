/*
|--------------------------------------------------------------------------
| Vite Bundle
|--------------------------------------------------------------------------
|
| A page names its entry and gets the tags that load it, whether a
| developer runs the Vite dev server or the build ships inside the
| binary. While the dev server runs, it writes its address to a hot file;
| as long as that file exists, pages load modules straight from the dev
| server, which pushes hot updates to the browser. Without it, pages load
| the production build, whose manifest is read once, on first use, so a
| binary built without the frontend still starts and only its pages fail.
|
| Only development configures a hot file. A production server never
| looks for one, so no file on its disk can redirect its scripts.
|
*/

package vite

import (
	"context"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/a-h/templ"
	vitejs "github.com/olivere/vite"

	"github.com/dimmihq/dimmi/framework/view"
)

/*
Options configure a bundle.

Build holds the production build, its manifest at .vite/manifest.json,
and Base is the URL path its files are served under, such as "/build".
HotFile, set only in development, names the file the dev server writes
its address to. React loads the React Fast Refresh runtime before the
entry in development, which @vitejs/plugin-react needs.
*/
type Options struct {
	Build   fs.FS
	Base    string
	HotFile string
	React   bool
}

/*
Bundle loads entries from the dev server or the production build.
*/
type Bundle struct {
	opts   Options
	once   sync.Once
	assets *Assets
	err    error
}

/*
New returns the bundle. Nothing is read until a page asks for tags.
*/
func New(opts Options) *Bundle {
	return &Bundle{opts: opts}
}

/*
DevServer returns the address of the running dev server, and false when
none runs or the bundle has no hot file.
*/
func (b *Bundle) DevServer() (string, bool) {
	if b.opts.HotFile == "" {
		return "", false
	}

	address, err := os.ReadFile(b.opts.HotFile)
	if err != nil {
		return "", false
	}

	trimmed := strings.TrimRight(strings.TrimSpace(string(address)), "/")

	return trimmed, trimmed != ""
}

/*
Tags returns the component writing the tags the entries need, such as
"resources/js/app.tsx", in order, from the dev server while it runs, else
from the production build. A stylesheet entry, such as
"resources/js/site.css", loads as a link in both, so the page is styled
from its first paint instead of once a script injects the styles.
Rendering fails with ErrNoManifest when there is no build, and with
ErrUnknownEntry when the build lacks an entry.
*/
func (b *Bundle) Tags(entries ...string) view.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if server, ok := b.DevServer(); ok {
			return b.devTags(ctx, w, server, entries)
		}

		assets, err := b.build()
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if err := assets.Tags(entry).Render(ctx, w); err != nil {
				return err
			}
		}

		return nil
	})
}

/*
build reads the production manifest once.
*/
func (b *Bundle) build() (*Assets, error) {
	b.once.Do(func() { b.assets, b.err = Manifest(b.opts.Build, b.opts.Base) })

	return b.assets, b.err
}

/*
devTags writes the tags that load the entries from the dev server: the
Fast Refresh runtime when asked for, the client that receives hot
updates, then each entry, a stylesheet as a link the dev server keeps
updating and a script as a module.
*/
func (b *Bundle) devTags(ctx context.Context, w io.Writer, server string, entries []string) error {
	var tags strings.Builder

	if b.opts.React {
		tags.WriteString(vitejs.PluginReactPreamble(server))
	}

	tags.WriteString(`<script type="module" src="` + html.EscapeString(server+"/@vite/client") + `"></script>`)
	for _, entry := range entries {
		src := html.EscapeString(server + "/" + strings.TrimLeft(entry, "/"))
		if strings.HasSuffix(entry, ".css") {
			tags.WriteString(`<link rel="stylesheet" href="` + src + `">`)
			continue
		}
		tags.WriteString(`<script type="module" src="` + src + `"></script>`)
	}

	if _, err := io.WriteString(w, withNonce(tags.String(), view.Nonce(ctx))); err != nil {
		return fmt.Errorf("write vite dev tags: %w", err)
	}

	return nil
}
