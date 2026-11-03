/*
|--------------------------------------------------------------------------
| Public Files
|--------------------------------------------------------------------------
|
| The browser app's production build and the messenger widget ship inside
| the binary, so a release is one file. A binary built before the
| frontend holds empty builds: it starts and serves the API, and only its
| pages and the widget's files fail.
|
*/

package public

import (
	"embed"
	"io/fs"
)

//go:embed all:build all:widget
var files embed.FS

/*
Build returns the production build of the browser app, its manifest at
.vite/manifest.json.
*/
func Build() fs.FS {
	build, _ := fs.Sub(files, "build")

	return build
}

/*
Widget returns the messenger widget's build: the loader, its manifest at
manifest.json, and the hashed files under assets.
*/
func Widget() fs.FS {
	widget, _ := fs.Sub(files, "widget")

	return widget
}
