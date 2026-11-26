/*
Package dimmi talks to dimmi's migrations API on the customer's behalf:
it starts a migration, uploads files, sends record batches, reads
progress and rolls back. It signs in with the OAuth token connect saved
and refreshes it when it expires.

The endpoints are documented in dimmi's REST API reference under
Migrations. The generated Go SDK covers them once its contract holds
them; this client keeps the CLI independent of the SDK's generated code,
which is built at release time, not committed.
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
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
Error is an error answer from dimmi. Message is dimmi's own words,
which never echo a record's content.
*/
type Error struct {
	Status  int
	Code    string
	Message string
}

/*
Refused reports whether dimmi turned the request down for what it held,
which sending it again won't change.
*/
func (e *Error) Refused() bool {
	if e.Status == http.StatusUnauthorized || e.Status == http.StatusTooManyRequests {
		return false
	}

	return e.Status >= 400 && e.Status < 500
}

/*
Error says what dimmi answered.
*/
func (e *Error) Error() string {
	return fmt.Sprintf("dimmi answered %d %s: %s", e.Status, e.Code, e.Message)
}

/*
Tokens hands out a valid access token, refreshing it when asked.
*/
type Tokens interface {
	/* Token returns a valid access token. */
	Token(ctx context.Context) (string, error)

	/* Refresh trades the token for a new one and returns it. */
	Refresh(ctx context.Context) (string, error)
}

/*
Client is a Destination over HTTP.
*/
type Client struct {
	BaseURL string
	Tokens  Tokens
	HTTP    *http.Client
	Agent   string
}

/*
NewClient returns a client for the dimmi at base, such as
https://trydimmi.com, sending its requests through the transport.
*/
func NewClient(base string, tokens Tokens, transport http.RoundTripper, version string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(base, "/"),
		Tokens:  tokens,
		HTTP:    &http.Client{Transport: transport},
		Agent:   "dimmi-cli/" + version,
	}
}

func (c *Client) url(path string) string { return c.BaseURL + "/api/v1" + path }

/*
call is one request to the migrations API: its method and path, and a
body of the content type, if any.
*/
type call struct {
	method      string
	path        string
	body        io.Reader
	contentType string
}

/*
do sends a request and decodes a JSON answer into out. A 401 refreshes
the token once.
*/
func (c *Client) do(ctx context.Context, in call, out any) error {
	p := newPayload(in.body)

	for attempt := range 2 {
		req, err := c.request(ctx, in, p, attempt)
		if err != nil {
			return err
		}

		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}

		if res.StatusCode == http.StatusUnauthorized && attempt == 0 && p.resendable() {
			_ = res.Body.Close()

			if err := p.rewind(); err != nil {
				return err
			}

			continue
		}

		return decode(res, out)
	}

	return errors.New("dimmi refused the refreshed token: run `dimmi login` again")
}

/*
request builds an attempt's request: the first with the saved token,
the second with a refreshed one.
*/
func (c *Client) request(ctx context.Context, in call, p payload, attempt int) (*http.Request, error) {
	token, err := c.Tokens.Token(ctx)
	if attempt == 1 {
		token, err = c.Tokens.Refresh(ctx)
	}

	if err != nil {
		return nil, err
	}

	req, err := p.request(ctx, in.method, c.url(in.path))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.Agent)

	if in.contentType != "" {
		req.Header.Set("Content-Type", in.contentType)
	}

	return req, nil
}

/*
payload is a request body that can be sent more than once: JSON bytes
are kept, and an upload is rewound when it can seek.
*/
type payload struct {
	raw  []byte
	body io.Reader
}

/*
newPayload keeps the bytes of a bytes.Reader, and any other body as it
is.
*/
func newPayload(body io.Reader) payload {
	if r, ok := body.(*bytes.Reader); ok {
		raw := make([]byte, r.Len())
		_, _ = r.Read(raw)

		return payload{raw: raw}
	}

	return payload{body: body}
}

