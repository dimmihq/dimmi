/*
|--------------------------------------------------------------------------
| SES Inbound Source
|--------------------------------------------------------------------------
|
| Amazon SES receives the mail, stores each message in an S3 bucket and
| announces it through an SNS topic posting to a webhook. A notification
| is trusted only when SNS signed it for a configured topic: anyone can
| post to a public endpoint.
|
| The webhook stays quick: it checks the signature, the stored object's
| size and the receipt, never the message itself. The raw message is
| opened later, through the filesystem disk configured on the bucket.
| SES saw the sending server's connection, so its SPF, DKIM and DMARC
| verdicts come along with the envelope.
|
*/

package ses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/aws/sns"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Options configure an ses source: the SNS topics whose notifications are
accepted, the filesystem disk on the bucket SES stores messages in, and
the largest message accepted. SES itself stores at most 40 MB.
*/
type Options struct {
	TopicARNs       []string `koanf:"topic_arns"`
	Disk            string   `default:"ses_inbound" koanf:"disk"`
	MaxMessageBytes int64    `default:"41943040" koanf:"max_message_bytes"`
}

/*
Source receives mail from SES notifications.
*/
type Source struct {
	client   *httpclient.Client
	verifier *sns.Verifier
	disk     filesystem.Filesystem
	opts     Options
}

/*
Factory builds ses sources for the mailbox manager, opening the disk the
options name.
*/
func Factory(client *httpclient.Client, disks *filesystem.Manager) manager.Factory[mailbox.Source] {
	return func(_ string, section config.Section) (mailbox.Source, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		disk, err := disks.Disk(opts.Disk)
		if err != nil {
			return nil, err
		}

		return New(client, disk, opts), nil
	}
}

/*
New returns a source reading raw messages from the disk.
*/
func New(client *httpclient.Client, disk filesystem.Filesystem, opts Options) *Source {
	return &Source{client: client, verifier: sns.NewVerifier(client), disk: disk, opts: opts}
}

/*
Name returns the driver name.
*/
func (s *Source) Name() string {
	return "ses"
}

/*
Receive reads one SNS post. A subscription confirmation is confirmed
and yields no envelope; the end of the subscription is reported as
mailbox.ErrUnsubscribed.
*/
func (s *Source) Receive(ctx context.Context, body []byte) (*mailbox.Envelope, error) {
	msg, err := sns.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", mailbox.ErrRejected, err)
	}

	if err := s.verifier.Verify(ctx, msg, sns.Allow{TopicARNs: s.opts.TopicARNs}); err != nil {
		return nil, verifyError(err)
	}

	switch msg.Type {
	case sns.TypeSubscriptionConfirmation:
		return nil, s.confirm(ctx, msg.SubscribeURL)
	case sns.TypeUnsubscribeConfirmation:
		return nil, fmt.Errorf("%w: topic %s", mailbox.ErrUnsubscribed, msg.TopicARN)
	}

	return s.received(ctx, msg.Message)
}

/*
verifyError rejects a message that failed the checks, and passes on a
failure to fetch the certificate, which a retry may fix.
*/
func verifyError(err error) error {
	untrusted := []error{sns.ErrTopicNotAllowed, sns.ErrSignature, sns.ErrCertificate, sns.ErrInvalidMessage}
	for _, rejected := range untrusted {
		if errors.Is(err, rejected) {
			return fmt.Errorf("%w: %w", mailbox.ErrRejected, err)
		}
	}

	return fmt.Errorf("verify sns message: %w", err)
}

/*
confirm visits the subscribe URL the signed message gave.
*/
func (s *Source) confirm(ctx context.Context, subscribeURL string) error {
	resp, err := s.client.New().WithoutRedirects().Get(ctx, subscribeURL)
	if err != nil {
		return fmt.Errorf("confirm sns subscription: %w", err)
	}

	if resp.Status != http.StatusOK {
		return fmt.Errorf("confirm sns subscription: status %d", resp.Status)
	}

	return nil
}

/*
received builds the envelope of a received message notification. Other
notifications, such as the setup test SES sends, yield none.
*/
func (s *Source) received(ctx context.Context, text string) (*mailbox.Envelope, error) {
	var n notification
	if err := json.Unmarshal([]byte(text), &n); err != nil {
		return nil, fmt.Errorf("%w: ses notification: %w", mailbox.ErrRejected, err)
	}

	if n.NotificationType != "Received" {
		return nil, nil
	}

	key := n.Receipt.Action.ObjectKey
	if n.Receipt.Action.Type != "S3" || key == "" {
		return nil, fmt.Errorf("%w: receipt action %q stores no s3 object", mailbox.ErrRejected, n.Receipt.Action.Type)
	}

	receivedAt, err := time.Parse(time.RFC3339, n.Receipt.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("%w: receipt timestamp: %w", mailbox.ErrRejected, err)
	}

	attrs, err := s.disk.Stat(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("stat raw message %s: %w", key, err)
	}

	if attrs.Size > s.opts.MaxMessageBytes {
		return nil, fmt.Errorf("%w: %d bytes", mailbox.ErrTooLarge, attrs.Size)
	}

	return &mailbox.Envelope{
		Source:            s.Name(),
		ProviderMessageID: n.Mail.MessageID,
		EnvelopeFrom:      strings.Trim(n.Mail.Source, "<>"),
		Recipients:        n.Receipt.Recipients,
		SizeBytes:         attrs.Size,
		ReceivedAt:        receivedAt,
		Verdicts:          n.Receipt.verdicts(),
		RawRef:            &mailbox.RawRef{Disk: s.opts.Disk, Path: key},
		OpenRaw: func(ctx context.Context) (io.ReadCloser, error) {
			return s.disk.Open(ctx, key)
		},
	}, nil
}

/*
notification is the SES receipt notification inside the SNS message.
*/
type notification struct {
	NotificationType string
	Mail             struct{ Source, MessageID string }
	Receipt          receipt
}

/*
receipt is what SES did with the message and what it concluded.
*/
type receipt struct {
	Timestamp                                                        string
	Recipients                                                       []string
	SPFVerdict, DKIMVerdict, DMARCVerdict, SpamVerdict, VirusVerdict struct{ Status string }
	DMARCPolicy                                                      string
	Action                                                           struct{ Type, ObjectKey string }
}

func (r receipt) verdicts() mailbox.Verdicts {
	return mailbox.Verdicts{
		SPF:         verdict(r.SPFVerdict.Status),
		DKIM:        verdict(r.DKIMVerdict.Status),
		DMARC:       verdict(r.DMARCVerdict.Status),
		Spam:        verdict(r.SpamVerdict.Status),
		Virus:       verdict(r.VirusVerdict.Status),
		DMARCPolicy: strings.ToLower(r.DMARCPolicy),
	}
}

/*
verdict maps an SES status. GRAY means SES could not tell, which is no
verdict either way; DISABLED means the check did not run.
*/
func verdict(status string) mailbox.Verdict {
	switch status {
	case "PASS":
		return mailbox.VerdictPass
	case "FAIL":
		return mailbox.VerdictFail
	case "GRAY":
		return mailbox.VerdictNone
	case "PROCESSING_FAILED":
		return mailbox.VerdictUnknown
	default:
		return ""
	}
}
