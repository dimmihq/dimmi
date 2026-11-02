/*
|--------------------------------------------------------------------------
| Mail Channel
|--------------------------------------------------------------------------
|
| A notification sent by mail renders as a mailable, the same kind of
| type every other mail of the application is written as, and goes to
| the addresses the recipient routes the channel to. A recipient without
| an address is skipped.
|
| Each channel sends on the mailer its options name, so notifications can
| leave on a stream of their own.
|
| A refusal from the provider that can never succeed, such as an invalid
| address, is marked permanent, so a queued delivery gives up on it
| instead of retrying.
|
*/

package mail

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/notifications"
)

/*
Notification is a notification that can be sent by mail.
*/
type Notification interface {
	/*
		ToMail returns the mail for the delivery.
	*/
	ToMail(ctx context.Context, d notifications.Delivery) (mail.Mailable, error)
}

/*
Options configure a mail channel. Mailer names the mailer notifications
go out on, keeping them apart from other mail; empty means the mail
manager's default mailer.
*/
type Options struct {
	Mailer string `koanf:"mailer"`
}

/*
Channel sends notifications through one mailer of the mail manager.
*/
type Channel struct {
	mails  *mail.Manager
	mailer string
}

/*
Factory builds mail channels for the notification manager, each sending
on the mailer its options name.
*/
func Factory(mails *mail.Manager) manager.Factory[notifications.Channel] {
	return func(name string, section config.Section) (notifications.Channel, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, fmt.Errorf("mail channel %s: %w", name, err)
		}

		return New(mails, opts), nil
	}
}

/*
New returns a channel sending through the mail manager.
*/
func New(mails *mail.Manager, opts Options) *Channel {
	return &Channel{mails: mails, mailer: opts.Mailer}
}

/*
Send renders the notification's mailable and sends it to the addresses
the recipient routes the channel to. The route is an address, an email
string, or a slice of either.
*/
func (c *Channel) Send(ctx context.Context, d notifications.Delivery, n notifications.Notification) error {
	renderer, ok := n.(Notification)
	if !ok {
		return fmt.Errorf("mail %s: %w", n.Type(), notifications.ErrUnsupported)
	}

	to, err := addresses(d.Notifiable.RouteFor(d.Channel))
	if err != nil || len(to) == 0 {
		return err
	}

	mailable, err := renderer.ToMail(ctx, d)
	if err != nil {
		return fmt.Errorf("render mail %s: %w", n.Type(), err)
	}

	pending := c.mails.To(to...)
	if c.mailer != "" {
		pending = pending.Mailer(c.mailer)
	}

	if _, err := pending.Send(ctx, mailable); err != nil {
		if errors.Is(err, mail.ErrPermanent) {
			return fmt.Errorf("mail %s: %w: %w", n.Type(), notifications.ErrPermanent, err)
		}

		return fmt.Errorf("mail %s: %w", n.Type(), err)
	}

	return nil
}

/*
addresses reads a route into mail addresses.
*/
func addresses(route any) ([]mail.Address, error) {
	switch route := route.(type) {
	case nil:
		return nil, nil
	case mail.Address:
		return []mail.Address{route}, nil
	case []mail.Address:
		return route, nil
	case string:
		return parse([]string{route})
	case []string:
		return parse(route)
	default:
		return nil, fmt.Errorf("mail route %T: %w", route, notifications.ErrInvalidRoute)
	}
}

/*
parse reads email strings, skipping empty ones as no address at all.
*/
func parse(route []string) ([]mail.Address, error) {
	to := make([]mail.Address, 0, len(route))

	for _, s := range route {
		if s == "" {
			continue
		}

		address, err := mail.ParseAddress(s)
		if err != nil {
			return nil, fmt.Errorf("mail route: %w: %w", notifications.ErrInvalidRoute, err)
		}

		to = append(to, address)
	}

	return to, nil
}
