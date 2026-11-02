/*
|--------------------------------------------------------------------------
| Signed URLs
|--------------------------------------------------------------------------
|
| Routes reached from links in emails carry their authority in the link.
| A link that was changed, forged or has expired is refused before the
| handler sees it.
|
*/

package middleware

import (
	"net/http"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
SignatureVerifier checks the signature of a request's URL, as the URL
generator of the router does.
*/
type SignatureVerifier interface {
	/*
		HasValidSignature reports whether the URL is signed and unexpired.
	*/
	HasValidSignature(r *http.Request) bool
}

/*
Signed refuses requests without a valid URL signature with 403
invalid_signature.
*/
func Signed(v SignatureVerifier) fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !v.HasValidSignature(r) {
				refuse(w, r, fault.New(fault.Forbidden, "invalid_signature", "Invalid signature."))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
