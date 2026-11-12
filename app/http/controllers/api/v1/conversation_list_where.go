/*
|--------------------------------------------------------------------------
| Unsaved View Conditions
|--------------------------------------------------------------------------
|
| A teammate can try a view's conditions on the list before saving them.
| filter[where] carries the definition a saved view would hold, as
| base64url JSON so its commas survive the comma-separated filters. It
| narrows whatever view the list shows, and is checked by the same
| compiler as a saved view, so what applies is exactly what would save.
|
*/

package v1

import (
	"encoding/base64"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/actions/conversations"
)

/*
maxWhereLength is the longest encoded definition a list accepts, well
past the largest a saved view may hold.
*/
const maxWhereLength = 16 << 10

/*
where reads filter[where] into the definition the list applies.
*/
func (p filterParser) where(f *conversations.Filters) error {
	encoded := p.first("where")
	if encoded == "" {
		return nil
	}

	if len(encoded) > maxWhereLength {
		return pagination.ErrInvalidFilter.With("parameter", "where")
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || !json.Valid(raw) {
		return pagination.ErrInvalidFilter.With("parameter", "where")
	}

	f.Definition = raw

	return nil
}
