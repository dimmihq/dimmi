/*
|--------------------------------------------------------------------------
| Query Dates
|--------------------------------------------------------------------------
|
| Date columns take a pgtype date on their queries; the package
| already owns pgx, so callers pass time.Time and the type stays
| inside the framework.
|
*/

package database

import (
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

/*
Date maps a time to the date a query takes, in the same zone.
*/
func Date(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

/*
NumericFloat builds a pgtype.Numeric from a float64 — it exists so
app code never touches pgx types directly.
*/
func NumericFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(f, 'g', -1, 64)); err != nil {
		return pgtype.Numeric{Valid: true}
	}
	return n
}
