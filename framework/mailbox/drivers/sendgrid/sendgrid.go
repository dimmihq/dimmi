/*
|--------------------------------------------------------------------------
| SendGrid Inbound Source
|--------------------------------------------------------------------------
|
| SendGrid's Inbound Parse posts a multipart form holding the raw message
| when "POST the raw, full MIME message" is ticked. A security policy
| signs the request with ECDSA over the timestamp and the body exactly as
| it was sent, so the signature is checked before anything is parsed, and
| a proxy that rewrites the body makes it fail on purpose.
|
| The envelope field names the real recipients. SendGrid's own verdicts
| sit in plain form fields, not in the message, so a sender cannot set
| them.
|
*/

package sendgrid

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/internal/webhook"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
The headers carrying the signature and its timestamp.
*/
const (
	HeaderSignature = "X-Twilio-Email-Event-Webhook-Signature"
	HeaderTimestamp = "X-Twilio-Email-Event-Webhook-Timestamp"
)

/*
spamThreshold is the score SendGrid's own spam check marks as spam.
*/
const spamThreshold = 5.0

/*
Options configure a sendgrid source: the security policy's public key in
base64 and the largest body taken.
*/
type Options struct {
	PublicKey       string `koanf:"public_key"`
	MaxWebhookBytes int64  `default:"67108864" koanf:"max_webhook_bytes"`
}

/*
Source receives mail from SendGrid's Inbound Parse.
*/
type Source struct {
	clock clock.Clock
	key   *ecdsa.PublicKey
	opts  Options
}

/*
Factory builds sendgrid sources for the mailbox manager.
*/
func Factory(source clock.Clock) manager.Factory[mailbox.Source] {
	return func(name string, section config.Section) (mailbox.Source, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		if opts.PublicKey == "" {
			return nil, fmt.Errorf("mailbox.sources.%s.public_key: %w", name, mailbox.ErrMissingOption)
		}

		return New(source, opts)
	}
}

/*
New returns a source with the options, or an error when the public key
is not a base64 ECDSA key.
*/
func New(source clock.Clock, opts Options) (*Source, error) {
	der, err := base64.StdEncoding.DecodeString(opts.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("sendgrid public key: %w", err)
	}

	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("sendgrid public key: %w", err)
	}

	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("sendgrid public key is not ecdsa: %w", mailbox.ErrMissingOption)
	}

	return &Source{clock: source, key: key, opts: opts}, nil
}

/*
Name returns the driver name.
*/
func (s *Source) Name() string {
	return "sendgrid"
}

/*
MaxWebhookBytes returns the largest body taken.
*/
func (s *Source) MaxWebhookBytes() int64 {
	return s.opts.MaxWebhookBytes
}

/*
Receive checks the signature over the body as sent, then reads the form.
*/
func (s *Source) Receive(_ context.Context, d mailbox.Delivery) (*mailbox.Envelope, error) {
	body, err := webhook.Read(d, s.opts.MaxWebhookBytes)
	if err != nil {
		return nil, err
	}

	if !s.signed(d.Header.Get(HeaderSignature), d.Header.Get(HeaderTimestamp), body) {
		return nil, fmt.Errorf("%w: bad signature", mailbox.ErrRejected)
	}

	form, err := webhook.Form(d.Header.Get("Content-Type"), body)
	if err != nil {
		return nil, err
	}

	raw := []byte(form.Get("email"))
	if len(raw) == 0 {
		return nil, &mailbox.MisconfiguredError{Source: s.Name(), Setting: "POST the raw, full MIME message"}
	}

	var envelope struct {
		To   []string `json:"to"`
		From string   `json:"from"`
	}
	if err := json.Unmarshal([]byte(form.Get("envelope")), &envelope); err != nil {
		return nil, fmt.Errorf("%w: sendgrid envelope: %w", mailbox.ErrRejected, err)
	}

	return &mailbox.Envelope{
		Source:            s.Name(),
		ProviderMessageID: webhook.RawID(raw),
		EnvelopeFrom:      webhook.Sender(envelope.From),
		Recipients:        webhook.Recipients(envelope.To),
		SizeBytes:         int64(len(raw)),
		ReceivedAt:        s.clock.Now(),
		Verdicts: mailbox.Verdicts{
			SPF: webhook.Verdict(form.Get("SPF")), DKIM: dkimVerdict(form.Get("dkim")),
			Spam: spamVerdict(form.Get("spam_score")),
		},
		OpenRaw: webhook.Opener(raw),
	}, nil
}

func (s *Source) signed(signature, timestamp string, body []byte) bool {
	given, err := base64.StdEncoding.DecodeString(signature)
	if err != nil || timestamp == "" {
		return false
	}

	digest := sha256.New()
	digest.Write([]byte(timestamp))
	digest.Write(body)

	return ecdsa.VerifyASN1(s.key, digest.Sum(nil), given)
}

/*
dkimVerdict reads SendGrid's list of signing domains, each with its
result: one passing signature is enough, since alignment is judged
later.
*/
func dkimVerdict(value string) mailbox.Verdict {
	value = strings.ToLower(value)

	switch {
	case value == "" || value == "none":
		return mailbox.VerdictNone
	case strings.Contains(value, "pass"):
		return mailbox.VerdictPass
	case strings.Contains(value, "fail"):
		return mailbox.VerdictFail
	default:
		return mailbox.VerdictUnknown
	}
}

/*
spamVerdict reads the score, which SendGrid leaves out when its check is
off or the message is large.
*/
func spamVerdict(score string) mailbox.Verdict {
	value, err := strconv.ParseFloat(strings.TrimSpace(score), 64)
	if err != nil {
		return ""
	}

	if value >= spamThreshold {
		return mailbox.VerdictFail
	}

	return mailbox.VerdictPass
}
