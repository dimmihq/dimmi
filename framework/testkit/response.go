/*
|--------------------------------------------------------------------------
| Response Assertions
|--------------------------------------------------------------------------
|
| A feature test checks what a client would see: the status, the headers
| and the JSON body, addressed by dotted paths such as "data.items.0.id".
| Every assertion returns the response, so the checks on one response
| read as one chain, and a failed check reports the body it looked at.
|
| Validation failures share one shape across the application: status 422
| with every rejected field under "errors", so AssertValidationError only
| needs the names of the fields that must be rejected.
|
*/

package testkit

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

/*
Response is what the handler answered. Status, Header and Body are the
raw response, for checks the assertions do not cover.
*/
type Response struct {
	tb     testing.TB
	Status int
	Header http.Header
	Body   []byte
}

func newResponse(tb testing.TB, res *http.Response) *Response {
	tb.Helper()

	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		tb.Fatalf("read response body: %v", err)
	}

	return &Response{tb: tb, Status: res.StatusCode, Header: res.Header, Body: body}
}

/*
AssertStatus checks the status.
*/
func (r *Response) AssertStatus(status int) *Response {
	r.tb.Helper()

	assert.Equal(r.tb, status, r.Status, "unexpected status; body: %s", r.Body)

	return r
}

/*
AssertOK checks the status is 200.
*/
func (r *Response) AssertOK() *Response {
	r.tb.Helper()

	return r.AssertStatus(http.StatusOK)
}

/*
AssertCreated checks the status is 201.
*/
func (r *Response) AssertCreated() *Response {
	r.tb.Helper()

	return r.AssertStatus(http.StatusCreated)
}

/*
AssertNoContent checks the status is 204.
*/
func (r *Response) AssertNoContent() *Response {
	r.tb.Helper()

	return r.AssertStatus(http.StatusNoContent)
}

/*
AssertUnauthorized checks the status is 401.
*/
func (r *Response) AssertUnauthorized() *Response {
	r.tb.Helper()

	return r.AssertStatus(http.StatusUnauthorized)
}

/*
AssertForbidden checks the status is 403.
*/
func (r *Response) AssertForbidden() *Response {
	r.tb.Helper()

	return r.AssertStatus(http.StatusForbidden)
}

/*
AssertNotFound checks the status is 404.
*/
func (r *Response) AssertNotFound() *Response {
	r.tb.Helper()

	return r.AssertStatus(http.StatusNotFound)
}

/*
AssertHeader checks the header has the value.
*/
func (r *Response) AssertHeader(name, value string) *Response {
	r.tb.Helper()

	assert.Equal(r.tb, value, r.Header.Get(name), "unexpected %s header", name)

	return r
}

/*
AssertJSONPath checks the value at the path equals the expected one, as
JSON compares them: 1 and 1.0 are equal, and a struct equals the object
it encodes to.
*/
func (r *Response) AssertJSONPath(path string, expected any) *Response {
	r.tb.Helper()

	found := r.lookup(path)
	if !found.Exists() {
		r.tb.Errorf("no JSON value at %q in body: %s", path, r.Body)

		return r
	}

	want, err := normalize(expected)
	if err != nil {
		r.tb.Errorf("encode expected value for %q: %v", path, err)

		return r
	}

	assert.Equal(r.tb, want, found.Value(), "unexpected JSON value at %q in body: %s", path, r.Body)

	return r
}

/*
AssertJSONMissing checks there is no value at the path.
*/
func (r *Response) AssertJSONMissing(path string) *Response {
	r.tb.Helper()

	if found := r.lookup(path); found.Exists() {
		r.tb.Errorf("unexpected JSON value at %q: %s", path, found.Raw)
	}

	return r
}

/*
AssertValidationError checks the request was rejected as invalid, with
an error for each of the fields. A field is named the way clients
address it, "items.0.name" for a nested one.
*/
func (r *Response) AssertValidationError(fields ...string) *Response {
	r.tb.Helper()

	r.AssertStatus(http.StatusUnprocessableEntity)

	errs := r.lookup("errors")
	for _, field := range fields {
		if !errs.Get(gjson.Escape(field)).Exists() {
			r.tb.Errorf("no validation error for %q in body: %s", field, r.Body)
		}
	}

	return r
}

/*
String returns the value at the path as a string, such as the id of a
created record, or "" when there is none.
*/
func (r *Response) String(path string) string {
	return r.lookup(path).String()
}

/*
Cookie returns the cookie the response set under the name, failing the
test when it set none, so a test can send it with the next request.
*/
func (r *Response) Cookie(name string) *http.Cookie {
	r.tb.Helper()

	for _, line := range r.Header.Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(line)
		if err == nil && cookie.Name == name {
			return cookie
		}
	}

	r.tb.Fatalf("response set no %s cookie; it set %v", name, r.Header.Values("Set-Cookie"))

	return nil
}

/*
Decode decodes the body into v, failing the test when it is not valid
JSON for v.
*/
func (r *Response) Decode(v any) {
	r.tb.Helper()

	if err := json.Unmarshal(r.Body, v); err != nil {
		r.tb.Fatalf("decode response body: %v; body: %s", err, r.Body)
	}
}

func (r *Response) lookup(path string) gjson.Result {
	r.tb.Helper()

	if !gjson.ValidBytes(r.Body) {
		r.tb.Errorf("response body is not JSON: %s", strings.TrimSpace(string(r.Body)))

		return gjson.Result{}
	}

	return gjson.GetBytes(r.Body, path)
}

/*
normalize turns the value into what decoding its JSON gives, so it
compares equal to a value read from a body.
*/
func normalize(v any) (any, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}

	return decoded, nil
}
