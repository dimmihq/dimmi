/*
|--------------------------------------------------------------------------
| Public Files
|--------------------------------------------------------------------------
|
| The browser app's production build ships inside the binary, so a
| release is one file. A binary built before the frontend holds an empty
| build: it starts and serves the API, and only its pages fail.
|
*/

package public

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var files embed.FS

/*
Build returns the production build of the browser app, its manifest at
.vite/manifest.json.
*/
func Build() fs.FS {
	build, _ := fs.Sub(files, "build")

	return build
}