/*
request builds a request carrying the payload.
*/
func (p payload) request(ctx context.Context, method, target string) (*http.Request, error) {
	reqBody := p.body
	if p.raw != nil {
		reqBody = bytes.NewReader(p.raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, reqBody)
	if err != nil {
		return nil, err
	}

	if sz, ok := p.body.(sized); ok {
		req.Body = io.NopCloser(sz.r)
		req.ContentLength = sz.n

		if seeker, ok := sz.r.(io.Seeker); ok {
			req.GetBody = rewinder(seeker, sz.r)
		}
	}

	if seeker, ok := p.body.(io.ReadSeeker); ok && p.raw == nil {
		req.GetBody = rewinder(seeker, seeker)
	}

	return req, nil
}

/*
rewinder returns a GetBody that seeks back to the start and reads r.
*/
func rewinder(seeker io.Seeker, r io.Reader) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}

		return io.NopCloser(r), nil
	}
}

/*
seeker is the upload's reader when it can seek.
*/
func (p payload) seeker() (io.Seeker, bool) {
	sz, ok := p.body.(sized)
	if !ok {
		return nil, false
	}

	seeker, ok := sz.r.(io.Seeker)

	return seeker, ok
}

/*
resendable reports whether the payload can be sent again: no body, kept
bytes, or an upload that can seek.
*/
func (p payload) resendable() bool {
	_, rewinds := p.seeker()

	return p.raw != nil || p.body == nil || rewinds
}

/*
rewind seeks an upload back to its start.
*/
func (p payload) rewind() error {
	seeker, ok := p.seeker()
	if !ok {
		return nil
	}

	_, err := seeker.Seek(0, io.SeekStart)

	return err
}

/*
sized is a body with a known length, which an upload must declare.
*/
type sized struct {
	r io.Reader
	n int64
}

func (s sized) Read(p []byte) (int, error) { return s.r.Read(p) }

/*
decode reads an answer into out, or turns an error answer into an Error.
*/
func decode(res *http.Response, out any) error {
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	_ = res.Body.Close()

	if err != nil {
		return err
	}

	if res.StatusCode == http.StatusRequestEntityTooLarge {
		return engine.ErrTooLarge
	}

	if res.StatusCode >= 300 {
		e := &Error{Status: res.StatusCode}

		var body struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}

		if json.Unmarshal(raw, &body) == nil {
			e.Code, e.Message = body.Code, body.Message
		}

		if e.Code == "" {
			e.Code = "http_" + strconv.Itoa(res.StatusCode)
		}

		if e.Message == "" {
			e.Message = http.StatusText(res.StatusCode)
		}

		return e
	}

	if out == nil || len(raw) == 0 {
		return nil
	}

	return json.Unmarshal(raw, out)
}

/*
json sends in as JSON and decodes the answer into out.
*/
func (c *Client) json(ctx context.Context, to call, in, out any) error {
	var body io.Reader

	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}

		body = bytes.NewReader(raw)
	}

	to.body, to.contentType = body, "application/json"

	return c.do(ctx, to, out)
}

/*
migration is the migration resource.
*/
type migration struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Counts []struct {
		Kind        format.Kind `json:"kind"`
		Applied     int         `json:"applied"`
		Created     int         `json:"created"`
		Deferred    int         `json:"deferred"`
		Failed      int         `json:"failed"`
		Unsupported int         `json:"unsupported"`
	} `json:"counts"`
}

/*
engine reads dimmi's counts: applied records are held, created ones a
part of them; deferred ones wait for the records they name.
*/
func (m migration) engine() engine.Migration {
	out := engine.Migration{ID: m.ID, Status: m.Status, Counts: make(map[format.Kind]engine.Counts)}
	for _, c := range m.Counts {
		out.Counts[c.Kind] = engine.Counts{
			Created: c.Created, Updated: c.Applied - c.Created, Unchanged: c.Unsupported,
			Failed: c.Failed, Deferred: c.Deferred,
		}
	}

	return out
}

type migrationEnvelope struct {
	Data migration `json:"data"`
}

/*
Start starts a migration, or resumes the one dimmi holds for the same
source account.
*/
func (c *Client) Start(ctx context.Context, s engine.Start) (engine.Migration, error) {
	var out migrationEnvelope
	err := c.json(ctx, call{method: http.MethodPost, path: "/migrations"}, s, &out)

	return out.Data.engine(), err
}

/*
Get reads a migration's progress.
*/
func (c *Client) Get(ctx context.Context, id string) (engine.Migration, error) {
	var out migrationEnvelope
	err := c.json(ctx, call{method: http.MethodGet, path: "/migrations/" + url.PathEscape(id)}, nil, &out)

	return out.Data.engine(), err
}

