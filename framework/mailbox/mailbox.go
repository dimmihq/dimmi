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
webhook.
*/
var (
	ErrRejected     = errors.New("mailbox: delivery rejected")
	ErrTooLarge     = errors.New("mailbox: message too large")
	ErrUnsubscribed = errors.New("mailbox: subscription ended")
	ErrNotReceiver  = errors.New("mailbox: source takes no webhook deliveries")
)

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
it again without the source.
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
		Receive reads one webhook body. It returns the envelope of a
		received message, or nil when the body needed handling but
		carried no mail, such as a subscription confirmation.
	*/
	Receive(ctx context.Context, body []byte) (*Envelope, error)
}
