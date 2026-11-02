/*
|--------------------------------------------------------------------------
| Maintenance Mode
|--------------------------------------------------------------------------
|
| While the application is down for maintenance every request gets a 503
| that tells clients when to come back, except the paths that must keep
| answering, such as health checks. If the switch itself cannot be read
| the application stays up: a broken flag must not cause an outage.
|
*/

package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
MaintenanceMode reports whether the application is down for maintenance.
*/
type MaintenanceMode interface {
	/*
		Active reports whether maintenance mode is on.
	*/
	Active(ctx context.Context) (bool, error)
}

/*
Maintenance answers 503 maintenance while the mode is active, with
Retry-After when retryAfter is positive. Paths in except keep working;
one ending in "*" covers every path starting with the rest.
*/
func Maintenance(mode MaintenanceMode, retryAfter time.Duration, except ...string) fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if fhttp.MatchesPath(r.URL.Path, except...) {
				next.ServeHTTP(w, r)

				return
			}

			active, err := mode.Active(ctx)
			if err != nil {
				log.From(ctx).WarnContext(ctx, "maintenance mode unknown", log.Err(err))
			}

			if !active {
				next.ServeHTTP(w, r)

				return
			}

			if retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter/time.Second)))
			}

			refuse(w, r, fault.New(fault.Unavailable, "maintenance", "Down for maintenance. Please try again soon."))
		})
	}
}
