/*
|--------------------------------------------------------------------------
| Standard Webhooks
|--------------------------------------------------------------------------
|
| Outgoing webhooks are signed per the Standard Webhooks specification,
| so receivers verify them with an official library in their language
| instead of code of our own. The signature covers the message id, the
| timestamp and the raw body; the timestamp bounds how long a captured
| request can be replayed.
|
| During a secret rotation a request carries one signature per secret,
| space-separated in a single header, and the receiver accepts any of
| them. Verify likewise takes every secret currently valid.
|
*/

package webhooks

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
The headers carrying the message id, its timestamp in Unix seconds and
its signatures.
*/
const (
	HeaderID        = standardwebhooks.HeaderWebhookID
	HeaderTimestamp = standardwebhooks.HeaderWebhookTimestamp
	HeaderSignature = standardwebhooks.HeaderWebhookSignature
)

/*
DefaultTolerance is how far a timestamp may be from now, either way,
when Options set none; it is the standard's own default.
*/
const DefaultTolerance = 5 * time.Minute

/*
Errors from Verify.

ErrMissingHeaders means one of the three headers is absent or the
timestamp is not a number. ErrTooOld and ErrTooNew mean the timestamp
lies outside the tolerance. ErrNoMatchingSignature means no signature
was made by any of the given secrets.
*/
var (
	ErrMissingHeaders      = errors.New("webhooks: missing or invalid headers")
	ErrTooOld              = errors.New("webhooks: timestamp too old")
	ErrTooNew              = errors.New("webhooks: timestamp too new")
	ErrNoMatchingSignature = errors.New("webhooks: no matching signature")
)

/*
Options configure Verify. Tolerance defaults to DefaultTolerance and
Clock to the system clock.
*/
type Options struct {
	Tolerance time.Duration
	Clock     clock.Clock
}

/*
Sign returns the "v1,<signature>" entry for one secret.
*/
func Sign(secret Secret, id string, timestamp time.Time, body []byte) string {
	return SignAll([]Secret{secret}, id, timestamp, body)
}

/*
SignAll returns the signature header value for several secrets: one
"v1,<signature>" entry per secret, space-separated, which is what a
request carries while a secret is being rotated. The library's signer
cannot fail, so its error is not passed on.
*/
func SignAll(secrets []Secret, id string, timestamp time.Time, body []byte) string {
	entries := make([]string, 0, len(secrets))

	for _, secret := range secrets {
		hook, _ := standardwebhooks.NewWebhookRaw(secret.key)
		signature, _ := hook.Sign(id, timestamp, body)
		entries = append(entries, signature)
	}

	return strings.Join(entries, " ")
}

/*
Headers returns the three Standard Webhooks headers for a request signed
with every given secret.
*/
func Headers(secrets []Secret, id string, timestamp time.Time, body []byte) http.Header {
	headers := http.Header{}
	headers.Set(HeaderID, id)
	headers.Set(HeaderTimestamp, strconv.FormatInt(timestamp.Unix(), 10))
	headers.Set(HeaderSignature, SignAll(secrets, id, timestamp, body))

	return headers
}

/*
Verify checks a received request: the headers must be present, the
timestamp within the tolerance, and one signature made by one of the
secrets over the raw body.
*/
func Verify(secrets []Secret, headers http.Header, body []byte, opts Options) error {
	if err := checkTimestamp(headers, opts); err != nil {
		return err
	}

	for _, secret := range secrets {
		hook, _ := standardwebhooks.NewWebhookRaw(secret.key)

		if hook.VerifyIgnoringTimestamp(body, headers) == nil {
			return nil
		}
	}

	return ErrNoMatchingSignature
}

func checkTimestamp(headers http.Header, opts Options) error {
	if headers.Get(HeaderID) == "" || headers.Get(HeaderSignature) == "" {
		return ErrMissingHeaders
	}

	seconds, err := strconv.ParseInt(headers.Get(HeaderTimestamp), 10, 64)
	if err != nil {
		return ErrMissingHeaders
	}

	tolerance := opts.Tolerance
	if tolerance <= 0 {
		tolerance = DefaultTolerance
	}

	source := opts.Clock
	if source == nil {
		source = clock.System()
	}

	now, sent := source.Now(), time.Unix(seconds, 0)
	if now.Sub(sent) > tolerance {
		return ErrTooOld
	}

	if sent.Sub(now) > tolerance {
		return ErrTooNew
	}

	return nil
}
