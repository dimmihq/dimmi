/*
|--------------------------------------------------------------------------
| Web Push Channel
|--------------------------------------------------------------------------
|
| A notification pushed to a browser or installed app reaches the
| recipient even when no screen is open. Each message is encrypted for
| the one subscription it goes to and signed with the application's
| VAPID key, which push services use to tell who is sending; webpush-go
| does both. Requests leave through the application's HTTP client, so
| an endpoint pointing at a private address is refused like any other
| outbound request.
|
| A recipient may have many subscriptions, one per device. Each gets the
| message on its own; a subscription the push service reports as gone
| is handed to the application to delete, and does not fail the rest.
|
*/

package webpush

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SherClockHolmes/webpush-go"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/notifications"
)

/*
Limits of a push request. The signed VAPID token lives twelve hours,
well under the one day some push services accept. At most 512 bytes of
a refusal's body are kept. A zero TTL or Timeout means four hours or
ten seconds.
*/
const (
	vapidLifetime  = 12 * time.Hour
	maxErrorBody   = 512
	defaultTTL     = 4 * time.Hour
	defaultTimeout = 10 * time.Second
)

/*
Config holds the VAPID key pair, base64url encoded, and the subject
push services reach the sender at: a "mailto:" address or an https
URL. TTL is how long a push service keeps a message for a device that
is offline, unless the message sets its own (four hours). Timeout
bounds one push (ten seconds).
*/
type Config struct {
	VAPIDPublicKey  string        `koanf:"vapid_public_key"`
	VAPIDPrivateKey string        `koanf:"vapid_private_key"`
	Subject         string        `koanf:"subject"`
	TTL             time.Duration `default:"4h"  koanf:"ttl"`
	Timeout         time.Duration `default:"10s" koanf:"timeout"`
}

/*
Urgency tells the push service how soon a device should wake for a
message.
*/
type Urgency string

/*
The urgencies, from waking only on power and Wi-Fi to waking even on a
low battery.
*/
const (
	UrgencyVeryLow Urgency = "very-low"
	UrgencyLow     Urgency = "low"
	UrgencyNormal  Urgency = "normal"
	UrgencyHigh    Urgency = "high"
)

/*
Message is one push. Payload is the plaintext the device receives. TTL
overrides the configured time to live when set. Topic lets a newer
message replace an undelivered one with the same topic; it is at most
32 URL-safe base64 characters.
*/
type Message struct {
	Payload []byte
	TTL     time.Duration
	Urgency Urgency
	Topic   string
}

/*
Notification is a notification that can be pushed.
*/
type Notification interface {
	/*
		ToWebPush returns the push for the delivery.
	*/
	ToWebPush(ctx context.Context, d notifications.Delivery) (Message, error)
}

/*
GoneFunc is told about a subscription the push service reports as gone,
so the application can delete it.
*/
type GoneFunc func(ctx context.Context, d notifications.Delivery, sub Subscription) error

/*
Channel pushes notifications to the recipient's subscriptions.
*/
type Channel struct {
	cfg        Config
	subscriber string
	client     webpush.HTTPClient
	clock      clock.Clock
	gone       GoneFunc
}

/*
Factory builds Web Push channels for the notification manager.
*/
func Factory(client *httpclient.Client, clk clock.Clock, gone GoneFunc) manager.Factory[notifications.Channel] {
	return func(_ string, section config.Section) (notifications.Channel, error) {
		var cfg Config
		if err := section.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("webpush: %w", err)
		}

		return New(cfg, client, clk, gone)
	}
}

/*
New returns a channel posting through the client and signing by the
clock. Gone subscriptions are reported to gone; when it is nil, Send
returns them as errors matching ErrGone.
*/
func New(cfg Config, client *httpclient.Client, clk clock.Clock, gone GoneFunc) (*Channel, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.TTL = cmp.Or(cfg.TTL, defaultTTL)
	cfg.Timeout = cmp.Or(cfg.Timeout, defaultTimeout)

	return &Channel{
		cfg:        cfg,
		subscriber: strings.TrimPrefix(cfg.Subject, "mailto:"),
		client:     client.StdClient(),
		clock:      clk,
		gone:       gone,
	}, nil
}

