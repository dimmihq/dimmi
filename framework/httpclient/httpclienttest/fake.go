/*
|--------------------------------------------------------------------------
| Fake Transport
|--------------------------------------------------------------------------
|
| Tests stub responses by URL pattern instead of reaching the network.
| The fake replaces only the transport, so everything above it runs for
| real: redirects are followed through stubbed Location headers, the size
| cap and timeouts apply, and SDKs on the standard client see the same
| stubs. Private destinations written as IP literals or localhost are
| still refused, and a request whose context has ended fails with the
| context's error, as it would on the network.
|
| A request no stub matches gets an empty 200 response, unless the test
| prevents stray requests; then it fails with ErrStrayRequest.
|
| The fake lives in its own package, imported only by tests, so the mock
| library behind it never reaches a production binary.
|
*/

package httpclienttest

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jarcoal/httpmock"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
ErrStrayRequest comes from a fake that prevents requests no stub
matches.
*/
var ErrStrayRequest = errors.New("httpclient: request matches no stub")

/*
SentRequest is a request the fake received.
*/
type SentRequest struct {
	Method string
	URL    *url.URL
	Header http.Header
	Body   []byte
}

/*
Fake is a transport answering from stubs and recording every request.
*/
type Fake struct {
	client *httpclient.Client
	mock   *httpmock.MockTransport

	mu   sync.Mutex
	sent []SentRequest
}

/*
New replaces the client's transport with a fresh fake and returns it,
so every holder of the client, SDKs included, stops reaching the
network at once. It panics with httpclient.ErrFakesNotAllowed unless
the client's configuration allows fakes.
*/
func New(client *httpclient.Client) *Fake {
	mock := httpmock.NewMockTransport()
	mock.RegisterNoResponder(responder(httpclient.Response{}))

	fake := &Fake{client: client, mock: mock}
	client.ReplaceTransport(fake)

	return fake
}

/*
Stub answers requests matching the pattern with the response. A pattern
is a URL where * matches anything, optionally led by a method:
"POST https://api.example.com/v1/*". Stubs are tried in the order they
were added.
*/
func (f *Fake) Stub(pattern string, resp httpclient.Response) *Fake {
	methods, glob := splitPattern(pattern)
	for _, method := range methods {
		f.mock.RegisterRegexpResponder(method, glob, responder(resp))
	}

	return f
}

/*
splitPattern reads a stub pattern into the methods it covers and the
URL expression it matches.
*/
func splitPattern(pattern string) ([]string, *regexp.Regexp) {
	methods := []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions,
	}

	if method, rest, ok := strings.Cut(pattern, " "); ok {
		methods, pattern = []string{strings.ToUpper(method)}, rest
	}

	glob := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*")

	return methods, regexp.MustCompile("^" + glob + "$")
}

/*
StubSequence answers requests matching the pattern with the responses
in turn, repeating the last one once they run out, so a test can
script a provider that fails before it succeeds or a poll that waits
before it answers.
*/
func (f *Fake) StubSequence(pattern string, resps ...httpclient.Response) *Fake {
	if len(resps) == 0 {
		return f.Stub(pattern, httpclient.Response{})
	}

	var mu sync.Mutex
	next := 0

	methods, glob := splitPattern(pattern)
	for _, method := range methods {
		f.mock.RegisterRegexpResponder(method, glob, func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			resp := resps[min(next, len(resps)-1)]
			next++
			mu.Unlock()

			return responder(resp)(req)
		})
	}

	return f
}

/*
Serve answers requests matching the pattern by running the handler in
process, so a test can stand up a real protocol server, such as an MCP
server, behind a URL the client believes is remote.
*/
func (f *Fake) Serve(pattern string, h http.Handler) *Fake {
	methods, glob := splitPattern(pattern)
	for _, method := range methods {
		f.mock.RegisterRegexpResponder(method, glob, func(req *http.Request) (*http.Response, error) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			resp := rec.Result()
			resp.Request = req

			return resp, nil
		})
	}

	return f
}

/*
Sent returns every request the fake received, oldest first.
*/
func (f *Fake) Sent() []SentRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.sent)
}

/*
PreventStrayRequests makes a request that matches no stub fail with
ErrStrayRequest instead of getting an empty response.
*/
func (f *Fake) PreventStrayRequests() *Fake {
	f.mock.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("%w: %s %s", ErrStrayRequest, req.Method, req.URL.Redacted())
	})

	return f
}

/*
AssertSent fails the test unless some recorded request matches.
*/
func (f *Fake) AssertSent(tb testing.TB, match func(req SentRequest) bool) {
	tb.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	if slices.ContainsFunc(f.sent, match) {
		return
	}

	tb.Errorf("httpclient: no matching request among %d sent", len(f.sent))
}

/*
RoundTrip refuses private literal destinations, records the request and
answers from the stubs. A request whose context already ended fails
with the context's error and is not recorded, as a real transport never
sends it.
*/
func (f *Fake) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, fmt.Errorf("%s %s: %w", req.Method, req.URL.Host, err)
	}

	if err := f.client.CheckAddress(hostPort(req.URL)); err != nil {
		return nil, fmt.Errorf("dial %s: %w", req.URL.Host, err)
	}

	body, err := readBody(req)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.sent = append(f.sent, SentRequest{Method: req.Method, URL: req.URL, Header: req.Header.Clone(), Body: body})
	f.mu.Unlock()

	return f.mock.RoundTrip(req)
}

func responder(resp httpclient.Response) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		header := resp.Header.Clone()
		if header == nil {
			header = http.Header{}
		}

		return &http.Response{
			StatusCode:    cmp.Or(resp.Status, http.StatusOK),
			Header:        header,
			Body:          io.NopCloser(bytes.NewReader(resp.Body)),
			ContentLength: int64(len(resp.Body)),
			Request:       req,
		}, nil
	}
}

/*
readBody reads the request body for the record and puts it back for the
stubs.
*/
func readBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}

	req.Body = io.NopCloser(bytes.NewReader(body))

	return body, nil
}

func hostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}

	port := "80"
	if u.Scheme == "https" {
		port = "443"
	}

	return net.JoinHostPort(u.Hostname(), port)
}
