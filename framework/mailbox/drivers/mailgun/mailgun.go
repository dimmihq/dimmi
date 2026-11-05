/*
|--------------------------------------------------------------------------
| Mailgun Inbound Source
|--------------------------------------------------------------------------
|
| Mailgun forwards mail to a route URL ending in "mime", the only form in
| which it sends the raw message. Each post signs a timestamp and a token
| with the account's signing key. A signature outside five minutes, or a
| token already accepted, is refused.
|
| Mailgun gives up on 406 and retries anything else for eight hours, so a
| refused delivery answers 406 and a passing failure stays retryable. Its
| verdicts sit in headers it writes on top of the message, so only the
| first occurrence of each is believed.
|
*/

package mailgun

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
signatureWindow is how far a signature's timestamp may be from now.
*/
const signatureWindow = 5 * time.Minute

/*
Options configure a mailgun source: the account's signing key and the
largest body taken.
*/
type Options struct {
	SigningKey      string `koanf:"signing_key"     secret:"true"`
	MaxWebhookBytes int64  `default:"67108864"      koanf:"max_webhook_bytes"`
}

/*
Source receives mail from a Mailgun route.
*/
type Source struct {
	clock   clock.Clock
	replays *mailbox.Replays
	opts    Options
}

/*
Factory builds mailgun sources for the mailbox manager.
*/
func Factory(source clock.Clock, replays *mailbox.Replays) manager.Factory[mailbox.Source] {
	return func(name string, section config.Section) (mailbox.Source, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		if opts.SigningKey == "" {
			return nil, fmt.Errorf("mailbox.sources.%s.signing_key: %w", name, mailbox.ErrMissingOption)
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
	return "mailgun"
}

/*
MaxWebhookBytes returns the largest body taken.
*/
func (s *Source) MaxWebhookBytes() int64 {
	return s.opts.MaxWebhookBytes
}

/*
Status answers a refused delivery with the one code Mailgun does not
retry.
*/
func (s *Source) Status(err error) int {
	if errors.Is(err, mailbox.ErrRejected) {
		return http.StatusNotAcceptable
	}

	return mailbox.DefaultStatus(err)
}

/*
Receive checks the signature and reads the delivery.
*/
func (s *Source) Receive(ctx context.Context, d mailbox.Delivery) (*mailbox.Envelope, error) {
	body, err := webhook.Read(d, s.opts.MaxWebhookBytes)
	if err != nil {
		return nil, err
	}

	form, err := webhook.Form(d.Header.Get("Content-Type"), body)
	if err != nil {
		return nil, err
	}

	if err := s.verify(form); err != nil {
		return nil, err
	}

	token := form.Get("token")
	if err := s.replays.Check(ctx, s.Name(), token); err != nil {
		return nil, err
	}

	raw := []byte(form.Get("body-mime"))
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: post has no body-mime field", mailbox.ErrRejected)
	}

	return &mailbox.Envelope{
		Source:            s.Name(),
		ProviderMessageID: webhook.RawID(raw),
		EnvelopeFrom:      webhook.Sender(form.Get("sender")),
		Recipients:        webhook.Recipients(strings.Split(form.Get("recipient"), ",")),
		SizeBytes:         int64(len(raw)),
		ReceivedAt:        s.clock.Now(),
		Verdicts:          verdicts(raw),
		OpenRaw:           webhook.Opener(raw),
		Accepted:          s.replays.Accepted(s.Name(), token),
	}, nil
}

func (s *Source) verify(form url.Values) error {
	timestamp, token := form.Get("timestamp"), form.Get("token")

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || token == "" {
		return fmt.Errorf("%w: post is not signed", mailbox.ErrRejected)
	}

	age := s.clock.Now().Sub(time.Unix(seconds, 0))
	if age > signatureWindow || age < -signatureWindow {
		return fmt.Errorf("%w: signature outside its window", mailbox.ErrRejected)
	}

	given, err := hex.DecodeString(form.Get("signature"))
	if err != nil || !hmac.Equal(given, sum(s.opts.SigningKey, timestamp, token)) {
		return fmt.Errorf("%w: bad signature", mailbox.ErrRejected)
	}

	return nil
}

/*
Sign returns the signature Mailgun sends with a timestamp and a token.
*/
func Sign(key, timestamp, token string) string {
	return hex.EncodeToString(sum(key, timestamp, token))
}

func sum(key, timestamp, token string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(timestamp + token))

	return mac.Sum(nil)
}

func verdicts(raw []byte) mailbox.Verdicts {
	headers := webhook.Headers(raw)

	return mailbox.Verdicts{
		SPF:  webhook.Verdict(webhook.First(headers, "X-Mailgun-Spf")),
		DKIM: webhook.Verdict(webhook.First(headers, "X-Mailgun-Dkim-Check-Result")),
		Spam: webhook.SpamVerdict(webhook.First(headers, "X-Mailgun-Sflag")),
	}
}
