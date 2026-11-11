/*
|--------------------------------------------------------------------------
| Panic Recovery
|--------------------------------------------------------------------------
|
| A panic in one request must not take the process down with every other
| request in flight. It is turned into a plain 500 for the client, and
| logged and reported with its stack, because it is always a bug.
|
*/

package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
Recover answers a panicking request with a bare 500 and logs and
reports the panic with its stack. A deliberate abort of the
response is passed on, as the server expects.
*/
func Recover(errs fhttp.Errors) fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				p := recover()
				if p == nil {
					return
				}

				if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(p)
				}

				errs.RenderBug(w, r, fmt.Errorf("panic: %v\n%s", p, debug.Stack()))
			}()

			next.ServeHTTP(w, r)
		})
	}
}
