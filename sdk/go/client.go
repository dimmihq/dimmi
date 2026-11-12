/*
Package dimmi is the Go SDK for dimmi, the open-source customer support
platform: a REST client generated from the API's OpenAPI document, signed
identity tokens for the messenger, and webhook verification.

	client, err := dimmi.New(
		dimmi.WithToken(os.Getenv("DIMMI_TOKEN")),
		dimmi.WithWorkspace("ws_..."),
	)
	res, err := client.API.ConversationsIndexWithResponse(ctx, nil)

Every call made through the client carries the token and workspace,
retries 429 and 503 answers after their Retry-After, sends an
Idempotency-Key with each POST, and turns error answers into *Error.
*/
package dimmi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dimmihq/dimmi-go/api"
)

// Version is this SDK's version, sent in the User-Agent.
const Version = "0.1.0"

// DefaultBaseURL is dimmi cloud. Self-hosted installs pass their own address.
const DefaultBaseURL = "https://app.trydimmi.com"

// WorkspaceHeader names the workspace a request acts in.
const WorkspaceHeader = "Dimmi-Workspace"

// IdempotencyHeader makes a POST safe to retry.
const IdempotencyHeader = "Idempotency-Key"

/*
Client talks to the dimmi REST API. API is the generated client with a
typed method per operation; Do, List and Pages work with any path.
*/
type Client struct {
	API *api.ClientWithResponses

	baseURL    string
	token      string
	workspace  string
	maxRetries int
	maxWait    time.Duration
	timeout    time.Duration
	userAgent  string
	inner      http.RoundTripper
	http       *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithToken sets the API token, DIMMI_TOKEN (or DIMMI_API_TOKEN) by default.
func WithToken(token string) Option { return func(c *Client) { c.token = token } }

// WithWorkspace sets the workspace id, ws_..., DIMMI_WORKSPACE by default.
func WithWorkspace(id string) Option { return func(c *Client) { c.workspace = id } }

// WithBaseURL points the client at a self-hosted install, DIMMI_BASE_URL by default.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithMaxRetries sets how often a 429, 503 or dropped connection is retried, 3 by default.
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = n } }

// WithMaxRetryWait caps how long one Retry-After wait may be, 60s by default.
func WithMaxRetryWait(d time.Duration) Option { return func(c *Client) { c.maxWait = d } }

// WithTimeout sets the whole-request timeout, 30s by default.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// WithTransport sets the underlying transport, http.DefaultTransport by default.
func WithTransport(rt http.RoundTripper) Option { return func(c *Client) { c.inner = rt } }

// WithUserAgent appends to the User-Agent sent.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

/*
New makes a client. The token is required; the workspace is required by
every workspace-scoped endpoint.
*/
func New(opts ...Option) (*Client, error) {
	c := &Client{
		token:      firstEnv("DIMMI_TOKEN", "DIMMI_API_TOKEN"),
		workspace:  os.Getenv("DIMMI_WORKSPACE"),
		baseURL:    firstEnv("DIMMI_BASE_URL"),
		maxRetries: 3,
		maxWait:    60 * time.Second,
		timeout:    30 * time.Second,
		inner:      http.DefaultTransport,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.token == "" {
		return nil, errors.New("dimmi: token_missing: pass WithToken or set DIMMI_TOKEN")
	}
	if c.baseURL == "" {
		c.baseURL = DefaultBaseURL
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	if !strings.HasSuffix(c.baseURL, "/api/v1") {
		c.baseURL += "/api/v1"
	}
	c.http = &http.Client{Transport: &transport{client: c}, Timeout: c.timeout}
	generated, err := api.NewClientWithResponses(c.baseURL+"/", api.WithHTTPClient(c.http))
	if err != nil {
		return nil, err
	}
	c.API = generated
	return c, nil
}

// HTTPClient is the configured *http.Client, for calls of your own.
func (c *Client) HTTPClient() *http.Client { return c.http }

// BaseURL is the API root, ending in /api/v1.
func (c *Client) BaseURL() string { return c.baseURL }

type idempotencyKey struct{}

/*
WithIdempotencyKey returns a context whose POST carries key, so a retry
of yours after a timeout cannot do the work twice. Without one, every
POST gets a fresh key that its own retries reuse.
*/
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyKey{}, key)
}

// IdempotencyKey is a request editor for generated calls.
func IdempotencyKey(key string) api.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set(IdempotencyHeader, key)
		return nil
	}
}

// Query is a request editor that adds query parameters to a generated call.
func Query(values url.Values) api.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		q := req.URL.Query()
		for k, vs := range values {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		req.URL.RawQuery = q.Encode()
		return nil
	}
}

/*
Do sends one request to path, relative to /api/v1, and decodes the
answer into out when it is not nil. body is encoded as JSON.
*/
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("dimmi: encode body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	u := c.baseURL + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return unwrapURLError(err)
	}
	defer res.Body.Close()
	if out == nil || res.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("dimmi: decode %s %s: %w", method, path, err)
	}
	return nil
}

// unwrapURLError hands back an *Error the transport raised as is.
func unwrapURLError(err error) error {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return err
}

/*
transport adds auth, workspace and idempotency headers, retries what is
safe to retry, and turns error answers into *Error.
*/
type transport struct{ client *Client }

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	c := t.client
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.workspace != "" && req.Header.Get(WorkspaceHeader) == "" {
		req.Header.Set(WorkspaceHeader, c.workspace)
	}
	ua := "dimmi-go/" + Version
	if c.userAgent != "" {
		ua += " " + c.userAgent
	}
	req.Header.Set("User-Agent", ua)
	if req.Method == http.MethodPost && req.Header.Get(IdempotencyHeader) == "" {
		key, _ := req.Context().Value(idempotencyKey{}).(string)
		if key == "" {
			key = uuid.NewString()
		}
		req.Header.Set(IdempotencyHeader, key)
	}

	var body []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		body, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; ; attempt++ {
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
		}
		res, err := c.inner.RoundTrip(req)
		if err != nil {
			if attempt < c.maxRetries && req.Context().Err() == nil {
				if werr := sleep(req.Context(), backoff(attempt)); werr != nil {
					return nil, werr
				}
				continue
			}
			return nil, err
		}
		if res.StatusCode < 400 {
			return res, nil
		}
		apiErr := readError(res)
		if retryable(res.StatusCode) && attempt < c.maxRetries {
			wait := backoff(attempt)
			if apiErr.RetryAfter > 0 {
				wait = apiErr.RetryAfter
			}
			if wait > c.maxWait {
				return nil, apiErr
			}
			if werr := sleep(req.Context(), wait); werr != nil {
				return nil, werr
			}
			continue
		}
		return nil, apiErr
	}
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable
}

func backoff(attempt int) time.Duration {
	return time.Duration(1<<attempt) * 500 * time.Millisecond
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}
