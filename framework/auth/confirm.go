/*
|--------------------------------------------------------------------------
| Recent Authentication
|--------------------------------------------------------------------------
|
| A stolen session should not be enough to lock the owner out. Changes
| that would, such as a new password, a new email or a new token, ask
| the caller to have proved who they are recently: logged in, or entered
| their password again, within a short window. Outside it the client is
| told to confirm and try again.
|
*/

package auth

import (
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
)

/*
ErrConfirmationRequired is the fault for a caller who must confirm
their identity before going on.
*/
var ErrConfirmationRequired = fault.New(fault.Locked, "password_confirmation_required",
	"Confirm your password to continue.")

/*
Confirmed reports whether the principal proved their identity within
the window before now.
*/
func Confirmed(p Principal, now time.Time, window time.Duration) bool {
	at := p.ConfirmedAt()

	return !at.IsZero() && !now.After(at.Add(window))
}

/*
RequireConfirmation returns the middleware that lets a request through
only when its caller proved their identity within the window, rendering
ErrConfirmationRequired otherwise. It runs after the auth middleware.
*/
func RequireConfirmation(clk clock.Clock, window time.Duration, errs http.Errors) http.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := From(r.Context())
			if !ok {
				errs.Render(w, r, ErrUnauthenticated)

				return
			}

			if !Confirmed(p, clk.Now(), window) {
				errs.Render(w, r, ErrConfirmationRequired)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
