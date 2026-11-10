/*
|--------------------------------------------------------------------------
| Numbered Pages
|--------------------------------------------------------------------------
|
| Directory lists in the dashboard show how many rows match and numbered
| pages, beside the cursor every client can follow. total=true counts the
| matching rows, up to a cap so a count never scans a whole large
| workspace, and page=N starts at that page within the counted rows.
|
*/

package v1

import (
	"context"
	"strconv"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
)

/*
PageListing is what a list that takes no filters, orders or inclusions
accepts: only its page size, its cursor and search text.
*/
func PageListing() pagination.Allow {
	return pagination.Allow{}
}

/*
ErrInvalidPage refuses a page number that is not a whole number from 1
or lies past the counted rows.
*/
var ErrInvalidPage = fault.New(fault.Invalid, "invalid_parameter",
	"The page must be a whole number from 1 within the counted rows.").With("parameter", "page")

/*
countedRows is how far numbered pages reach: the cap every list counts
its rows up to.
*/
const countedRows = 10_000

/*
numberedPage reads where a numbered page starts, zero without one, and
whether to count.
*/
func numberedPage(ctx context.Context, limit int) (int, bool, error) {
	r, _ := fhttp.RequestFrom(ctx)
	values := r.URL.Query()
	count := values.Get("total") == "true"

	raw := values.Get("page")
	if raw == "" {
		return 0, count, nil
	}

	number, err := strconv.Atoi(raw)
	if err != nil || number < 1 || (number-1)*limit >= countedRows {
		return 0, count, ErrInvalidPage
	}

	return (number - 1) * limit, count, nil
}
