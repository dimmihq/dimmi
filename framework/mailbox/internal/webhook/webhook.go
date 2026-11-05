/*
|--------------------------------------------------------------------------
| Webhook Reading
|--------------------------------------------------------------------------
|
| Providers post mail in several shapes, but each source reads them the
| same careful way: never past the limit it declared, and never trusting
| the raw message's headers beyond the few a provider writes itself.
|
*/

package webhook

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
idLength is how many hex characters of a raw message's digest serve as
the id of mail whose provider gives none.
*/
const idLength = 32

/*
Read returns the delivery's body, refusing one over the limit.
*/
func Read(d mailbox.Delivery, limit int64) ([]byte, error) {
	if d.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(d.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read webhook body: %w", err)
	}

	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: webhook body over %d bytes", mailbox.ErrTooLarge, limit)
	}

	return body, nil
}

/*
Form reads the fields of a form body, url-encoded or multipart, the two
ways providers post forms.
*/
func Form(contentType string, body []byte) (url.Values, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(mediaType, "multipart/") {
		return multipartFields(body, params["boundary"])
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("%w: form body: %w", mailbox.ErrRejected, err)
	}

	return values, nil
}

func multipartFields(body []byte, boundary string) (url.Values, error) {
	values := url.Values{}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)

	for {
		part, err := reader.NextRawPart()
		if errors.Is(err, io.EOF) {
			return values, nil
		}

		if err != nil {
			return nil, fmt.Errorf("%w: multipart body: %w", mailbox.ErrRejected, err)
		}

		content, err := io.ReadAll(part)
		if err != nil {
			return nil, fmt.Errorf("%w: multipart part: %w", mailbox.ErrRejected, err)
		}

		values.Add(part.FormName(), string(content))
	}
}

/*
RawID returns the id of mail whose provider gives none stable across
retries: the digest of the message, so a retry is recognized and the
same mail to two tenants still gets a row each.
*/
func RawID(raw []byte) string {
	sum := sha256.Sum256(raw)

	return hex.EncodeToString(sum[:])[:idLength]
}

/*
Opener returns a function opening the bytes as a raw message.
*/
func Opener(raw []byte) func(context.Context) (io.ReadCloser, error) {
	return func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(raw)), nil
	}
}

/*
Headers returns the raw message's top-level headers in their order,
empty for a message too broken to read: such mail simply has no verdict.
*/
func Headers(raw []byte) mail.Header {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return mail.Header{}
	}

	return msg.Header
}

/*
First returns the first occurrence of a header. A receiving provider
writes its own headers on top, so anything below may be the sender's.
*/
func First(headers mail.Header, name string) string {
	values := headers[textproto.CanonicalMIMEHeaderKey(name)]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

/*
Verdict reads a provider's word for a check. Words mean the same across
providers; anything else is no verdict.
*/
func Verdict(word string) mailbox.Verdict {
	fields := strings.Fields(strings.ToLower(word))
	if len(fields) == 0 {
		return ""
	}

	switch strings.Trim(fields[0], ";,") {
	case "pass":
		return mailbox.VerdictPass
	case "fail", "softfail", "hardfail":
		return mailbox.VerdictFail
	case "none", "neutral":
		return mailbox.VerdictNone
	case "temperror", "permerror", "unknown":
		return mailbox.VerdictUnknown
	default:
		return ""
	}
}

/*
SpamVerdict reads a provider's yes or no on whether mail is spam: a yes
fails the check, a no passes it.
*/
func SpamVerdict(word string) mailbox.Verdict {
	fields := strings.FieldsFunc(strings.ToLower(word), func(r rune) bool { return r == ' ' || r == ',' })
	if len(fields) == 0 {
		return ""
	}

	switch fields[0] {
	case "yes":
		return mailbox.VerdictFail
	case "no":
		return mailbox.VerdictPass
	default:
		return ""
	}
}

/*
Recipients returns the addresses once each, in the order given, dropping
blanks.
*/
func Recipients(lists ...[]string) []string {
	merged := slices.Concat(lists...)
	out := make([]string, 0, len(merged))

	for _, address := range merged {
		address = strings.TrimSpace(address)
		if address != "" && !slices.Contains(out, address) {
			out = append(out, address)
		}
	}

	return out
}

/*
Sender returns the envelope sender in a Return-Path value, empty for the
null sender bounces use.
*/
func Sender(returnPath string) string {
	return strings.Trim(strings.TrimSpace(returnPath), "<>")
}
