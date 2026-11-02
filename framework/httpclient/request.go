/*
|--------------------------------------------------------------------------
| Requests
|--------------------------------------------------------------------------
|
| A request is built fluently and sent with a context. The response body
| is read here, up to the cap, so a hostile server cannot exhaust memory
| and the timeout covers the body as well as the headers. Errors name the
| method and host only: URLs often carry tokens that must not end up in
| logs.
|
*/

package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"resty.dev/v3"

	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
)

/*
Request is an outbound request being built. It is not safe for
concurrent use; start one per request with Client.New.
*/
type Request struct {
	client    *Client
	header    http.Header
	timeout   time.Duration
	maxBody   int64
	redirects bool
	stream    bool
}

/*
WithToken sends the token as a bearer Authorization header.
*/
func (r *Request) WithToken(token string) *Request {
	return r.WithHeader("Authorization", "Bearer "+token)
}

/*
WithHeader sets a request header, replacing earlier values.
*/
func (r *Request) WithHeader(name, value string) *Request {
	r.header.Set(name, value)

	return r
}

/*
Accept sets the content type the caller wants back.
*/
func (r *Request) Accept(contentType string) *Request {
	return r.WithHeader("Accept", contentType)
}

/*
Timeout bounds this request, body included, instead of the client's
timeout.
*/
func (r *Request) Timeout(d time.Duration) *Request {
	r.timeout = d

	return r
}

/*
MaxBody caps this response body in bytes instead of the client's cap.
*/
func (r *Request) MaxBody(n int64) *Request {
	r.maxBody = n

	return r
}

/*
WithoutRedirects returns a redirect response as is instead of following
it.
*/
func (r *Request) WithoutRedirects() *Request {
	r.redirects = false

	return r
}

/*
Stream leaves the body unread so the caller can read it as it arrives,
for server-sent events and large downloads. The size cap does not apply;
the timeout covers the request until the response is closed.
*/
func (r *Request) Stream() *Request {
	r.stream = true

	return r
}

/*
Get sends a GET request.
*/
func (r *Request) Get(ctx context.Context, rawURL string) (*Response, error) {
	return r.send(ctx, http.MethodGet, rawURL, nil)
}

/*
Delete sends a DELETE request.
*/
func (r *Request) Delete(ctx context.Context, rawURL string) (*Response, error) {
	return r.send(ctx, http.MethodDelete, rawURL, nil)
}

/*
Post sends a POST request. A string, byte slice or reader body is sent
as is; any other value is encoded as JSON.
*/
func (r *Request) Post(ctx context.Context, rawURL string, body any) (*Response, error) {
	return r.send(ctx, http.MethodPost, rawURL, body)
}

/*
Put sends a PUT request with a body, encoded as for Post.
*/
func (r *Request) Put(ctx context.Context, rawURL string, body any) (*Response, error) {
	return r.send(ctx, http.MethodPut, rawURL, body)
}

/*
Patch sends a PATCH request with a body, encoded as for Post.
*/
func (r *Request) Patch(ctx context.Context, rawURL string, body any) (*Response, error) {
	return r.send(ctx, http.MethodPatch, rawURL, body)
}

func (r *Request) send(ctx context.Context, method, rawURL string, body any) (*Response, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", strings.ToLower(method), err)
	}

	label := strings.ToLower(method) + " " + target.Host
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("%s: %w: %q scheme", label, ErrBlockedHost, target.Scheme)
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)

	resp, err := r.build(ctx, body).Execute(method, rawURL)
	if err != nil {
		cancel()

		return nil, fmt.Errorf("%s: %w", label, translate(err))
	}

	if r.stream {
		return r.streamed(ctx, resp, cancel), nil
	}

	defer cancel()

	return r.read(label, resp)
}

func (r *Request) build(ctx context.Context, body any) *resty.Request {
	client := r.client.follow
	if !r.redirects {
		client = r.client.stay
	}

	req := client.R().SetContext(ctx).SetResponseDoNotParse(true)
	maps.Copy(req.Header, r.header)

	if body != nil {
		req.SetBody(body)
	}

	return req
}

func (r *Request) streamed(ctx context.Context, resp *resty.Response, cancel context.CancelFunc) *Response {
	return &Response{
		Status: resp.StatusCode(),
		Header: resp.Header(),
		stream: &streamBody{ReadCloser: resp.Body, cancel: cancel, ended: ctx.Err},
	}
}

/*
read reads the body, reading one byte past the cap to tell a body that
fills the cap exactly from one that exceeds it.
*/
func (r *Request) read(label string, resp *resty.Response) (*Response, error) {
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, r.maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("%s: read body: %w", label, translate(err))
	}

	if int64(len(body)) > r.maxBody {
		return nil, fmt.Errorf("%s: %w: over %d bytes", label, ErrTooLarge, r.maxBody)
	}

	return &Response{Status: resp.StatusCode(), Header: resp.Header(), Body: body}, nil
}

/*
translate maps a refused connection to ErrBlockedHost, keeping the
original error in the chain.
*/
func translate(err error) error {
	if errors.Is(err, safedial.ErrBlocked) && !errors.Is(err, ErrBlockedHost) {
		return fmt.Errorf("%w: %w", ErrBlockedHost, err)
	}

	return err
}