/*
Upsert sends one batch of records of a kind.
*/
func (c *Client) Upsert(
	ctx context.Context, id string, kind format.Kind, records []format.Record,
) ([]engine.Result, error) {
	data := make([]any, len(records))
	for i, r := range records {
		data[i] = r.Data
	}

	in := struct {
		Kind    format.Kind `json:"kind"`
		Records []any       `json:"records"`
	}{kind, data}

	var out struct {
		Data struct {
			Results  []result `json:"results"`
			Resolved []result `json:"resolved"`
		} `json:"data"`
	}

	to := call{method: http.MethodPost, path: "/migrations/" + url.PathEscape(id) + "/records"}
	if err := c.json(ctx, to, in, &out); err != nil {
		return nil, err
	}

	results := make([]engine.Result, 0, len(out.Data.Results))
	for _, r := range out.Data.Results {
		results = append(results, r.engine())
	}

	return results, nil
}

/*
result is one record's outcome as dimmi reports it: applied (created or
updated), deferred until the records it names arrive, unsupported, or
failed with a problem.
*/
type result struct {
	SourceID string  `json:"source_id"`
	Status   string  `json:"status"`
	ID       *string `json:"id"`
	Error    *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field"`
	} `json:"error"`
	WaitingFor []engine.Ref `json:"waiting_for"`
}

func (r result) engine() engine.Result {
	out := engine.Result{SourceID: r.SourceID, WaitingFor: r.WaitingFor}
	if r.ID != nil {
		out.ID = *r.ID
	}

	switch r.Status {
	case "created":
		out.Status = engine.StatusCreated
	case "updated":
		out.Status = engine.StatusUpdated
	case "unchanged", "unsupported":
		out.Status = engine.StatusUnchanged
	case "deferred":
		out.Status = engine.StatusDeferred
	default:
		out.Status = engine.StatusFailed
	}

	if r.Error != nil {
		msg := r.Error.Message
		if r.Error.Field != "" {
			msg = r.Error.Field + ": " + msg
		}

		out.Error = &engine.RecordError{Code: r.Error.Code, Message: msg}
	}

	return out
}

/*
Upload sends one file's bytes and returns its file id.
*/
func (c *Client) Upload(ctx context.Context, id string, f engine.Upload) (string, error) {
	q := url.Values{"source_id": {f.SourceID}, "filename": {f.Name}}
	ct := f.ContentType

	if ct == "" {
		ct = "application/octet-stream"
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	err := c.do(ctx, call{
		method:      http.MethodPost,
		path:        "/migrations/" + url.PathEscape(id) + "/files?" + q.Encode(),
		body:        sized{f.Body, f.Size},
		contentType: ct,
	}, &out)

	return out.Data.ID, err
}

/*
Lookup reports which source ids of a kind dimmi holds.
*/
func (c *Client) Lookup(ctx context.Context, id string, kind format.Kind, ids []string) (map[string]bool, error) {
	in := struct {
		Kind      format.Kind `json:"kind"`
		SourceIDs []string    `json:"source_ids"`
	}{kind, ids}

	var out struct {
		Data []struct {
			SourceID string `json:"source_id"`
			Status   string `json:"status"`
		} `json:"data"`
	}

	to := call{method: http.MethodPost, path: "/migrations/" + url.PathEscape(id) + "/lookups"}
	if err := c.json(ctx, to, in, &out); err != nil {
		var e *Error
		if errors.As(err, &e) && (e.Status == http.StatusNotFound || e.Status == http.StatusMethodNotAllowed) {
			return nil, engine.ErrNoLookup
		}

		return nil, err
	}

	found := map[string]bool{}
	for _, r := range out.Data {
		found[r.SourceID] = r.Status == "applied" || r.Status == "unsupported"
	}

	return found, nil
}

/*
Rollback undoes everything the migration created.
*/
func (c *Client) Rollback(ctx context.Context, id string) (engine.Migration, error) {
	var out migrationEnvelope
	to := call{method: http.MethodPost, path: "/migrations/" + url.PathEscape(id) + "/rollbacks"}
	err := c.json(ctx, to, struct{}{}, &out)

	return out.Data.engine(), err
}
