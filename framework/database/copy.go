/*
|--------------------------------------------------------------------------
| Bulk Copy
|--------------------------------------------------------------------------
|
| Imports move thousands of rows at a time. COPY loads them in one round
| trip instead of one insert each, and it runs inside the transaction so
| a failed batch leaves nothing behind.
|
| PostgreSQL refuses COPY into a table with row-level security enabled
| for the current role, so tenant tables are filled with inserts instead.
|
*/

package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

/*
CopyFrom loads the rows into the table's columns with COPY and returns
how many rows it wrote. The table may be qualified with its schema, as
in "archive.events". Each row lists its values in the order of columns.
*/
func (tx *Tx) CopyFrom(ctx context.Context, table string, columns []string, rows [][]any) (int64, error) {
	n, err := tx.tx.CopyFrom(ctx, pgx.Identifier(strings.Split(table, ".")), columns, pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("copy into %s: %w", table, tx.db.mapError(err))
	}

	return n, nil
}
