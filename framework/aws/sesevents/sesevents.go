/*
|--------------------------------------------------------------------------
| SES Delivery Events
|--------------------------------------------------------------------------
|
| A configuration set publishes what became of each message to an SNS
| topic, which posts it to a webhook. A post is trusted only when SNS
| signed it for a configured topic: anyone can post to a public endpoint.
|
| Only bounces and complaints are read. A bounce is permanent when SES
| says the address will never take mail, and transient when it gave up
| on this message alone. Every other event, and the test message SES
| sends when a destination is set up, yields nothing.
|
*/

package sesevents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/dimmihq/dimmi/framework/aws/sns"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
MaxBytes is the largest SNS post taken: SNS posts at most 256 KB.
*/
const MaxBytes = 1 << 20

/*
The kinds of event read.
*/
const (
	KindBounce    = "bounce"
	KindComplaint = "complaint"
)

/*
ErrRejected refuses a post SNS did not sign for a configured
topic, or one that is not an SNS message at all.
*/
var ErrRejected = errors.New("sesevents: rejected")

/*
Event is what became of one sent message. MessageID is the id SES gave
the message when it took it. Permanent is set on a bounce SES will not
retry because the address is bad. NotSpam is set on a complaint whose
feedback says the recipient marked the mail as wanted.
*/
type Event struct {
	Kind       string
	MessageID  string
	Permanent  bool
	NotSpam    bool
	Recipients []Recipient
}

/*
Recipient is one address an event is about, with the delivery
status code and the receiving server's words when SES has them.
*/
type Recipient struct {
	Address    string
	Status     string
	Diagnostic string
}

/*
Reader reads the events SES publishes through SNS.
*/
type Reader struct {
	client   *httpclient.Client
	verifier *sns.Verifier
	topics   []string
}

/*
New returns a reader accepting posts from the topics.
*/
func New(client *httpclient.Client, topicARNs []string) *Reader {
	return &Reader{client: client, verifier: sns.NewVerifier(client), topics: topicARNs}
}

/*
Receive reads one SNS post. A subscription confirmation is confirmed
and yields no event, and so does the end of a subscription or an event
that is neither a bounce nor a complaint.

Errors: [ErrRejected] for a post that cannot be trusted; anything
else may pass on a retry.
*/
func (e *Reader) Receive(ctx context.Context, body []byte) (*Event, error) {
	msg, err := sns.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRejected, err)
	}

	if err := e.verifier.Verify(ctx, msg, sns.Allow{TopicARNs: e.topics}); err != nil {
		return nil, untrusted(err)
	}

	switch msg.Type {
	case sns.TypeSubscriptionConfirmation:
		return nil, e.confirm(ctx, msg.SubscribeURL)
	case sns.TypeUnsubscribeConfirmation:
		return nil, nil
	}

	return Parse(msg.Message), nil
}

/*
untrusted rejects a message that failed the checks, and passes on a
failure to fetch the certificate, which a retry may fix.
*/
func untrusted(err error) error {
	for _, rejected := range []error{
		sns.ErrTopicNotAllowed, sns.ErrSignature, sns.ErrCertificate, sns.ErrInvalidMessage,
	} {
		if errors.Is(err, rejected) {
			return fmt.Errorf("%w: %w", ErrRejected, err)
		}
	}

	return fmt.Errorf("verify sns message: %w", err)
}

/*
confirm visits the subscribe URL the signed message gave.
*/
func (e *Reader) confirm(ctx context.Context, subscribeURL string) error {
	resp, err := e.client.New().WithoutRedirects().Get(ctx, subscribeURL)
	if err != nil {
		return fmt.Errorf("confirm sns subscription: %w", err)
	}

	if resp.Status != http.StatusOK {
		return fmt.Errorf("confirm sns subscription: status %d", resp.Status)
	}

	return nil
}

/*
published is the document SES publishes. A configuration set names the
kind eventType; an identity's own notifications name it
notificationType.
*/
type published struct {
	EventType        string
	NotificationType string
	Mail             struct {
		MessageID string
	}
	Bounce struct {
		BounceType        string
		BouncedRecipients []struct {
			EmailAddress   string
			Status         string
			DiagnosticCode string
		}
	}
	Complaint struct {
		ComplaintFeedbackType string
		ComplainedRecipients  []struct {
			EmailAddress string
		}
	}
}

/*
Parse reads the bounce or complaint in a notification's text, and
returns nil for anything else.
*/
func Parse(text string) *Event {
	var p published
	if json.Unmarshal([]byte(text), &p) != nil || p.Mail.MessageID == "" {
		return nil
	}

	kind := p.EventType
	if kind == "" {
		kind = p.NotificationType
	}

	switch kind {
	case "Bounce":
		event := &Event{Kind: KindBounce, MessageID: p.Mail.MessageID, Permanent: p.Bounce.BounceType == "Permanent"}
		for _, r := range p.Bounce.BouncedRecipients {
			event.Recipients = append(event.Recipients, Recipient{
				Address: r.EmailAddress, Status: r.Status, Diagnostic: r.DiagnosticCode,
			})
		}

		return event
	case "Complaint":
		event := &Event{
			Kind: KindComplaint, MessageID: p.Mail.MessageID,
			NotSpam: p.Complaint.ComplaintFeedbackType == "not-spam",
		}
		for _, r := range p.Complaint.ComplainedRecipients {
			event.Recipients = append(event.Recipients, Recipient{Address: r.EmailAddress})
		}

		return event
	}

	return nil
}
