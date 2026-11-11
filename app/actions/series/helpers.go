/*
|--------------------------------------------------------------------------
| Small Helpers
|--------------------------------------------------------------------------
|
| Conversions the engine needs in several places: ids stored as text in
| a wait, optional text for the trace, counts bounded for a query, and
| jobs that find their row gone.
|
*/

package series

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
parseID reads an id stored as text.
*/
func parseID(s string) (ids.UUID, error) {
	return ids.Parse(s)
}

/*
optional returns the text, nil when it is empty.
*/
func optional(text string) *string {
	if text == "" {
		return nil
	}

	return &text
}

/*
int32Of bounds a count to what a query takes.
*/
func int32Of(n int) int32 {
	return int32(min(max(n, 0), 1<<30))
}

/*
gone treats a row that no longer exists as nothing left to do.
*/
func gone(err error, what string) error {
	if err == nil || errors.Is(err, exceptions.ErrNotFound) || errors.Is(err, database.ErrNotFound) {
		return nil
	}

	return wrap(what, err)
}
