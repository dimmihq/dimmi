/*
|--------------------------------------------------------------------------
| Middleware Responses
|--------------------------------------------------------------------------
|
| Middleware that refuses a request answers with the same JSON envelope a
| handler would, so a client cannot tell which layer said no. The faults
| refused here are the client's problem and are never reported as bugs.
|
*/

package middleware

import (
	"bytes"
	"net/http"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
refuse renders the error as the response.
*/
func refuse(w http.ResponseWriter, r *http.Request, err error) {
	fhttp.Errors{}.Render(w, r, err)
}

/*
recorder remembers the status a handler sent and whether it wrote
anything, and copies the body when asked to. It unwraps to the original
writer, so flushing and hijacking still reach the connection.
*/
type recorder struct {
	http.ResponseWriter
	status  int
	written bool
	body    *bytes.Buffer
}

/*
WriteHeader remembers the first status sent and passes it on.
*/
func (rec *recorder) WriteHeader(status int) {
	if !rec.written {
		rec.status = status
		rec.written = true
	}

	rec.ResponseWriter.WriteHeader(status)
}

/*
Write remembers that the response started, copies the bytes when asked
to and passes them on.
*/
func (rec *recorder) Write(b []byte) (int, error) {
	if !rec.written {
		rec.status = http.StatusOK
		rec.written = true
	}

	if rec.body != nil {
		rec.body.Write(b)
	}

	return rec.ResponseWriter.Write(b)
}

/*
Unwrap returns the wrapped writer.
*/
func (rec *recorder) Unwrap() http.ResponseWriter {
	return rec.ResponseWriter
}
