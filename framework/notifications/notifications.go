/*
|--------------------------------------------------------------------------
| Notifications
|--------------------------------------------------------------------------
|
| A notification is one thing the application tells people about, written
| once as a type. It picks the channels it goes to for each recipient and
| says how it reads on each of them; the channels deliver it. Whether a
| recipient should hear about it right now is the application's call,
| asked once per recipient and channel at the moment of delivery.
|
| Every delivery is written in its recipient's locale, never the
| sender's: a recipient that has a locale of its own is served in it,
| and any other in the locale of the context it was sent from. Channels
| render with that locale in the context.
|
| One notification sent to one recipient is one delivery with its own id,
| shared by every channel and kept across retries, so a channel that
| stores or shows it can recognize the same delivery arriving twice.
|
*/

package notifications

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
Notification errors.

ErrUnsupported means the notification has no rendering for a channel it
was sent to. ErrInvalidRoute refuses a route a channel cannot read.
ErrPermanent marks a failure that can never succeed, so a queued
delivery gives up instead of retrying. ErrUnregistered refuses to queue
a notification whose type was never registered, and ErrNoResolver one
for a recipient type nobody can load again. ErrNotifiableGone is what a
resolver returns for a recipient that no longer exists; the delivery is
then dropped. ErrNoQueue refuses SendTx on a manager built without a
queue.
*/
var (
	ErrUnsupported    = errors.New("notifications: notification does not support the channel")
	ErrInvalidRoute   = errors.New("notifications: invalid route")
	ErrPermanent      = errors.New("notifications: permanent failure")
	ErrUnregistered   = errors.New("notifications: notification type not registered")
	ErrNoResolver     = errors.New("notifications: no resolver for notifiable type")
	ErrNotifiableGone = errors.New("notifications: notifiable no longer exists")
	ErrNoQueue        = errors.New("notifications: no queue")
)

/*
Notification is one kind of notice the application sends. Channels it
goes to read their own rendering from it through interfaces their
packages define, such as ToMail.
*/
type Notification interface {
	/*
		Type names the notification, such as "invoice_paid". It is stored
		with every queued delivery and picks the type the delivery is
		decoded into, so it must stay stable and must not read fields.
	*/
	Type() string

	/*
		Via returns the names of the channels the notification goes to
		for the recipient.
	*/
	Via(to Notifiable) []string
}

/*
Notifiable is anyone or anything that receives notifications.
*/
type Notifiable interface {
	/*
		NotifiableType names the kind of recipient, such as "user".
	*/
	NotifiableType() string

	/*
		NotifiableID identifies the recipient among its type.
	*/
	NotifiableID() string

	/*
		RouteFor returns where the named channel reaches the recipient,
		such as an email address, or nil when it cannot.
	*/
	RouteFor(channel string) any
}

/*
Localized is a recipient with a locale of its own, such as the language
a user picked. An empty locale means it has none.
*/
type Localized interface {
	/*
		Locale returns the recipient's locale, such as "fr".
	*/
	Locale() string
}

/*
Delivery is one notification on its way to one recipient over one
channel. ID is shared by every channel the recipient gets the
notification on and stays the same when a queued delivery is retried.
Channel is the name the channel is configured under. Locale is the
locale the delivery renders in, "" when none is known.
*/
type Delivery struct {
	ID         string
	Channel    string
	Locale     string
	Notifiable Notifiable
}

/*
Channel delivers notifications one way, such as by mail.
*/
type Channel interface {
	/*
		Send delivers the notification. It does nothing when the
		recipient has no route for the channel.
	*/
	Send(ctx context.Context, d Delivery, n Notification) error
}

/*
Conditional is a notification that decides at delivery time whether it
still goes out, for example because the recipient already saw what it
is about or asked for quiet. A queued delivery asks when it runs, not
when it is queued.
*/
type Conditional interface {
	/*
		ShouldSend reports whether the delivery goes out.
	*/
	ShouldSend(ctx context.Context, d Delivery) (bool, error)
}

/*
Queued is a notification that sets how its queued deliveries are
pushed, per recipient and channel: a later time, another queue, a
priority.
*/
type Queued interface {
	/*
		QueueOptions returns the options for the delivery's job.
	*/
	QueueOptions(d Delivery) []queue.Opt
}
