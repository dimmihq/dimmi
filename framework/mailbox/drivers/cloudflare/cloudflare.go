/*
|--------------------------------------------------------------------------
| Cloudflare Inbound Source
|--------------------------------------------------------------------------
|
| Cloudflare Email Routing hands mail to a Worker, which posts the raw
| message here unchanged. The Worker signs a timestamp and the digest of
| the message with a shared secret. A signature older than five minutes,
| or one already accepted, is refused.
|
| Only one verdict is believed: the first Authentication-Results header,
| and only when Cloudflare's own mail server wrote it. A sender can write
| that header too, but never above the one Cloudflare adds.
|
*/

package cloudflare

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/internal/webhook"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
The headers the Worker sends beside the raw message.
*/
const (
	HeaderTimestamp    = "X-Inbound-Timestamp"
	HeaderSignature    = "X-Inbound-Signature"
	HeaderEnvelopeFrom = "X-Inbound-Envelope-From"
	HeaderEnvelopeTo   = "X-Inbound-Envelope-To"
	HeaderFallback     = "X-Inbound-Fallback"
)

/*
FallbackAdvice names the Worker setting a sender's bounce depends on: it
is left empty, so mail the install cannot take right now bounces.
*/
const FallbackAdvice = "FALLBACK_ADDRESS in the Worker is empty, so senders get a bounce during an outage"

/*
signatureWindow is how far a signature's timestamp may be from now.
*/
const signatureWindow = 5 * time.Minute

/*
authServer is the mail server whose verdicts are believed.
*/
const authServer = "mx.cloudflare.net"

/*
Options configure a cloudflare source: the secret shared with the Worker
and the largest message taken. Cloudflare refuses mail over 25 MiB
itself.
*/
type Options struct {
	Secret          string `koanf:"secret"   secret:"true"`
	MaxWebhookBytes int64  `default:"67108864" koanf:"max_webhook_bytes"`
}

/*
Source receives mail from the Email Routing Worker.
*/
type Source struct {
	clock   clock.Clock
	replays *mailbox.Replays
	opts    Options
}

/*
Factory builds cloudflare sources for the mailbox manager.
*/
func Factory(source clock.Clock, replays *mailbox.Replays) manager.Factory[mailbox.Source] {
	return func(name string, section config.Section) (mailbox.Source, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		if opts.Secret == "" {
			return nil, fmt.Errorf("mailbox.sources.%s.secret: %w", name, mailbox.ErrMissingOption)
		}

		return New(source, replays, opts), nil
	}
}

/*
New returns a source with the options.
*/
func New(source clock.Clock, replays *mailbox.Replays, opts Options) *Source {
	return &Source{clock: source, replays: replays, opts: opts}
}

/*
Name returns the driver name.
*/
func (s *Source) Name() string {
	return "cloudflare"
}

/*
MaxWebhookBytes returns the largest body taken.
*/
func (s *Source) MaxWebhookBytes() int64 {
	return s.opts.MaxWebhookBytes
}

/*
Sign returns the signature the Worker sends for a timestamp and a raw
message.
*/
func Sign(secret, timestamp string, raw []byte) string {
	digest := sha256.Sum256(raw)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + hex.EncodeToString(digest[:])))

	return hex.EncodeToString(mac.Sum(nil))
}

/*
Receive checks the signature and reads the delivery.
*/
func (s *Source) Receive(ctx context.Context, d mailbox.Delivery) (*mailbox.Envelope, error) {
	raw, err := webhook.Read(d, s.opts.MaxWebhookBytes)
	if err != nil {
		return nil, err
	}

	signature, timestamp := d.Header.Get(HeaderSignature), d.Header.Get(HeaderTimestamp)
	if err := s.verify(signature, timestamp, raw); err != nil {
		return nil, err
	}

	if err := s.replays.Check(ctx, s.Name(), signature); err != nil {
		return nil, err
	}

	return &mailbox.Envelope{
		Source:            s.Name(),
		ProviderMessageID: webhook.RawID(raw),
		EnvelopeFrom:      webhook.Sender(d.Header.Get(HeaderEnvelopeFrom)),
		Recipients:        webhook.Recipients(strings.Split(d.Header.Get(HeaderEnvelopeTo), ",")),
		SizeBytes:         int64(len(raw)),
		ReceivedAt:        s.clock.Now(),
		Verdicts:          verdicts(raw),
		OpenRaw:           webhook.Opener(raw),
		Accepted:          s.replays.Accepted(s.Name(), signature),
		Advice:            advice(d.Header.Get(HeaderFallback)),
	}, nil
}

func advice(fallback string) string {
	if fallback == "off" {
		return FallbackAdvice
	}

	return ""
}

func (s *Source) verify(signature, timestamp string, raw []byte) error {
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: delivery is not signed", mailbox.ErrRejected)
	}

	age := s.clock.Now().Sub(time.Unix(seconds, 0))
	if age > signatureWindow || age < -signatureWindow {
		return fmt.Errorf("%w: signature outside its window", mailbox.ErrRejected)
	}

	if !hmac.Equal([]byte(signature), []byte(Sign(s.opts.Secret, timestamp, raw))) {
		return fmt.Errorf("%w: bad signature", mailbox.ErrRejected)
	}

	return nil
}

func verdicts(raw []byte) mailbox.Verdicts {
	found, ok := webhook.TopAuthResults(raw, authServer)
	if !ok {
		return mailbox.Verdicts{}
	}

	return found
}
