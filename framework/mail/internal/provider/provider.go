/*
|--------------------------------------------------------------------------
| Provider API Calls
|--------------------------------------------------------------------------
|
| The mail APIs answer alike: success is a 2xx status, a 429 asks to
| slow down, a refused key passes once the operator fixes it, a client
| error the HTTP client calls permanent means this message will never be
| accepted, and anything else, such as a server error, passes once the
| provider recovers. Each driver reads its own success body; the failure
| kinds are decided once, here.
|
| Error text keeps the start of the provider's answer, which names the
| actual problem, but never the request: it holds the API key.
|
*/

package provider

import (
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
)

/*
maxDetail caps how much of a failure body an error keeps.
*/
const maxDetail = 300

/*
Check returns nil for a 2xx response and the mail error kind for
anything else, including a request that never got an answer.
*/
func Check(name string, resp *httpclient.Response, err error) error {
	if err != nil {
		return fmt.Errorf("%s send: %w: %w", name, mail.ErrTemporary, err)
	}

	if resp.Status >= 200 && resp.Status < 300 {
		return nil
	}

	detail := fmt.Sprintf("%s send: status %d: %s", name, resp.Status, truncate(string(resp.Body)))

	switch {
	case resp.Status == http.StatusTooManyRequests:
		wait := httpclient.RetryAfter(resp.Header.Get("Retry-After"), clock.System().Now())

		return fmt.Errorf("%s: %w", detail, &mail.RateLimitedError{RetryAfter: wait})
	case resp.Status == http.StatusUnauthorized, resp.Status == http.StatusForbidden:
		return fmt.Errorf("%s: %w", detail, mail.ErrTemporary)
	case httpclient.Permanent(resp.Status):
		return fmt.Errorf("%s: %w", detail, mail.ErrPermanent)
	default:
		return fmt.Errorf("%s: %w", detail, mail.ErrTemporary)
	}
}

/*
Emails returns the bare email addresses.
*/
func Emails(list []mail.Address) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.Email)
	}

	return out
}

/*
Formatted returns the addresses as header text, each with its name.
*/
func Formatted(list []mail.Address) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, a.String())
	}

	return out
}

func truncate(s string) string {
	if len(s) <= maxDetail {
		return s
	}

	cut := maxDetail
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}

	return s[:cut] + "..."
}
