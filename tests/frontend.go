/*
|--------------------------------------------------------------------------
| Test Frontend Build
|--------------------------------------------------------------------------
|
| Pages link the browser app's files through the Vite manifest, and a page
| without one fails. Tests run without building the browser app, so every
| test case starts with a small build that names each entry the server
| renders. A test about the build itself replaces it with Before.
|
*/

package tests

import (
	"io/fs"
	"testing/fstest"
)

const frontendManifest = `{
  "resources/js/app.tsx": {
    "file": "assets/app-test.js",
    "src": "resources/js/app.tsx",
    "isEntry": true,
    "css": ["assets/app-test.css"]
  },
  "resources/js/portal/portal.ts": {
    "file": "assets/portal-test.js",
    "src": "resources/js/portal/portal.ts",
    "isEntry": true
  },
  "resources/js/portal/portal.css": {
    "file": "assets/portal-test.css",
    "src": "resources/js/portal/portal.css",
    "isEntry": true
  }
}`

/*
Frontend returns the build every test case starts with: a manifest for
each entry the server renders and the files it names.
*/
func Frontend() fs.FS {
	return fstest.MapFS{
		".vite/manifest.json":    &fstest.MapFile{Data: []byte(frontendManifest)},
		"assets/app-test.js":     &fstest.MapFile{Data: []byte("console.log('app')")},
		"assets/app-test.css":    &fstest.MapFile{Data: []byte("body{}")},
		"assets/portal-test.js":  &fstest.MapFile{Data: []byte("console.log('portal')")},
		"assets/portal-test.css": &fstest.MapFile{Data: []byte("body{}")},
	}
}
