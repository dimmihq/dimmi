/*
|--------------------------------------------------------------------------
| Text Search Documents
|--------------------------------------------------------------------------
|
| Full text search runs in PostgreSQL, on tsvector columns the database
| generates from the row itself. The application never writes one, but
| any query that returns whole rows reads it, so the type has to scan.
|
*/

package database

import "github.com/jackc/pgx/v5/pgtype"

/*
SearchDocument is a tsvector value: the lexemes a row is found by. It
scans from a tsvector column.
*/
type SearchDocument = pgtype.TSVector