/*
GenerateKeys returns a new VAPID key pair, base64url encoded.
*/
func GenerateKeys() (privateKey, publicKey string, err error) {
	privateKey, publicKey, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", fmt.Errorf("generate vapid keys: %w", err)
	}

	return privateKey, publicKey, nil
}

/*
Send pushes the notification to every subscription the recipient routes
the channel to: one subscription or a slice of them. Subscriptions the
push service reports as gone go to the channel's GoneFunc.
*/
func (c *Channel) Send(ctx context.Context, d notifications.Delivery, n notifications.Notification) error {
	renderer, ok := n.(Notification)
	if !ok {
		return fmt.Errorf("push %s: %w", n.Type(), notifications.ErrUnsupported)
	}

	subs, err := subscriptions(d.Notifiable.RouteFor(d.Channel))
	if err != nil || len(subs) == 0 {
		return err
	}

	msg, err := renderer.ToWebPush(ctx, d)
	if err != nil {
		return fmt.Errorf("render %s for push: %w", n.Type(), err)
	}

	var errs []error

	for _, sub := range subs {
		err := c.Push(ctx, sub, msg)
		if errors.Is(err, ErrGone) && c.gone != nil {
			err = c.gone(ctx, d, sub)
		}

		if err != nil {
			errs = append(errs, fmt.Errorf("push %s: %w", n.Type(), err))
		}
	}

	return errors.Join(errs...)
}

/*
Push sends one message to one subscription. A refusal is a
*StatusError. A malformed subscription, or an endpoint at an address
the HTTP client refuses, is permanent.
*/
func (c *Channel) Push(ctx context.Context, sub Subscription, msg Message) error {
	if err := sub.validate(); err != nil {
		return fmt.Errorf("%w: %w", notifications.ErrPermanent, err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	resp, err := webpush.SendNotificationWithContext(ctx, msg.Payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{Auth: sub.Keys.Auth, P256dh: sub.Keys.P256dh},
	}, c.options(msg))
	if errors.Is(err, safedial.ErrBlocked) {
		return fmt.Errorf("webpush: %w: %w: %w", notifications.ErrPermanent, httpclient.ErrBlockedHost, err)
	}

	if err != nil {
		return fmt.Errorf("webpush: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))

	return &StatusError{
		Endpoint:   sub.Endpoint,
		Status:     resp.StatusCode,
		Body:       strings.TrimSpace(string(body)),
		RetryAfter: httpclient.RetryAfter(resp.Header.Get("Retry-After"), c.clock.Now()),
	}
}

/*
options translates a message for webpush-go. The subscriber is passed
without its "mailto:" scheme, which the library adds itself.
*/
func (c *Channel) options(msg Message) *webpush.Options {
	ttl := msg.TTL
	if ttl == 0 {
		ttl = c.cfg.TTL
	}

	return &webpush.Options{
		HTTPClient:      c.client,
		Subscriber:      c.subscriber,
		TTL:             int(ttl / time.Second),
		Urgency:         webpush.Urgency(msg.Urgency),
		Topic:           msg.Topic,
		VAPIDPublicKey:  c.cfg.VAPIDPublicKey,
		VAPIDPrivateKey: c.cfg.VAPIDPrivateKey,
		VapidExpiration: c.clock.Now().Add(vapidLifetime),
	}
}

/*
validate checks the VAPID key pair decodes and the subject is a mailto
address or an https URL.
*/
func (c Config) validate() error {
	private, err := decodeKey(c.VAPIDPrivateKey)
	if err != nil || len(private) != 32 {
		return fmt.Errorf("%w: vapid_private_key is not a P-256 private key", ErrInvalidConfig)
	}

	public, err := decodeKey(c.VAPIDPublicKey)
	if err != nil || len(public) != 65 {
		return fmt.Errorf("%w: vapid_public_key is not a P-256 public key", ErrInvalidConfig)
	}

	if !strings.HasPrefix(c.Subject, "mailto:") && !strings.HasPrefix(c.Subject, "https:") {
		return fmt.Errorf("%w: subject must be a mailto: address or an https URL", ErrInvalidConfig)
	}

	return nil
}
