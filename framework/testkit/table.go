/*
|--------------------------------------------------------------------------
| Database Assertions
|--------------------------------------------------------------------------
|
| A test proves a change was stored by looking for the row, the same way
| it would look for anything else a user can see. A row is given as the
| columns that matter and their values; columns left out may hold
| anything. A nil value matches NULL.
|
| The checks run through any querier, so a test can look through the
| pool or inside a transaction where the tenant is set and row-level
| security shows only that tenant's rows.
|
*/

package testkit

import (
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
Row is a set of column values a row must hold.
*/
type Row = map[string]any

/*
TableAssertions checks the rows of one table.
*/
type TableAssertions struct {
	tb    testing.TB
	q     database.Querier
	table string
}

/*
Table returns the assertions for the table, which may be qualified by
its schema, as in "audit.events".
*/
func Table(tb testing.TB, q database.Querier, table string) *TableAssertions {
	tb.Helper()

	return &TableAssertions{tb: tb, q: q, table: table}
}

/*
AssertDatabaseHas checks the table holds at least one row with the
values.
*/
func AssertDatabaseHas(tb testing.TB, q database.Querier, table string, row Row) {
	tb.Helper()

	Table(tb, q, table).AssertHas(row)
}

/*
AssertDatabaseMissing checks the table holds no row with the values.
*/
func AssertDatabaseMissing(tb testing.TB, q database.Querier, table string, row Row) {
	tb.Helper()

	Table(tb, q, table).AssertMissing(row)
}

/*
AssertDatabaseCount checks the table holds exactly the number of rows.
*/
func AssertDatabaseCount(tb testing.TB, q database.Querier, table string, count int) {
	tb.Helper()

	Table(tb, q, table).AssertCount(count)
}

/*
AssertHas checks the table holds at least one row with the values. On
failure it shows a few of the rows the table does hold.
*/
func (t *TableAssertions) AssertHas(row Row) *TableAssertions {
	t.tb.Helper()

	if t.count(row) == 0 {
		t.tb.Errorf("no row in %s matches %s; found: %s", t.table, describe(row), t.sample())
	}

	return t
}

/*
AssertMissing checks the table holds no row with the values.
*/
func (t *TableAssertions) AssertMissing(row Row) *TableAssertions {
	t.tb.Helper()

	if found := t.count(row); found > 0 {
		t.tb.Errorf("%d rows in %s match %s", found, t.table, describe(row))
	}

	return t
}

/*
AssertCount checks the number of rows that hold all the values of every
row given, or of all rows when none is given.
*/
func (t *TableAssertions) AssertCount(count int, where ...Row) *TableAssertions {
	t.tb.Helper()

	merged := Row{}
	for _, row := range where {
		maps.Copy(merged, row)
	}

	if found := t.count(merged); found != count {
		t.tb.Errorf("%s has %d rows matching %s, want %d", t.table, found, describe(merged), count)
	}

	return t
}

func (t *TableAssertions) count(row Row) int {
	t.tb.Helper()

	where, args := conditions(row)

	var found int
	if err := t.q.QueryRow(t.tb.Context(), "SELECT count(*) FROM "+t.identifier()+where, args...).
		Scan(&found); err != nil {
		t.tb.Fatalf("count rows in %s: %v", t.table, err)
	}

	return found
}

/*
sample returns up to five rows of the table as JSON, to show what is
there when an expected row is not.
*/
func (t *TableAssertions) sample() string {
	t.tb.Helper()

	rows, err := t.q.Query(t.tb.Context(), "SELECT row_to_json(r)::text FROM "+t.identifier()+" r LIMIT 5")
	if err != nil {
		return err.Error()
	}

	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err.Error()
	}

	if len(found) == 0 {
		return "no rows"
	}

	return strings.Join(found, ", ")
}

func (t *TableAssertions) identifier() string {
	return pgx.Identifier(strings.Split(t.table, ".")).Sanitize()
}

/*
conditions returns the WHERE clause matching every value of the row and
its arguments. Columns are compared with IS NOT DISTINCT FROM, so a nil
value matches NULL.
*/
func conditions(row Row) (string, []any) {
	if len(row) == 0 {
		return "", nil
	}

	columns := slices.Sorted(maps.Keys(row))
	clauses := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))

	for i, column := range columns {
		clauses = append(clauses, pgx.Identifier{column}.Sanitize()+" IS NOT DISTINCT FROM $"+strconv.Itoa(i+1))
		args = append(args, row[column])
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}

func describe(row Row) string {
	encoded, err := json.Marshal(row)
	if err != nil {
		return "a row that cannot be shown"
	}

	return string(encoded)
}
