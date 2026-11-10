/*
|--------------------------------------------------------------------------
| Slack Signatures
|--------------------------------------------------------------------------
|
| Slack signs every request it sends with the app's signing secret: an
| HMAC of the version, the request's timestamp and its raw body. A
| request is believed only when the signature matches, compared in
| constant time, and its timestamp is within five minutes of now, so a
| recorded request replayed later is refused. Nothing in the body is
| read before this check passes.
|
*/

package slack

import (
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
signatureWindow is how far a request's timestamp may be from now.
*/
const signatureWindow = 300 * time.Second

/*
Errors of a signature check. ErrSignatureStale means the timestamp is
outside the window; ErrSignatureInvalid that the signature is missing
or does not match.
*/
var (
	ErrSignatureStale   = errors.New("slack: request timestamp outside the window")
	ErrSignatureInvalid = errors.New("slack: request signature invalid")
)

/*
SignedRequest is what a signature covers: the request's timestamp and
signature headers and its raw body.
*/
type SignedRequest struct {
	Timestamp string
	Signature string
	Body      []byte
}

/*
VerifySignature checks Slack's signature of the request.
*/
func VerifySignature(secret string, req SignedRequest, now time.Time) error {
	timestamp, signature, body := req.Timestamp, req.Signature, req.Body

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrSignatureStale
	}

	if age := now.Sub(time.Unix(seconds, 0)); age > signatureWindow || age < -signatureWindow {
		return ErrSignatureStale
	}

	sent, ok := strings.CutPrefix(signature, "v0=")
	if secret == "" || !ok {
		return ErrSignatureInvalid
	}

	got, err := hex.DecodeString(sent)
	if err != nil {
		return ErrSignatureInvalid
	}

	if !hashing.Equal(got, Sign(secret, timestamp, body)) {
		return ErrSignatureInvalid
	}

	return nil
}

/*
Sign returns Slack's signature of the body at the timestamp, without
its version prefix.
*/
func Sign(secret, timestamp string, body []byte) []byte {
	message := make([]byte, 0, len(body)+len(timestamp)+4)
	message = append(message, "v0:"+timestamp+":"...)
	message = append(message, body...)

	return hashing.HMAC([]byte(secret), message)
}

/*
SignatureHeader returns the signature header Slack sends for the body,
for tests and tools that sign as Slack would.
*/
func SignatureHeader(secret, timestamp string, body []byte) string {
	return "v0=" + hex.EncodeToString(Sign(secret, timestamp, body))
}
