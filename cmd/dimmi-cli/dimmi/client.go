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

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/engine"
	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
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

func (e *Error) Error() string {
	return fmt.Sprintf("dimmi answered %d %s: %s", e.Status, e.Code, e.Message)
}

/*
Tokens hands out a valid access token, refreshing it when asked.
*/
type Tokens interface {
	Token(ctx context.Context) (string, error)
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
do sends a request and decodes a JSON answer into out. A 401 refreshes
the token once.
*/
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, out any) error {
	var raw []byte

	if body != nil {
		if r, ok := body.(*bytes.Reader); ok {
			raw = make([]byte, r.Len())
			_, _ = r.Read(raw)
			body = nil
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.Tokens.Token(ctx)
		if attempt == 1 {
			token, err = c.Tokens.Refresh(ctx)
		}

		if err != nil {
			return err
		}

		var reqBody io.Reader = body
		if raw != nil {
			reqBody = bytes.NewReader(raw)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.url(path), reqBody)
		if err != nil {
			return err
		}

		if sz, ok := body.(sized); ok {
			req.Body = io.NopCloser(sz.r)
			req.ContentLength = sz.n

			if seeker, ok := sz.r.(io.Seeker); ok {
				req.GetBody = func() (io.ReadCloser, error) {
					if _, err := seeker.Seek(0, io.SeekStart); err != nil {
						return nil, err
					}

					return io.NopCloser(sz.r), nil
				}
			}
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.Agent)

		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		if seeker, ok := body.(io.ReadSeeker); ok && raw == nil {
			req.GetBody = func() (io.ReadCloser, error) {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return nil, err
				}

				return io.NopCloser(seeker), nil
			}
		}

		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}

		if res.StatusCode == http.StatusUnauthorized && attempt == 0 {
			var seeker io.Seeker

			rewinds := false
			if sz, ok := body.(sized); ok {
				seeker, rewinds = sz.r.(io.Seeker)
			}
			if raw != nil || body == nil || rewinds {
				_ = res.Body.Close()

				if rewinds {
					if _, err := seeker.Seek(0, io.SeekStart); err != nil {
						return err
					}
				}

				continue
			}
		}

		return decode(res, out)
	}

	return errors.New("dimmi refused the refreshed token: run `dimmi login` again")
}

/*
sized is a body with a known length, which an upload must declare.
*/
type sized struct {
	r io.Reader
	n int64
}

func (s sized) Read(p []byte) (int, error) { return s.r.Read(p) }

func decode(res *http.Response, out any) error {
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
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

func (c *Client) json(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader

	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}

		body = bytes.NewReader(raw)
	}

	return c.do(ctx, method, path, body, "application/json", out)
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
	out := engine.Migration{ID: m.ID, Status: m.Status, Counts: map[format.Kind]engine.Counts{}}
	for _, c := range m.Counts {
		out.Counts[c.Kind] = engine.Counts{
			Created: c.Created, Updated: c.Applied - c.Created, Unchanged: c.Unsupported, Failed: c.Failed, Deferred: c.Deferred,
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
	err := c.json(ctx, http.MethodPost, "/migrations", s, &out)

	return out.Data.engine(), err
}

/*
Get reads a migration's progress.
*/
func (c *Client) Get(ctx context.Context, id string) (engine.Migration, error) {
	var out migrationEnvelope
	err := c.json(ctx, http.MethodGet, "/migrations/"+url.PathEscape(id), nil, &out)

	return out.Data.engine(), err
}

/*
Upsert sends one batch of records of a kind.
*/
func (c *Client) Upsert(ctx context.Context, id string, kind format.Kind, records []format.Record) ([]engine.Result, error) {
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

	if err := c.json(ctx, http.MethodPost, "/migrations/"+url.PathEscape(id)+"/records", in, &out); err != nil {
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
}

func (r result) engine() engine.Result {
	out := engine.Result{SourceID: r.SourceID}
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

	err := c.do(ctx, http.MethodPost, "/migrations/"+url.PathEscape(id)+"/files?"+q.Encode(), sized{f.Body, f.Size}, ct, &out)

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
			ID       string `json:"id"`
		} `json:"data"`
	}

	if err := c.json(ctx, http.MethodPost, "/migrations/"+url.PathEscape(id)+"/lookups", in, &out); err != nil {
		var e *Error
		if errors.As(err, &e) && (e.Status == http.StatusNotFound || e.Status == http.StatusMethodNotAllowed) {
			return nil, engine.ErrNoLookup
		}

		return nil, err
	}

	found := map[string]bool{}
	for _, r := range out.Data {
		found[r.SourceID] = r.ID != ""
	}

	return found, nil
}

/*
Rollback undoes everything the migration created.
*/
func (c *Client) Rollback(ctx context.Context, id string) (engine.Migration, error) {
	var out migrationEnvelope
	err := c.json(ctx, http.MethodPost, "/migrations/"+url.PathEscape(id)+"/rollbacks", struct{}{}, &out)

	return out.Data.engine(), err
}
