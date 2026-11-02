/*
|--------------------------------------------------------------------------
| Application Migrations
|--------------------------------------------------------------------------
|
| The product's tables, as plain SQL run in file name order after the
| framework's own tables. Each file is one forward step; production never
| runs the down sections, which exist for local development only.
|
*/

package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var files embed.FS

/*
Files returns the migrations at the root of the filesystem, the shape
the migration runner reads.
*/
func Files() fs.FS {
	return files
}
