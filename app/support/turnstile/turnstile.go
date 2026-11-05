/*
|--------------------------------------------------------------------------
| Turnstile
|--------------------------------------------------------------------------
|
| The browser solves a Turnstile challenge and sends its answer with the
| form; the server asks Cloudflare whether the answer is genuine, fresh
| and meant for this site. Only a clear yes passes. A provider that does
| not answer, or answers with something unreadable, is told apart from a
| wrong answer, so the caller can decide whether to let people through
| while it is down.
|
*/

package turnstile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
Check failures.

ErrRejected means the provider judged the answer wrong, used or missing.
ErrUnavailable means the provider could not be asked or gave no usable
answer.
*/
var (
	ErrRejected    = errors.New("turnstile: answer rejected")
	ErrUnavailable = errors.New("turnstile: provider unavailable")
)

/*
Client checks answers with the provider at VerifyURL, using the secret
key, giving up after Timeout.
*/
type Client struct {
	HTTP      *httpclient.Client
	Secret    string
	VerifyURL string
	Timeout   time.Duration
}

/*
verdict is the provider's answer.
*/
type verdict struct {
	Success bool `json:"success"`
}

/*
Verify asks the provider about the answer the person at the address
gave.

Errors: [ErrRejected], or [ErrUnavailable] wrapping the cause.
*/
func (c *Client) Verify(ctx context.Context, answer, ip string) error {
	if answer == "" {
		return ErrRejected
	}

	resp, err := c.HTTP.New().Timeout(c.Timeout).Post(ctx, c.VerifyURL, map[string]string{
		"secret": c.Secret, "response": answer, "remoteip": ip,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	if resp.Status >= 500 {
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.Status)
	}

	var v verdict
	if err := resp.JSON(&v); err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	if !v.Success {
		return ErrRejected
	}

	return nil
}
