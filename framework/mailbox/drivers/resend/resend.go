/*
|--------------------------------------------------------------------------
| Resend Inbound Source
|--------------------------------------------------------------------------
|
| Resend announces each received mail with a signed webhook holding only
| metadata. The raw message is fetched from Resend's API through a short
| lived link, so one that expired before use is simply asked for again.
| While Resend has no raw message yet the delivery fails, and Resend's
| webhook sender retries it.
|
| Resend keeps the mail it received, so mail missed during an outage can
| be listed and fed through again.
|
*/

package resend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/internal/webhook"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/webhooks"
)

/*
The headers of a signed delivery, which name the same three values the
Standard Webhooks headers do.
*/
const (
	HeaderID        = "Svix-Id"
	HeaderTimestamp = "Svix-Timestamp"
	HeaderSignature = "Svix-Signature"
)

/*
apiTimeout bounds each call to Resend's API.
*/
const apiTimeout = 15 * time.Second

/*
receivedEvent is the type of the webhook announcing received mail.
*/
const receivedEvent = "email.received"

/*
ErrRawPending means Resend has no raw message for the mail yet.
*/
var ErrRawPending = errors.New("resend: raw message not ready")

/*
Options configure a resend source: the secret of this webhook, which is
not the one of outbound events, an API key allowed to read mail, the API
address and the largest body and message taken.
*/
type Options struct {
	WebhookSecret   string `koanf:"webhook_secret"    secret:"true"`
	APIKey          string `koanf:"api_key"           secret:"true"`
	BaseURL         string `default:"https://api.resend.com" koanf:"base_url"`
	MaxWebhookBytes int64  `default:"1048576"         koanf:"max_webhook_bytes"`
	MaxMessageBytes int64  `default:"67108864"        koanf:"max_message_bytes"`
}

/*
Source receives mail from Resend's webhooks.
*/
type Source struct {
	client  *httpclient.Client
	clock   clock.Clock
	replays *mailbox.Replays
	secret  webhooks.Secret
	opts    Options
}

/*
Factory builds resend sources for the mailbox manager.
*/
func Factory(client *httpclient.Client, source clock.Clock, replays *mailbox.Replays) manager.Factory[mailbox.Source] {
	return func(name string, section config.Section) (mailbox.Source, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		if opts.WebhookSecret == "" || opts.APIKey == "" {
			return nil, fmt.Errorf("mailbox.sources.%s webhook_secret and api_key: %w", name, mailbox.ErrMissingOption)
		}

		return New(client, source, replays, opts)
	}
}

/*
New returns a source with the options, or an error when the webhook
secret is malformed.
*/
func New(client *httpclient.Client, source clock.Clock, replays *mailbox.Replays, opts Options) (*Source, error) {
	secret, err := webhooks.ParseSecret(opts.WebhookSecret)
	if err != nil {
		return nil, fmt.Errorf("resend webhook secret: %w", err)
	}

	return &Source{client: client, clock: source, replays: replays, secret: secret, opts: opts}, nil
}

/*
Name returns the driver name.
*/
func (s *Source) Name() string {
	return "resend"
}

/*
MaxWebhookBytes returns the largest webhook body taken. Webhooks carry
only metadata.
*/
func (s *Source) MaxWebhookBytes() int64 {
	return s.opts.MaxWebhookBytes
}

/*
FetchesRaw says the source reads the raw message itself, so its small
webhook body says nothing about the memory a delivery needs.
*/
func (s *Source) FetchesRaw() bool {
	return true
}

/*
event is the part of a webhook the source reads.
*/
type event struct {
	Type string `json:"type"`
	Data struct {
		EmailID string   `json:"email_id"`
		From    string   `json:"from"`
		To      []string `json:"to"`
		Cc      []string `json:"cc"`
		Bcc     []string `json:"bcc"`
	} `json:"data"`
}

/*
Receive checks the signature and fetches the raw message of received
mail. Other events yield no envelope.
*/
func (s *Source) Receive(ctx context.Context, d mailbox.Delivery) (*mailbox.Envelope, error) {
	body, err := webhook.Read(d, s.opts.MaxWebhookBytes)
	if err != nil {
		return nil, err
	}

	if err := s.verify(d.Header, body); err != nil {
		return nil, err
	}

	id := d.Header.Get(HeaderID)
	if err := s.replays.Check(ctx, s.Name(), id); err != nil {
		return nil, err
	}

	var e event
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, fmt.Errorf("%w: resend event: %w", mailbox.ErrRejected, err)
	}

	if e.Type != receivedEvent {
		return nil, nil
	}

	env, err := s.envelope(ctx, e)
	if err != nil {
		return nil, err
	}

	env.Accepted = s.replays.Accepted(s.Name(), id)

	return env, nil
}

func (s *Source) verify(header http.Header, body []byte) error {
	standard := http.Header{}
	standard.Set(webhooks.HeaderID, header.Get(HeaderID))
	standard.Set(webhooks.HeaderTimestamp, header.Get(HeaderTimestamp))
	standard.Set(webhooks.HeaderSignature, header.Get(HeaderSignature))

	err := webhooks.Verify([]webhooks.Secret{s.secret}, standard, body, webhooks.Options{Clock: s.clock})
	if err != nil {
		return fmt.Errorf("%w: %w", mailbox.ErrRejected, err)
	}

	return nil
}

