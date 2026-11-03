/*
|--------------------------------------------------------------------------
| Ensure Email Verified
|--------------------------------------------------------------------------
|
| Steps that reach other people, such as inviting them or minting a
| token, wait until the user proved their address, where the install
| asks for it; everything else works from the first minute. The check
| runs after the caller is known.
|
*/

package middleware

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/config"
)

/*
EnsureEmailVerified returns the middleware that refuses a caller whose
email is not proven with exceptions.ErrEmailUnverified, when the
configuration requires a proven email, and lets every caller through
otherwise.
*/
func EnsureEmailVerified(cfg config.VerificationConfig, errs fhttp.Errors) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
			p, _ := appauth.CurrentPrincipal(r.Context())
			if *cfg.Required && !p.User.IsVerified() {
				errs.Render(w, r, exceptions.ErrEmailUnverified)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
