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
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

/*
Date maps a time to the date a query takes, in the same zone.
*/
func Date(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}
