/*
|--------------------------------------------------------------------------
| Graph API
|--------------------------------------------------------------------------
|
| Every call a Meta channel makes, WhatsApp, Messenger or Instagram, goes
| through here, on the install's HTTP client and at one pinned Graph
| version, so requests pass the same address checks and tests answer
| them from fixtures. A call Meta refuses comes back as a GraphError with
| Meta's code, which each driver maps onto the channel contract. Media is
| downloaded only from Meta's own hosts, so a crafted address in a
| delivery can never make the server fetch anything else.
|
*/

package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
Errors of the Graph client. ErrMediaHost refuses a media address off
Meta's hosts, and ErrMediaUnavailable a file Meta no longer serves.
*/
var (
	ErrMediaHost        = errors.New("meta: media address is not on a Meta host")
	ErrMediaUnavailable = errors.New("meta: media unavailable")
)

/*
mediaHosts are the only hosts media is downloaded from.
*/
func mediaHosts() []string {
	return []string{"fbsbx.com", "fbcdn.net", "whatsapp.net", "facebook.com", "cdninstagram.com"}
}

/*
GraphError is a call Meta refused: its code and subcode, its own words,
and the HTTP status it came with.
*/
type GraphError struct {
	Status  int    `json:"-"`
	Code    int    `json:"code"`
	Subcode int    `json:"error_subcode"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Details string `json:"-"`
	TraceID string `json:"fbtrace_id"`
	// UserTitle and UserMessage are Meta's own words for a person.
	UserTitle   string `json:"error_user_title"`
	UserMessage string `json:"error_user_msg"`
}

/*
Error returns Meta's code and words.
*/
func (e *GraphError) Error() string {
	return fmt.Sprintf("graph error %d: %s", e.Code, e.Message)
}

/*
Graph calls Meta's Graph API with one token.
*/
type Graph struct {
	http    *httpclient.Client
	base    string
	version string
	token   string
}

/*
NewGraph returns the Graph API at the base URL and version, calling with
the token.
*/
func NewGraph(http *httpclient.Client, base, version, token string) *Graph {
	return &Graph{http: http, base: strings.TrimRight(base, "/"), version: version, token: token}
}

/*
address is the full address of a Graph path, with its query.
*/
func (g *Graph) address(path string, query url.Values) string {
	out := g.base + "/" + g.version + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		out += "?" + query.Encode()
	}

	return out
}

/*
Get reads the path into out.
*/
func (g *Graph) Get(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := g.http.New().WithToken(g.token).Get(ctx, g.address(path, query))
	if err != nil {
		return fmt.Errorf("graph get %s: %w", path, err)
	}

	return decode(resp, out)
}

/*
Post sends the body as JSON to the path and reads the answer into out,
which may be nil.
*/
func (g *Graph) Post(ctx context.Context, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode graph body: %w", err)
	}

	resp, err := g.http.New().WithToken(g.token).WithHeader("Content-Type", "application/json").
		Post(ctx, g.address(path, nil), raw)
	if err != nil {
		return fmt.Errorf("graph post %s: %w", path, err)
	}

	return decode(resp, out)
}

/*
Delete removes what the path names.
*/
func (g *Graph) Delete(ctx context.Context, path string) error {
	resp, err := g.http.New().WithToken(g.token).Delete(ctx, g.address(path, nil))
	if err != nil {
		return fmt.Errorf("graph delete %s: %w", path, err)
	}

	return decode(resp, nil)
}

/*
Form is what a multipart call carries: its plain fields and one file.
*/
type Form struct {
	Fields map[string]string
	File   httpclient.FilePart
}

/*
Multipart sends the form to the path and reads the answer into out,
which may be nil.
*/
func (g *Graph) Multipart(ctx context.Context, path string, form Form, out any) error {
	resp, err := g.http.New().WithToken(g.token).PostMultipart(ctx, g.address(path, nil), form.Fields, form.File)
	if err != nil {
		return fmt.Errorf("graph upload %s: %w", path, err)
	}

	return decode(resp, out)
}

/*
Download reads the media at the address Meta gave, at most limit bytes.
The caller closes what it returns.
*/
func (g *Graph) Download(ctx context.Context, address string, limit int64) (io.ReadCloser, error) {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || !mediaHost(u.Hostname()) {
		return nil, ErrMediaHost
	}

	resp, err := g.http.New().WithToken(g.token).MaxBody(limit).Get(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("download media: %w", err)
	}

	if resp.Status != fhttp.StatusOK {
		return nil, fmt.Errorf("download media: %w: status %d", ErrMediaUnavailable, resp.Status)
	}

	return io.NopCloser(bytes.NewReader(resp.Body)), nil
}

/*
Fetched is a file read from one of Meta's hosts: its bytes and the type
the host says it is.
*/
type Fetched struct {
	Body        []byte
	ContentType string
}

/*
Fetch reads the file at an address Meta put in a delivery, at most limit
bytes, without the token: such addresses carry their own signature.
*/
func (g *Graph) Fetch(ctx context.Context, address string, limit int64) (Fetched, error) {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || !mediaHost(u.Hostname()) {
		return Fetched{}, ErrMediaHost
	}

	resp, err := g.http.New().MaxBody(limit).Get(ctx, address)
	if err != nil {
		return Fetched{}, fmt.Errorf("fetch media: %w", err)
	}

	if resp.Status != fhttp.StatusOK {
		return Fetched{}, fmt.Errorf("fetch media: %w: status %d", ErrMediaUnavailable, resp.Status)
	}

	return Fetched{Body: resp.Body, ContentType: resp.Header.Get("Content-Type")}, nil
}

/*
mediaHost reports whether media is downloaded from the host.
*/
func mediaHost(host string) bool {
	for _, allowed := range mediaHosts() {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}

	return false
}

/*
decode reads a Graph answer into out, or Meta's refusal as a GraphError.
*/
func decode(resp *httpclient.Response, out any) error {
	if resp.Status >= 200 && resp.Status < 300 {
		if out == nil || len(resp.Body) == 0 {
			return nil
		}

		if err := json.Unmarshal(resp.Body, out); err != nil {
			return fmt.Errorf("decode graph answer: %w", err)
		}

		return nil
	}

	var envelope struct {
		Error struct {
			GraphError

			Data struct {
				Details string `json:"details"`
			} `json:"error_data"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp.Body, &envelope); err != nil || envelope.Error.Code == 0 {
		return &GraphError{Status: resp.Status, Message: fmt.Sprintf("status %d", resp.Status)}
	}

	refused := envelope.Error.GraphError
	refused.Status, refused.Details = resp.Status, envelope.Error.Data.Details

	return &refused
}
