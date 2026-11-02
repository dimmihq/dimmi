/*
|--------------------------------------------------------------------------
| Body Limit
|--------------------------------------------------------------------------
|
| A request body is read into memory to be parsed, so its size is capped
| before anything reads it. A body that announces it is too large is
| refused at once; one that grows past the cap while being read fails the
| read, and the handler's error becomes the same 413.
|
*/

package middleware

import (
	"net/http"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
BodyLimit caps request bodies at limit bytes, answering 413
payload_too_large beyond it.
*/
func BodyLimit(limit int64) fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				refuse(w, r, fault.New(fault.TooLarge, fhttp.CodePayloadTooLarge, "The request body is too large."))

				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
