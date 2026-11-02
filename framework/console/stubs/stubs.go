/*
|--------------------------------------------------------------------------
| Default Stubs
|--------------------------------------------------------------------------
|
| Each make command renders one of these templates. They produce files
| that already follow the house style, banner and docblocks included,
| and pass the linter as generated, so a new file starts clean and only
| its placeholders need writing. An application replaces any stub by
| putting a file of the same name in its own stubs directory.
|
*/

package stubs

import (
	"embed"
	"io/fs"
)

//go:embed *.tmpl
var templates embed.FS

/*
FS returns the stubs, each at the root under its file name, such as
"job.go.tmpl".
*/
func FS() fs.FS {
	return templates
}
