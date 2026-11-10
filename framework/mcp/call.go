/*
|--------------------------------------------------------------------------
| Calling An Operation In Process
|--------------------------------------------------------------------------
|
| A tool built from an API operation makes the very request the API
| would receive and hands it to the API's own handler, in process, so
| everything the API checks applies unchanged. The request repeats what
| matters of the agent's own request: its credential, language and
| address, and is routed from scratch, so the API works everything out
| from it rather than from the MCP request it was made for.
|
*/

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/dimmihq/dimmi/framework/routing"
)

/*
Origin is what each call repeats of the agent's request: the headers
kept, the client's address and the host it called.
*/
type Origin struct {
	Header     http.Header
	RemoteAddr string
	Host       string
}

/*
OriginOf keeps the named headers of a request with its address and host.
*/
func OriginOf(r *http.Request, headers ...string) Origin {
	kept := http.Header{}
	for _, name := range headers {
		if value := r.Header.Get(name); value != "" {
			kept.Set(name, value)
		}
	}

	return Origin{Header: kept, RemoteAddr: r.RemoteAddr, Host: r.Host}
}

/*
Response is what the handler answered a call with.
*/
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

/*
StatusText names the response's status, such as "Not Found".
*/
func (a Response) StatusText() string {
	return http.StatusText(a.Status)
}

/*
Decoded reads the response's JSON body, the raw text when it is not JSON,
and nil when it is empty.
*/
func (a Response) Decoded() any {
	if len(bytes.TrimSpace(a.Body)) == 0 {
		return nil
	}

	var v any
	if err := json.Unmarshal(a.Body, &v); err != nil {
		return string(a.Body)
	}

	return v
}

/*
API is where operations are served: the handler of the API and the path
prefix it is served under.
*/
type API struct {
	Handler http.Handler
	Prefix  string
}

/*
Call makes the operation's request with the arguments and serves it
with the API's handler, routed from scratch.
*/
func (o Operation) Call(ctx context.Context, api API, args map[string]any, from Origin) (Response, error) {
	e, err := o.Encode(args)
	if err != nil {
		return Response{}, fmt.Errorf("build request for %s: %w", o.ID, err)
	}

	target := api.Prefix + e.Path
	if len(e.Query) > 0 {
		target += "?" + e.Query.Encode()
	}

	var body io.Reader
	if e.Body != nil {
		body = bytes.NewReader(e.Body)
	}

	req := httptest.NewRequestWithContext(routing.Unrouted(ctx), o.Method, target, body)
	req.Header = from.Header.Clone()
	if req.Header == nil {
		req.Header = http.Header{}
	}

	req.Header.Set("Accept", "application/json")
	if e.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.RemoteAddr, req.Host = from.RemoteAddr, from.Host

	rec := &recorder{header: http.Header{}, status: http.StatusOK}
	api.Handler.ServeHTTP(rec, req)

	return Response{Status: rec.status, Header: rec.header, Body: rec.body.Bytes()}, nil
}

/*
recorder keeps what a handler writes.
*/
type recorder struct {
	header  http.Header
	status  int
	body    bytes.Buffer
	written bool
}

/*
Header returns the response headers.
*/
func (r *recorder) Header() http.Header {
	return r.header
}

/*
WriteHeader keeps the first status written.
*/
func (r *recorder) WriteHeader(status int) {
	if r.written {
		return
	}

	r.status, r.written = status, true
}

/*
Write keeps the body.
*/
func (r *recorder) Write(p []byte) (int, error) {
	r.written = true

	return r.body.Write(p)
}
