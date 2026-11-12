/*
|--------------------------------------------------------------------------
| Reading Another Tool
|--------------------------------------------------------------------------
|
| Both tools answer JSON over HTTPS with a token. A refusal stops the
| import with what the tool said, so a revoked token or a missing scope
| reads plainly on the import instead of as a half-empty workspace.
|
*/

package ticketimports

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
ErrSourceRefused is the other tool refusing a read.
*/
var ErrSourceRefused = fault.New(fault.Invalid, "ticket_import_source_refused",
	"The other tool refused to share its tickets. Check the token and its permissions.")

/*
readTimeout bounds one read of the other tool, and maxBody one answer.
*/
const (
	readTimeout = 60 * time.Second
	maxBody     = 32 << 20
)

/*
reader reads JSON from a tool with fixed headers.
*/
type reader struct {
	http    *httpclient.Client
	headers map[string]string
}

/*
request starts a request carrying the tool's headers.
*/
func (r reader) request() *httpclient.Request {
	req := r.http.New().Timeout(readTimeout).MaxBody(maxBody).Accept("application/json")
	for k, v := range r.headers {
		req = req.WithHeader(k, v)
	}

	return req
}

/*
get reads the address into out.
*/
func (r reader) get(ctx context.Context, address string, out any) error {
	resp, err := r.request().Get(ctx, address)
	if err != nil {
		return fmt.Errorf("read %s: %w", address, err)
	}

	return decode(resp, out)
}

/*
post sends the body to the address and reads the answer into out.
*/
func (r reader) post(ctx context.Context, address string, body, out any) error {
	resp, err := r.request().WithHeader("Content-Type", "application/json").Post(ctx, address, body)
	if err != nil {
		return fmt.Errorf("read %s: %w", address, err)
	}

	return decode(resp, out)
}

/*
decode reads a successful answer, refusing any other.
*/
func decode(resp *httpclient.Response, out any) error {
	if resp.Status < 200 || resp.Status > 299 {
		return ErrSourceRefused.With("status", resp.Status)
	}

	if err := resp.JSON(out); err != nil {
		return fmt.Errorf("decode answer: %w", err)
	}

	return nil
}
