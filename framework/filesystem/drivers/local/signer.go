/*
|--------------------------------------------------------------------------
| Local Signed URLs
|--------------------------------------------------------------------------
|
| A local disk signs temporary URLs with the application's signed links,
| which the application serves and checks. These names keep the checks
| reachable from this package.
|
*/

package local

import (
	"net/http"
	"time"

	"github.com/dimmihq/dimmi/framework/filesystem/signedlinks"
)

/*
Errors from checking a signed request; see the signedlinks package.
*/
var (
	ErrInvalidSignature = signedlinks.ErrInvalidSignature
	ErrExpired          = signedlinks.ErrExpired
)

/*
Verify checks a request made to a temporary URL of a local disk with the
disk's signing key; see signedlinks.Verify.
*/
func Verify(r *http.Request, path, signingKey string, now time.Time) error {
	return signedlinks.Verify(r, path, signingKey, now)
}
