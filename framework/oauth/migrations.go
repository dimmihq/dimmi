/*
|--------------------------------------------------------------------------
| OAuth Migration
|--------------------------------------------------------------------------
|
| The client, code, grant and token tables ship with the framework so the
| authorization server works on a fresh database. The foundation adds
| these files to the framework migration set, which runs before the
| application's own migrations.
|
*/

package oauth

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var migrations embed.FS

/*
Migrations returns the goose SQL migrations that create the OAuth
tables, at the root of the filesystem, for the framework migration set.
*/
func Migrations() fs.FS {
	return migrations
}
