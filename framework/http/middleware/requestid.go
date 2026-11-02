/*
|--------------------------------------------------------------------------
| Request IDs
|--------------------------------------------------------------------------
|
| One id follows a request through the logs of every service it touches,
| so a support request that quotes it leads straight to what happened. An
| id set by a proxy in front is kept when it looks like an id; anything
| else is replaced, so a client cannot write arbitrary text into logs.
|
*/

package middleware

import (
	"net/http"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
DefaultRequestIDHeader is the header RequestID reads and writes when
given none.
*/
const DefaultRequestIDHeader = "X-Request-Id"

/*
RequestID gives every request an id, in the context, in every log line
written with that context and in the response header.
*/
func RequestID(header string) fhttp.Middleware {
	if header == "" {
		header = DefaultRequestIDHeader
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(header)
			if !validRequestID(id) {
				id = ids.New().String()
			}

			w.Header().Set(header, id)
			ctx := log.With(fhttp.WithRequestID(r.Context(), id), log.String("request_id", id))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

/*
validRequestID accepts up to 128 letters, digits, dots, dashes, colons
and underscores.
*/
func validRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}

	for _, c := range id {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' ||
			c == '.' || c == ':'
		if !ok {
			return false
		}
	}

	return true
}
