/*
|--------------------------------------------------------------------------
| Request Timeout
|--------------------------------------------------------------------------
|
| A request gets a deadline in its context, so every query and outbound
| call it makes gives up together when time runs out. Responses are not
| buffered, so streaming still works; a handler that gave up without
| writing anything leaves the client a 503 rather than an empty reply.
|
*/

package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
Timeout gives each request a deadline of d.
*/
func Timeout(d time.Duration) fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			rec := &recorder{ResponseWriter: w}
			next.ServeHTTP(rec, r.WithContext(ctx))

			if rec.written || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return
			}

			refuse(w, r, fault.New(fault.Unavailable, "request_timeout", "The request took too long."))
		})
	}
}