func (s *Source) envelope(ctx context.Context, e event) (*mailbox.Envelope, error) {
	raw, err := s.fetchRaw(ctx, e.Data.EmailID)
	if err != nil {
		return nil, err
	}

	return &mailbox.Envelope{
		Source:            s.Name(),
		ProviderMessageID: e.Data.EmailID,
		EnvelopeFrom:      webhook.Sender(e.Data.From),
		Recipients:        webhook.Recipients(e.Data.To, e.Data.Cc, e.Data.Bcc),
		SizeBytes:         int64(len(raw)),
		ReceivedAt:        s.clock.Now(),
		OpenRaw:           webhook.Opener(raw),
	}, nil
}

/*
fetchRaw asks for the mail's download link and reads it. A link that has
expired by the time it is used is asked for once more.
*/
func (s *Source) fetchRaw(ctx context.Context, id string) ([]byte, error) {
	raw, status, err := s.download(ctx, id)
	if err == nil && status == http.StatusOK {
		return raw, nil
	}

	if err != nil {
		return nil, err
	}

	raw, status, err = s.download(ctx, id)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("download raw email: status %d", status)
	}

	return raw, nil
}

func (s *Source) download(ctx context.Context, id string) ([]byte, int, error) {
	link, err := s.rawLink(ctx, id)
	if err != nil {
		return nil, 0, err
	}

	resp, err := s.client.New().Timeout(apiTimeout).MaxBody(s.opts.MaxMessageBytes).Get(ctx, link)
	if errors.Is(err, httpclient.ErrTooLarge) {
		return nil, 0, fmt.Errorf("%w: raw email over %d bytes", mailbox.ErrTooLarge, s.opts.MaxMessageBytes)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("download raw email: %w", err)
	}

	return resp.Body, resp.Status, nil
}

func (s *Source) rawLink(ctx context.Context, id string) (string, error) {
	resp, err := s.api().Get(ctx, s.opts.BaseURL+"/emails/receiving/"+url.PathEscape(id))
	if err != nil {
		return "", fmt.Errorf("retrieve received email: %w", err)
	}

	if resp.Status != http.StatusOK {
		return "", fmt.Errorf("retrieve received email: status %d", resp.Status)
	}

	var received struct {
		Raw *struct {
			DownloadURL string `json:"download_url"`
		} `json:"raw"`
	}
	if err := resp.JSON(&received); err != nil {
		return "", err
	}

	if received.Raw == nil || received.Raw.DownloadURL == "" {
		return "", ErrRawPending
	}

	return received.Raw.DownloadURL, nil
}

func (s *Source) api() *httpclient.Request {
	return s.client.New().Timeout(apiTimeout).WithToken(s.opts.APIKey).Accept("application/json")
}

/*
Backfill walks the mail Resend received since the given time, newest
first, and hands each envelope to the function. The function's error
stops the walk.
*/
func (s *Source) Backfill(
	ctx context.Context, since time.Time, handle func(context.Context, mailbox.Received) error,
) error {
	after := ""

	for {
		page, err := s.listPage(ctx, after)
		if err != nil {
			return err
		}

		for _, item := range page.Data {
			created := parseTime(item.CreatedAt)
			if created.Before(since) {
				return nil
			}

			if err := s.backfillOne(ctx, item, created, handle); err != nil {
				return err
			}

			after = item.ID
		}

		if !page.HasMore || len(page.Data) == 0 {
			return nil
		}
	}
}

/*
listedEmail is a row of the received mail list.
*/
type listedEmail struct {
	ID        string   `json:"id"`
	CreatedAt string   `json:"created_at"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
	Bcc       []string `json:"bcc"`
}

type listPage struct {
	Data    []listedEmail `json:"data"`
	HasMore bool          `json:"has_more"`
}

func (s *Source) listPage(ctx context.Context, after string) (listPage, error) {
	query := url.Values{"limit": {"100"}}
	if after != "" {
		query.Set("after", after)
	}

	var page listPage

	resp, err := s.api().Get(ctx, s.opts.BaseURL+"/emails/receiving?"+query.Encode())
	if err != nil {
		return page, fmt.Errorf("list received email: %w", err)
	}

	if resp.Status != http.StatusOK {
		return page, fmt.Errorf("list received email: status %d", resp.Status)
	}

	if err := resp.JSON(&page); err != nil {
		return page, err
	}

	return page, nil
}

func (s *Source) backfillOne(
	ctx context.Context, item listedEmail, created time.Time, handle func(context.Context, mailbox.Received) error,
) error {
	var e event

	e.Type = receivedEvent
	e.Data.EmailID, e.Data.From, e.Data.To, e.Data.Cc, e.Data.Bcc = item.ID, item.From, item.To, item.Cc, item.Bcc

	env, err := s.envelope(ctx, e)
	if err != nil {
		return err
	}

	return handle(ctx, mailbox.Received{ID: item.ID, CreatedAt: created, Envelope: env})
}

/*
parseTime reads Resend's timestamp, which a space or a T may separate.
*/
func parseTime(value string) time.Time {
	value = strings.Replace(value, " ", "T", 1)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999-07"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}

	return time.Time{}
}
