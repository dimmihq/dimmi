/*
|--------------------------------------------------------------------------
| Vite Assets
|--------------------------------------------------------------------------
|
| A Vite build names every file after its content, so the names change
| with each release and only the build's manifest knows them. Pages ask
| for an entry by its source path and get the tags for its stylesheets,
| the modules it imports and its own script, each carrying the request's
| content security policy nonce so a strict policy still lets them load.
|
*/

package vite

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"strings"

	"github.com/a-h/templ"
	vitejs "github.com/olivere/vite"

	"github.com/dimmihq/dimmi/framework/view"
)

/*
Errors the assets return.

ErrNoManifest means the build holds neither .vite/manifest.json, where
Vite 5 and later write it, nor manifest.json, where older versions did.
ErrUnknownEntry means a page asked for an entry the build does not have.
*/
var (
	ErrNoManifest   = errors.New("vite: build has no manifest")
	ErrUnknownEntry = errors.New("vite: unknown entry")
)

/*
Assets is the manifest of one production build and the URL path its
files are served under.
*/
type Assets struct {
	manifest vitejs.Manifest
	base     string
}

/*
Manifest reads the manifest of the build in the file system. Base is the
URL path the build's files are served under, such as "/build". The file
is only read, so an error closing it changes nothing.
*/
func Manifest(fsys fs.FS, base string) (*Assets, error) {
	file, err := openManifest(fsys)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	manifest, err := vitejs.ParseManifest(file)
	if err != nil {
		return nil, fmt.Errorf("parse vite manifest: %w", err)
	}

	return &Assets{manifest: *manifest, base: strings.TrimRight(base, "/")}, nil
}

/*
Tags returns the component writing the tags an entry needs, such as
"src/main.ts": its stylesheets and those of its imports, a preload link
for each module it imports and its own module script. Rendering it fails
with ErrUnknownEntry when the build has no such entry.
*/
func (a *Assets) Tags(entry string) view.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, ok := a.manifest.GetChunk(entry); !ok {
			return fmt.Errorf("%w: %q", ErrUnknownEntry, entry)
		}

		tags := a.manifest.GenerateCSS(entry, a.base) +
			a.manifest.GeneratePreloadModules(entry, a.base) +
			a.manifest.GenerateModules(entry, a.base)

		if _, err := io.WriteString(w, withNonce(tags, view.Nonce(ctx))); err != nil {
			return fmt.Errorf("write vite tags: %w", err)
		}

		return nil
	})
}

func openManifest(fsys fs.FS) (fs.File, error) {
	for _, name := range []string{".vite/manifest.json", "manifest.json"} {
		file, err := fsys.Open(name)
		if err == nil {
			return file, nil
		}

		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("open vite manifest %s: %w", name, err)
		}
	}

	return nil, ErrNoManifest
}

/*
withNonce adds the nonce to every script and link tag the library wrote,
since its tag builders take no attributes. Without a nonce the tags are
left as they are.
*/
func withNonce(tags, nonce string) string {
	if nonce == "" {
		return tags
	}

	attr := `nonce="` + html.EscapeString(nonce) + `" `

	return strings.NewReplacer("<script ", "<script "+attr, "<link ", "<link "+attr).Replace(tags)
}
