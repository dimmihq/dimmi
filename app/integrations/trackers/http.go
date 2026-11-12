/*
|--------------------------------------------------------------------------
| Tracker Requests
|--------------------------------------------------------------------------
|
| Every tool is reached the same way: JSON in and out, the token as the
| tool wants it, a refused token as ErrAuth, a missing record as
| ErrNotFound and any other failure as the tool's. The description
| written on a new issue and the impact line kept current on it read
| the same in every tool.
|
*/

package trackers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
client sends JSON requests with the headers a tool needs.
*/
type client struct {
	http   *httpclient.Client
	header map[string]string
}

/*
request is one call: the method, the address, the body to encode, nil
for none, and where to decode the answer, nil to drop it.
*/
type request struct {
	method string
	url    string
	body   any
	out    any
}

/*
do sends the request with the header values and decodes the answer.
*/
func (c client) do(ctx context.Context, header map[string]string, r request) error {
	req := c.http.New().Accept("application/json")
	for k, v := range c.header {
		req.WithHeader(k, v)
	}

	for k, v := range header {
		req.WithHeader(k, v)
	}

	var (
		resp *httpclient.Response
		err  error
	)

	switch r.method {
	case fhttp.MethodGet:
		resp, err = req.Get(ctx, r.url)
	default:
		resp, err = c.send(ctx, req, r)
	}

	if err != nil {
		return fmt.Errorf("%s %s: %w", r.method, r.url, err)
	}

	return answer(resp, r)
}

/*
send encodes the body and sends a request that carries one.
*/
func (c client) send(ctx context.Context, req *httpclient.Request, r request) (*httpclient.Response, error) {
	var body []byte

	if r.body != nil {
		encoded, err := json.Marshal(r.body)
		if err != nil {
			return nil, err
		}

		body = encoded
	}

	req.WithHeader("Content-Type", "application/json")

	switch r.method {
	case fhttp.MethodPut:
		return req.Put(ctx, r.url, body)
	case fhttp.MethodPatch:
		return req.Patch(ctx, r.url, body)
	default:
		return req.Post(ctx, r.url, body)
	}
}

/*
answer reads the tool's answer into the request's out value.
*/
func answer(resp *httpclient.Response, r request) error {
	switch {
	case resp.Status == fhttp.StatusUnauthorized || resp.Status == fhttp.StatusForbidden:
		return fmt.Errorf("%s %s: %w", r.method, r.url, ErrAuth)
	case resp.Status == fhttp.StatusNotFound:
		return fmt.Errorf("%s %s: %w", r.method, r.url, ErrNotFound)
	case resp.Status < 200 || resp.Status > 299:
		return fmt.Errorf("%s %s: %w: answered %d", r.method, r.url, ErrProvider, resp.Status)
	}

	if r.out == nil || len(resp.Body) == 0 {
		return nil
	}

	return resp.JSON(r.out)
}

/*
Description is the text a new issue starts with: what the customers
said it is, then who it reaches and the link back to the tracker.
*/
func Description(text string, impact Impact) string {
	var b strings.Builder

	if text = strings.TrimSpace(text); text != "" {
		b.WriteString(text)
		b.WriteString("\n\n")
	}

	b.WriteString(ImpactLine(impact))

	if len(impact.CompanyNames) > 0 {
		b.WriteString("\n\nCompanies: ")
		b.WriteString(strings.Join(impact.CompanyNames, ", "))
	}

	if impact.Link != "" {
		b.WriteString("\n\nTracked in dimmi: ")
		b.WriteString(impact.Link)
	}

	return b.String()
}

/*
ImpactLine says in one line who an issue reaches.
*/
func ImpactLine(impact Impact) string {
	line := fmt.Sprintf("Customer impact: %s, %s, %s",
		plural(impact.Customers, "customer"), plural(impact.Companies, "company"),
		plural(impact.Conversations, "conversation"))

	if impact.Revenue > 0 {
		line += ", " + Money(impact.Revenue) + " monthly revenue at stake"
	}

	return line + "."
}

/*
Money writes a monthly amount rounded to whole units.
*/
func Money(amount float64) string {
	return "$" + strconv.FormatInt(int64(math.Round(amount)), 10)
}

/*
plural writes a count with its noun.
*/
func plural(n int64, noun string) string {
	if n == 1 {
		return "1 " + noun
	}

	if stem, ok := strings.CutSuffix(noun, "y"); ok {
		return strconv.FormatInt(n, 10) + " " + stem + "ies"
	}

	return strconv.FormatInt(n, 10) + " " + noun + "s"
}

/*
formatInt writes a number as decimal text.
*/
func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

/*
remarshal reads a decoded JSON value into a typed one.
*/
func remarshal(in, out any) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}

	return decode(raw, out)
}

/*
str reads a JSON string, empty for anything else.
*/
func str(v any) string {
	s, _ := v.(string)

	return s
}

/*
asMap reads a JSON object, empty for anything else.
*/
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}

/*
get is a GET of the path decoded into out.
*/
func get(path string, out any) request {
	return request{method: fhttp.MethodGet, url: path, out: out}
}

/*
post is a POST of the body to the path decoded into out.
*/
func post(path string, body, out any) request {
	return request{method: fhttp.MethodPost, url: path, body: body, out: out}
}
