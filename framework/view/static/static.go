/*
|--------------------------------------------------------------------------
| Static Files
|--------------------------------------------------------------------------
|
| Built assets ship inside the binary. Compressing them once at build time
| beats compressing every response, so a file.js.gz next to file.js is
| sent to every client that accepts gzip, and unpacked on the fly for the
| rare one that does not. Every response carries an ETag, so a browser
| revalidating a file it already has gets a 304.
|
| Files whose names change with their content can be cached by browsers
| and proxies forever; Immutable says so for every file served.
|
*/

package static

import (
	"io/fs"
	"net/http"

	"github.com/vearutop/statigz"
)

/*
Options configure a handler.

Immutable marks every file as never changing, so clients cache it for a
year without revalidating; use it only for files named after their
content. Precompressed serves a .gz sibling of a file in its place to
clients that accept gzip.
*/
type Options struct {
	Immutable     bool
	Precompressed bool
}

/*
Handler serves the files in the file system at the request path, with
index.html for a directory. Only GET and HEAD are answered.
*/
func Handler(fsys fs.FS, opts Options) http.Handler {
	server := statigz.FileServer(readDirFS{fsys}, encodings(opts))
	if !opts.Immutable {
		return server
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if server.Found(r) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		server.ServeHTTP(w, r)
	})
}

func encodings(opts Options) func(*statigz.Server) {
	return func(s *statigz.Server) {
		if opts.Precompressed {
			return
		}

		s.Encodings = nil
	}
}

/*
readDirFS lets the library list any file system, since it asks for one
that can read directories and fs.ReadDir works on every file system.
*/
type readDirFS struct {
	fs.FS
}

/*
ReadDir lists the directory through fs.ReadDir.
*/
func (f readDirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.FS, name)
}
