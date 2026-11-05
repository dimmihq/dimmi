/*
|--------------------------------------------------------------------------
| Postmark Inbound Source
|--------------------------------------------------------------------------
|
| Postmark posts each received message as JSON. It has no signature for
| inbound mail, so the webhook URL carries a username and password that
| are compared in constant time. The raw message comes along only when
| "Include raw email content" is ticked on the stream; without it a
| delivery is refused for now, so Postmark keeps the mail and retries
| while an operator fixes the setting.
|
| Postmark stops retrying only on 403, which is exactly what a bad
| password should do. Every other failure stays retryable.
|
*/

package postmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/internal/webhook"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
RawEmailSetting names the Postmark setting a delivery without the raw
message is missing.
*/
const RawEmailSetting = "Include raw email content in JSON payload"

/*
Options configure a postmark source: the credentials in the webhook URL
and the largest body taken. Attachments reach 35 MB, base64 in JSON.
*/
type Options struct {
	Username        string `koanf:"username"`
	Password        string `koanf:"password"         secret:"true"`
	MaxWebhookBytes int64  `default:"67108864"       koanf:"max_webhook_bytes"`
}

/*
Source receives mail from Postmark's inbound webhook.
*/
type Source struct {
	clock clock.Clock
	opts  Options
}

/*
Factory builds postmark sources for the mailbox manager.
*/
func Factory(source clock.Clock) manager.Factory[mailbox.Source] {
	return func(name string, section config.Section) (mailbox.Source, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		if opts.Username == "" || opts.Password == "" {
			return nil, fmt.Errorf("mailbox.sources.%s username and password: %w", name, mailbox.ErrMissingOption)
		}

		return New(source, opts), nil
	}
}

/*
New returns a source with the options.
*/
func New(source clock.Clock, opts Options) *Source {
	return &Source{clock: source, opts: opts}
}

/*
Name returns the driver name.
*/
func (s *Source) Name() string {
	return "postmark"
}

/*
MaxWebhookBytes returns the largest body taken.
*/
func (s *Source) MaxWebhookBytes() int64 {
	return s.opts.MaxWebhookBytes
}

/*
Status answers a refused delivery with the one code Postmark does not
retry.
*/
func (s *Source) Status(err error) int {
	if errors.Is(err, mailbox.ErrRejected) {
		return http.StatusForbidden
	}

	return mailbox.DefaultStatus(err)
}

/*
payload is the part of Postmark's inbound JSON the source reads.
*/
type payload struct {
	MessageID         string
	OriginalRecipient string
	RawEmail          string
	FromFull          address
	ToFull            []address
	CcFull            []address
	BccFull           []address
	Headers           []struct{ Name, Value string }
}

type address struct{ Email string }

/*
Receive checks the credentials and reads the delivery.
*/
func (s *Source) Receive(_ context.Context, d mailbox.Delivery) (*mailbox.Envelope, error) {
	if !s.authorized(d.Header) {
		return nil, fmt.Errorf("%w: bad credentials", mailbox.ErrRejected)
	}

	body, err := webhook.Read(d, s.opts.MaxWebhookBytes)
	if err != nil {
		return nil, err
	}

	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("%w: postmark payload: %w", mailbox.ErrRejected, err)
	}

	if p.RawEmail == "" {
		return nil, &mailbox.MisconfiguredError{Source: s.Name(), Setting: RawEmailSetting}
	}

	raw := []byte(p.RawEmail)

	return &mailbox.Envelope{
		Source:            s.Name(),
		ProviderMessageID: p.MessageID,
		EnvelopeFrom:      p.sender(),
		Recipients:        p.recipients(),
		SizeBytes:         int64(len(raw)),
		ReceivedAt:        s.clock.Now(),
		Verdicts:          p.verdicts(),
		OpenRaw:           webhook.Opener(raw),
	}, nil
}

func (s *Source) authorized(header http.Header) bool {
	request := http.Request{Header: header}

	username, password, ok := request.BasicAuth()
	if !ok {
		return false
	}

	matched := hashing.Equal([]byte(username), []byte(s.opts.Username))

	return hashing.Equal([]byte(password), []byte(s.opts.Password)) && matched
}

func (p payload) header(name string) (string, bool) {
	for _, h := range p.Headers {
		if http.CanonicalHeaderKey(h.Name) == name {
			return h.Value, true
		}
	}

	return "", false
}

func (p payload) sender() string {
	if path, ok := p.header("Return-Path"); ok {
		return webhook.Sender(path)
	}

	return p.FromFull.Email
}

func (p payload) recipients() []string {
	list := []string{p.OriginalRecipient}
	for _, group := range [][]address{p.ToFull, p.CcFull, p.BccFull} {
		for _, a := range group {
			list = append(list, a.Email)
		}
	}

	return webhook.Recipients(list)
}

func (p payload) verdicts() mailbox.Verdicts {
	spam, _ := p.header("X-Spam-Status")
	spf, _ := p.header("Received-Spf")

	return mailbox.Verdicts{Spam: webhook.SpamVerdict(spam), SPF: webhook.Verdict(spf)}
}
