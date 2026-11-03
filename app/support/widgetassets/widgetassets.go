/*
|--------------------------------------------------------------------------
| Widget Assets
|--------------------------------------------------------------------------
|
| Customers paste one fixed address, the loader, and it must always lead
| to a core that exists. The build leaves a placeholder in the loader and
| a manifest naming the current core; at boot the loader is filled in
| with the core's hashed name, its integrity hash and the assets host, so
| a cached loader and the files it asks for always belong to the same
| release. The hashed files never change, and the files of the previous
| release stay in the build, so a page still running the old core can
| load its lazy parts after a deploy.
|
| The loader and manifest are compressed once at boot and revalidated by
| ETag; the hashed files are served with their brotli and gzip copies.
|
*/

package widgetassets

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/view/static"
)

/*
ErrNoBuild means the binary was built before the widget, so there is
nothing to serve. ErrBrokenBuild means the build is there but its
manifest or loader cannot be used.
*/
var (
	ErrNoBuild     = errors.New("widgetassets: no widget build")
	ErrBrokenBuild = errors.New("widgetassets: widget build is broken")
)

/*
Prefix is the URL path the hashed files are served under.
*/
const Prefix = "/widget/assets"

const placeholder = "__DIMMI_BUILD__"

/*
File is one hashed file of the build: its name, its size in bytes and
its sha384 integrity hash.
*/
type File struct {
	Name      string `json:"name"`
	Size      int    `json:"size"`
	Integrity string `json:"integrity"`
}

/*
manifest is the part of the build's manifest.json the server reads.
*/
type manifest struct {
	Entry string `json:"entry"`
	Files []File `json:"files"`
}

/*
Assets is a loaded widget build. Loader and Manifest are served from
memory; Files serves the hashed files under Prefix.
*/
type Assets struct {
	Loader   Asset
	Manifest Asset
	Files    fhttp.Handler
}

/*
New loads the build and fills in the loader. The assets base URL is the
origin serving the hashed files, empty for this install's own host,
which the loader then works out from its own address.
*/
func New(build fs.FS, assetsBaseURL string) (*Assets, error) {
	if build == nil {
		return nil, ErrNoBuild
	}

	raw, err := fs.ReadFile(build, "manifest.json")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoBuild
	}

	if err != nil {
		return nil, fmt.Errorf("read widget manifest: %w", err)
	}

	loader, err := fillLoader(build, raw, assetsBaseURL)
	if err != nil {
		return nil, err
	}

	files, err := fs.Sub(build, "assets")
	if err != nil {
		return nil, fmt.Errorf("open widget assets: %w", err)
	}

	return &Assets{
		Loader:   newAsset(loader),
		Manifest: newAsset(raw),
		Files:    static.Handler(files, static.Options{Immutable: true, Precompressed: true, Prefix: Prefix}),
	}, nil
}

/*
fillLoader replaces the loader's placeholder with the current core: its
file name, integrity hash and the host to fetch it from.
*/
func fillLoader(build fs.FS, raw []byte, assetsBaseURL string) ([]byte, error) {
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: manifest: %w", ErrBrokenBuild, err)
	}

	entry, ok := m.file(m.Entry)
	if !ok {
		return nil, fmt.Errorf("%w: manifest lists no core %q", ErrBrokenBuild, m.Entry)
	}

	loader, err := fs.ReadFile(build, "loader.js")
	if err != nil {
		return nil, fmt.Errorf("%w: loader: %w", ErrBrokenBuild, err)
	}

	if !bytes.Contains(loader, []byte(placeholder)) {
		return nil, fmt.Errorf("%w: loader has no %s placeholder", ErrBrokenBuild, placeholder)
	}

	current, err := json.Marshal(map[string]string{
		"core":      entry.Name,
		"integrity": entry.Integrity,
		"assets":    assetsHost(assetsBaseURL),
	})
	if err != nil {
		return nil, fmt.Errorf("encode widget build: %w", err)
	}

	return bytes.ReplaceAll(loader, []byte(placeholder), current), nil
}

func (m manifest) file(name string) (File, bool) {
	for _, f := range m.Files {
		if f.Name == name {
			return f, true
		}
	}

	return File{}, false
}

/*
assetsHost is where the loader fetches hashed files from: the configured
origin followed by Prefix, or empty for the loader's own host.
*/
func assetsHost(base string) string {
	if base == "" {
		return ""
	}

	return strings.TrimRight(base, "/") + Prefix + "/"
}

/*
Asset is a file served from memory: its bytes, a gzip copy and an ETag
taken from its content.
*/
type Asset struct {
	Body []byte
	gzip []byte
	etag string
}

func newAsset(body []byte) Asset {
	var compressed bytes.Buffer

	w, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	_, _ = w.Write(body)
	_ = w.Close()

	sum := hashing.SHA256(body)

	return Asset{Body: body, gzip: compressed.Bytes(), etag: `"` + hex.EncodeToString(sum[:8]) + `"`}
}

/*
Write answers a request for the asset: 304 when the client already holds
this version, the gzip copy when it accepts gzip, the plain bytes
otherwise. Headers set beforehand, such as the content type, are kept.
*/
func (a Asset) Write(w fhttp.ResponseWriter, r *fhttp.Request) {
	h := w.Header()
	h.Set("ETag", a.etag)
	h.Add("Vary", "Accept-Encoding")

	if r.Header.Get("If-None-Match") == a.etag {
		w.WriteHeader(fhttp.StatusNotModified)

		return
	}

	body := a.Body
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "gzip") {
		h.Set("Content-Encoding", "gzip")
		body = a.gzip
	}

	_, _ = w.Write(body)
}
