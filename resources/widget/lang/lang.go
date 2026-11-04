/*
|--------------------------------------------------------------------------
| Messenger Languages
|--------------------------------------------------------------------------
|
| The messenger's built-in words, one file per language, shipped inside the
| binary so a visitor's language never needs a request of its own.
|
*/

package lang

import (
	"embed"
	"io/fs"
)

//go:embed *.json
var files embed.FS

/*
Files returns the language files.
*/
func Files() fs.FS {
	return files
}
