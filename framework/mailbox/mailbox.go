/*
|--------------------------------------------------------------------------
| Inbound Mail
|--------------------------------------------------------------------------
|
| Mail reaches the application only through a provider that receives it,
| never through a mail server of its own. Each source turns what its
| provider delivers into the same envelope: who sent it and to whom, its
| size, the provider's own verdicts, and a way to open the raw message.
|
| The envelope is small and quick to produce, so a webhook can answer
| at once; the raw message is read later, by a job, from wherever the
| provider stored it.
|
*/

package mailbox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

/*
Errors a source returns.

ErrRejected refuses a delivery that cannot be trusted or understood: a
bad signature, an unknown topic, a malformed notification.
ErrTooLarge refuses a message over the source's size limit.
ErrUnsubscribed reports that the provider ended the subscription, so
no more mail will arrive until an operator restores it.
ErrNotReceiver means the named source does not take deliveries from a
webhook. ErrMissingOption means a source is enabled without a setting
it cannot work without.
*/
var (
	ErrRejected      = errors.New("mailbox: delivery rejected")
	ErrTooLarge      = errors.New("mailbox: message too large")
	ErrUnsubscribed  = errors.New("mailbox: subscription ended")
	ErrNotReceiver   = errors.New("mailbox: source takes no webhook deliveries")
	ErrMissingOption = errors.New("mailbox: required option missing")
)

/*
MisconfiguredError reports a delivery the provider's own settings make
useless. The mail stays with the provider, which retries, while an
operator changes the named setting.
*/
type MisconfiguredError struct {
	Source  string
	Setting string
}

/*
Error names the source and the setting to change.
*/
func (e *MisconfiguredError) Error() string {
	return "mailbox: source " + e.Source + " misconfigured: " + e.Setting
}

/*
Verdict is a provider's authentication or scanning result. Empty means
the provider gave none.
*/
type Verdict string

/*
Verdicts a provider can give.
*/
const (
	VerdictPass    Verdict = "pass"
	VerdictFail    Verdict = "fail"
	VerdictNone    Verdict = "none"
	VerdictUnknown Verdict = "unknown"
)

/*
Verdicts are what the receiving provider concluded about a message.
They are trustworthy only when the provider received the connection
itself. DMARCPolicy is the sender domain's published policy, set when
DMARC failed.
*/
type Verdicts struct {
	SPF, DKIM, DMARC, Spam, Virus Verdict
	DMARCPolicy                   string
}

/*
RawRef names where the provider stored the raw message: a path on a
filesystem disk.
*/
type RawRef struct {
	Disk string
	Path string
}

/*
Envelope is one received message as its source delivered it.

EnvelopeFrom is the SMTP sender, empty for the null sender bounces use.
Recipients are the envelope recipients the provider accepted it for.
OpenRaw opens the raw message; RawRef, when set, lets a later job open
it again without the source. Accepted, when set, must be called once
the raw message is safely kept: only then does the source refuse the
same delivery again, so a failed attempt can always be retried. Advice,
when set, names a setting worth changing at the provider although this
delivery arrived fine.
*/
type Envelope struct {
	Source            string
	ProviderMessageID string
	EnvelopeFrom      string
	Recipients        []string
	SizeBytes         int64
	ReceivedAt        time.Time
	Verdicts          Verdicts
	OpenRaw           func(ctx context.Context) (io.ReadCloser, error)
	RawRef            *RawRef
	Accepted          func(ctx context.Context) error
	Advice            string
}

/*
Delivery is one webhook request from a provider. Body is capped by the
caller at the source's MaxWebhookBytes plus one byte, and Size is the
declared length, -1 when unknown.
*/
type Delivery struct {
	Header http.Header
	Query  url.Values
	Body   io.Reader
	Size   int64
}

/*
Source is a configured inbound mail source.
*/
type Source interface {
	/*
		Name returns the driver name, recorded on every envelope.
	*/
	Name() string
}

/*
Receiver is a source fed by provider webhooks.
*/
type Receiver interface {
	Source

	/*
		Receive reads one webhook delivery. It returns the envelope of a
		received message, or nil when the delivery needed handling but
		carried no mail, such as a subscription confirmation.
	*/
	Receive(ctx context.Context, d Delivery) (*Envelope, error)

	/*
		MaxWebhookBytes returns the largest webhook body the source takes.
	*/
	MaxWebhookBytes() int64
}

/*
StatusMapper is a receiver whose provider expects its own status codes,
for example one that stops retrying only on a specific answer.
*/
type StatusMapper interface {
	/*
		Status returns the status code answering the error.
	*/
	Status(err error) int
}

/*
RawFetcher is a receiver that fetches the raw message itself, so the
size of its webhook body says nothing about the memory a delivery needs.
*/
type RawFetcher interface {
	/*
		FetchesRaw reports that the source reads the raw message itself.
	*/
	FetchesRaw() bool
}

/*
Status returns the status code a webhook answers the receiver's result
with.
*/
func Status(receiver Receiver, err error) int {
	if mapper, ok := receiver.(StatusMapper); ok && err != nil {
		return mapper.Status(err)
	}

	return DefaultStatus(err)
}

/*
DefaultStatus is the status code of a result when the provider expects
nothing special. Success and the end of a subscription, which a retry
cannot change, answer OK; a rejected delivery is unauthorized, an
oversized one too large, and anything else asks the provider to try
again later.
*/
func DefaultStatus(err error) int {
	switch {
	case err == nil, errors.Is(err, ErrUnsubscribed):
		return http.StatusOK
	case errors.Is(err, ErrRejected):
		return http.StatusUnauthorized
	case errors.Is(err, ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusServiceUnavailable
	}
}
