/*
|--------------------------------------------------------------------------
| HTTP Test Client
|--------------------------------------------------------------------------
|
| Feature tests drive the application the way its clients do: a request
| goes through the real handler, router, middleware and all, and the test
| reads the response. Nothing listens on a port; the handler serves the
| request in process, so tests stay fast and run in parallel.
|
| Who the caller is depends on how the application authenticates, so
| ActingAs hands the principal to an impersonator the test picks. The
| default stores it in the request context, which suits handlers that
| read it there; an application whose guards read a credential instead
| passes one that sets the header or cookie its test guard trusts.
|
*/

package testkit

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
JSON is a JSON object to send as a request body.
*/
type JSON = map[string]any

/*
Impersonator returns the request as made by the principal.
*/
type Impersonator func(r *http.Request, principal any) *http.Request

/*
ContextPrincipal is the default impersonator: it stores the principal in
the request context, where guards store the callers they authenticate.
*/
func ContextPrincipal(r *http.Request, principal any) *http.Request {
	return r.WithContext(fhttp.WithPrincipal(r.Context(), principal))
}

/*
Client sends requests to a handler in process. Every setter returns a
copy, so one client can be shared by subtests that each add their own
caller, headers or cookies.
*/
type Client struct {
	tb          testing.TB
	handler     http.Handler
	header      http.Header
	cookies     []*http.Cookie
	impersonate Impersonator
	principal   any
}

/*
NewClient returns a client for the handler. Requests ask for JSON unless
a header says otherwise.
*/
func NewClient(tb testing.TB, handler http.Handler) *Client {
	tb.Helper()

	return &Client{
		tb:          tb,
		handler:     handler,
		header:      http.Header{"Accept": {"application/json"}},
		impersonate: ContextPrincipal,
	}
}

/*
ImpersonateWith returns a client that makes requests as a principal
through the impersonator, for applications whose guards authenticate a
credential rather than read the context.
*/
func (c *Client) ImpersonateWith(impersonate Impersonator) *Client {
	clone := c.clone()
	clone.impersonate = impersonate

	return clone
}

/*
ActingAs returns a client that makes every request as the principal.
*/
func (c *Client) ActingAs(principal any) *Client {
	clone := c.clone()
	clone.principal = principal

	return clone
}

/*
WithHeader returns a client that sends the header with every request,
replacing any value it had.
*/
func (c *Client) WithHeader(name, value string) *Client {
	clone := c.clone()
	clone.header.Set(name, value)

	return clone
}

/*
WithCookie returns a client that sends the cookie with every request.
*/
func (c *Client) WithCookie(cookie *http.Cookie) *Client {
	clone := c.clone()
	clone.cookies = append(clone.cookies, cookie)

	return clone
}

/*
GetJSON sends a GET request.
*/
func (c *Client) GetJSON(path string) *Response {
	c.tb.Helper()

	return c.Send(http.MethodGet, path, nil)
}

/*
PostJSON sends a POST request with the body encoded as JSON.
*/
func (c *Client) PostJSON(path string, body any) *Response {
	c.tb.Helper()

	return c.Send(http.MethodPost, path, body)
}

/*
PatchJSON sends a PATCH request with the body encoded as JSON.
*/
func (c *Client) PatchJSON(path string, body any) *Response {
	c.tb.Helper()

	return c.Send(http.MethodPatch, path, body)
}

/*
PutJSON sends a PUT request with the body encoded as JSON.
*/
func (c *Client) PutJSON(path string, body any) *Response {
	c.tb.Helper()

	return c.Send(http.MethodPut, path, body)
}

/*
Delete sends a DELETE request without a body.
*/
func (c *Client) Delete(path string) *Response {
	c.tb.Helper()

	return c.Send(http.MethodDelete, path, nil)
}

/*
Send sends a request and returns the response. A nil body sends none, a
byte slice or string is sent as it is, and anything else is encoded as
JSON. A body is labeled JSON unless the client set its own content type.
*/
func (c *Client) Send(method, path string, body any) *Response {
	c.tb.Helper()

	payload, err := encodeBody(body)
	if err != nil {
		c.tb.Fatalf("encode %s %s body: %v", method, path, err)
	}

	return c.request(method, path, payload, "application/json")
}

/*
PutFile uploads the contents as a multipart form with one file in the
field.
*/
func (c *Client) PutFile(path, field, name string, contents []byte) *Response {
	c.tb.Helper()

	return c.sendForm(http.MethodPut, path, FormFile{Field: field, Name: name, Contents: contents}, nil)
}

/*
FormFile is one file of a multipart form: its field, its file name and
its contents.
*/
type FormFile struct {
	Field    string
	Name     string
	Contents []byte
}

/*
PostForm posts a multipart form with the file and the plain values.
*/
func (c *Client) PostForm(path string, file FormFile, values map[string]string) *Response {
	c.tb.Helper()

	return c.sendForm(http.MethodPost, path, file, values)
}

/*
sendForm sends a multipart form with the file and the plain values.
*/
func (c *Client) sendForm(method, path string, file FormFile, values map[string]string) *Response {
	c.tb.Helper()

	var body bytes.Buffer

	form := multipart.NewWriter(&body)

	for name, value := range values {
		if err := form.WriteField(name, value); err != nil {
			c.tb.Fatalf("write field for %s %s: %v", method, path, err)
		}
	}

	part, err := form.CreateFormFile(file.Field, file.Name)
	if err != nil {
		c.tb.Fatalf("start form for %s %s: %v", method, path, err)
	}

	if _, err := part.Write(file.Contents); err != nil {
		c.tb.Fatalf("write form for %s %s: %v", method, path, err)
	}

	if err := form.Close(); err != nil {
		c.tb.Fatalf("finish form for %s %s: %v", method, path, err)
	}

	return c.request(method, path, &body, form.FormDataContentType())
}

/*
request makes the request, naming the payload's type when there is one.
*/
func (c *Client) request(method, path string, payload io.Reader, contentType string) *Response {
	c.tb.Helper()

	r := httptest.NewRequestWithContext(c.tb.Context(), method, path, payload)
	r.Header = c.header.Clone()

	if payload != nil && r.Header.Get("Content-Type") == "" {
		r.Header.Set("Content-Type", contentType)
	}

	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}

	if c.principal != nil {
		r = c.impersonate(r, c.principal)
	}

	w := httptest.NewRecorder()
	c.handler.ServeHTTP(w, r)

	return newResponse(c.tb, w.Result())
}

func (c *Client) clone() *Client {
	clone := *c
	clone.header = c.header.Clone()
	clone.cookies = append([]*http.Cookie(nil), c.cookies...)

	return &clone
}

func encodeBody(body any) (io.Reader, error) {
	switch b := body.(type) {
	case nil:
		return nil, nil
	case []byte:
		return bytes.NewReader(b), nil
	case string:
		return bytes.NewReader([]byte(b)), nil
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(encoded), nil
}
