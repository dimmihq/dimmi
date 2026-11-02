/*
|--------------------------------------------------------------------------
| Storage Migration
|--------------------------------------------------------------------------
|
| The certificate table belongs to the framework's own migration set, so
| it exists before the first server tries to obtain a certificate, and
| an application upgrade never has to know it is there.
|
*/

package storage

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var migrations embed.FS

/*
Migrations returns the goose SQL migrations that create the table, at
the root of the filesystem, for the framework migration set.
*/
func Migrations() fs.FS {
	return migrations
}
