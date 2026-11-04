/*
|--------------------------------------------------------------------------
| Translations
|--------------------------------------------------------------------------
|
| The product's messages, one directory per locale and one file per
| group, loaded over the framework's English messages. English is the
| source; adding a language is adding a directory and listing its
| locale in app.locales, never a code change. A directory of messages
| that is not listed, such as the pseudo locale generated for testing
| layouts, is carried but never served.
|
*/

package lang

import (
	"embed"
	"io/fs"
)

//go:embed */*.json
var files embed.FS

/*
Files returns the translation files as <locale>/<group>.json.
*/
func Files() fs.FS {
	return files
}
