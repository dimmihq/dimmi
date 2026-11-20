/*
|--------------------------------------------------------------------------
| Graph API
|--------------------------------------------------------------------------
|
| Every call the channel makes to Meta goes through here, on the
| install's HTTP client and at one pinned Graph version, so requests pass
| the same address checks and tests answer them from fixtures. A call
| Meta refuses comes back as a GraphError with Meta's code, which the
| driver maps onto the channel contract. Media is downloaded only from
| Meta's own hosts, with the token, so a crafted address in a delivery
| can never make the server fetch anything else.
|
*/

package whatsapp

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
	ErrMediaHost        = errors.New("whatsapp: media address is not on a Meta host")
	ErrMediaUnavailable = errors.New("whatsapp: media unavailable")
)

/*
mediaHosts are the only hosts media is downloaded from.
*/
func mediaHosts() []string {
	return []string{"lookaside.fbsbx.com", "fbcdn.net", "whatsapp.net", "facebook.com"}
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
Upload sends one file to the number's media store and returns its id.
The file part's field is filled in here.
*/
func (g *Graph) Upload(ctx context.Context, phoneNumberID string, file httpclient.FilePart) (string, error) {
	file.Field = "file"
	fields := map[string]string{"messaging_product": "whatsapp", "type": file.ContentType}

	resp, err := g.http.New().WithToken(g.token).
		PostMultipart(ctx, g.address(phoneNumberID+"/media", nil), fields, file)
	if err != nil {
		return "", fmt.Errorf("graph upload media: %w", err)
	}

	var out struct {
		ID string `json:"id"`
	}

	if err := decode(resp, &out); err != nil {
		return "", err
	}

	return out.ID, nil
}

/*
Media is a file a customer sent, as Meta describes it.
*/
type Media struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"file_size"`
	SHA256   string `json:"sha256"`
}

/*
MediaInfo returns where Meta serves the media, for a few minutes.
*/
func (g *Graph) MediaInfo(ctx context.Context, mediaID string) (Media, error) {
	var out Media

	err := g.Get(ctx, mediaID, nil, &out)

	return out, err
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
		return nil, fmt.Errorf("download whatsapp media: %w", err)
	}

	if resp.Status != fhttp.StatusOK {
		return nil, fmt.Errorf("download whatsapp media: %w: status %d", ErrMediaUnavailable, resp.Status)
	}

	return io.NopCloser(bytes.NewReader(resp.Body)), nil
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
