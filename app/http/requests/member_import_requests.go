/*
|--------------------------------------------------------------------------
| Member Import Requests
|--------------------------------------------------------------------------
|
| A member import arrives as a multipart form: the CSV file in the file
| field and, for a dry run, dry_run set to true. The import reads the
| file itself, so the request only carries the form.
|
*/

package requests

import (
	"strconv"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
StoreMemberImportRequest is the input of POST /member-imports.
*/
type StoreMemberImportRequest struct {
	RawBody fhttp.MultipartForm
}

/*
DryRun reports whether the form asks only to check the file.
*/
func (r *StoreMemberImportRequest) DryRun() bool {
	values := r.RawBody.Value["dry_run"]
	if len(values) == 0 {
		return false
	}

	on, err := strconv.ParseBool(values[0])

	return err == nil && on
}
