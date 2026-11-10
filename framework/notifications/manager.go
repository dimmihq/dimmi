/*
|--------------------------------------------------------------------------
| Notification Manager
|--------------------------------------------------------------------------
|
| Channels are declared in configuration under the names notifications
| pick, each naming its driver with the driver's options beside it. The
| application registers a driver for each one it uses, and may add its
| own, so a new channel needs no change here.
|
|   notifications:
|     queue: notifications
|     channels:
|       mail: { driver: mail }
|       database: { driver: database, table: notifications }
|
| Sending now delivers on every channel in turn. A channel that fails
| does not stop the others, so one broken channel never silences the
| rest; every failure is returned together.
|
*/

package notifications

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
Config names the queue that queued deliveries wait in and declares the
channels. Translator, set in code, is put in the context channels render
with, so notifications can call i18n.T.
*/
type Config struct {
	Queue      string                    `default:"default" koanf:"queue"`
	Channels   map[string]config.Section `koanf:"channels"`
	Translator *i18n.Translator          `koanf:"-"`
}

/*
Resolver loads a recipient again by its id when a queued delivery runs.
It returns ErrNotifiableGone when the recipient no longer exists.
*/
type Resolver func(ctx context.Context, id string) (Notifiable, error)

/*
decoder rebuilds a queued notification from its stored JSON.
*/
type decoder func(data []byte) (Notification, error)

/*
Manager sends notifications through its channels, now or queued. It is
safe for concurrent use; register channels, notification types and
resolvers at boot.
*/
type Manager struct {
	channels   *manager.Manager[Channel]
	queue      *queue.Queue
	queueName  string
	translator *i18n.Translator

	mu        sync.RWMutex
	types     map[string]decoder
	resolvers map[string]Resolver
}

/*
New returns a manager for the channels in the configuration, or, when
it names none, one channel per built-in driver under the driver's own
name: database, mail, broadcast and webpush. With a queue, it registers
the worker that runs queued deliveries, so call it before the queue
starts; without one, only Send works.
*/
func New(cfg Config, q *queue.Queue) (*Manager, error) {
	channels := cfg.Channels
	if len(channels) == 0 {
		channels = DefaultChannels()
	}

	m := &Manager{
		channels:   manager.New[Channel]("notification channel", channels, ""),
		queue:      q,
		queueName:  cfg.Queue,
		translator: cfg.Translator,
		types:      map[string]decoder{},
		resolvers:  map[string]Resolver{},
	}

	if q == nil {
		return m, nil
	}

	if err := queue.Register(q, deliverWorker{manager: m}); err != nil {
		return nil, fmt.Errorf("notifications: %w", err)
	}

	return m, nil
}

/*
DefaultChannels configures one channel per built-in driver, named after
it.
*/
func DefaultChannels() map[string]config.Section {
	out := map[string]config.Section{}
	for _, driver := range []string{"database", "mail", "broadcast", "webpush"} {
		out[driver] = config.Section{"driver": driver}
	}

	return out
}

/*
Extend registers the factory for a channel driver.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[Channel]) {
	m.channels.Extend(driver, factory)
}

/*
Add registers a channel the application builds itself under the name,
replacing any channel of that name. It is for channels whose services
only exist once the application has booted, so they cannot be built
from configuration.
*/
func (m *Manager) Add(name string, channel Channel) {
	m.channels.Swap(name, channel)
}

/*
Channel returns the named channel, building it on first use.
*/
func (m *Manager) Channel(name string) (Channel, error) {
	return m.channels.Instance(name)
}

/*
Resolve registers how recipients of the type are loaded again when a
queued delivery runs.
*/
func (m *Manager) Resolve(notifiableType string, resolver Resolver) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.resolvers[notifiableType] = resolver
}

/*
Send delivers the notification to every recipient now, on every channel
it picks for each. It returns every failure, joined.
*/
func (m *Manager) Send(ctx context.Context, n Notification, to ...Notifiable) error {
	var errs []error

	for _, notifiable := range to {
		id := ids.New().String()

		for _, channel := range n.Via(notifiable) {
			d := Delivery{ID: id, Channel: channel, Locale: localeOf(ctx, notifiable), Notifiable: notifiable}
			if err := m.deliver(ctx, d, n); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

/*
localeOf returns the recipient's own locale, or else the locale of the
context.
*/
func localeOf(ctx context.Context, to Notifiable) string {
	return cmp.Or(ownLocale(to), i18n.Locale(ctx))
}

/*
ownLocale returns the recipient's own locale, or "" when it has none.
*/
func ownLocale(to Notifiable) string {
	if localized, ok := to.(Localized); ok {
		return localized.Locale()
	}

	return ""
}

/*
deliver sends one delivery in its locale unless the notification
declines it.
*/
func (m *Manager) deliver(ctx context.Context, d Delivery, n Notification) error {
	if d.Locale != "" {
		ctx = i18n.WithLocale(ctx, d.Locale)
	}

	if m.translator != nil {
		ctx = i18n.WithTranslator(ctx, m.translator)
	}

	if conditional, ok := n.(Conditional); ok {
		send, err := conditional.ShouldSend(ctx, d)
		if err != nil {
			return fmt.Errorf("check %s via %s: %w", n.Type(), d.Channel, err)
		}

		if !send {
			return nil
		}
	}

	channel, err := m.Channel(d.Channel)
	if err != nil {
		return fmt.Errorf("send %s: %w", n.Type(), err)
	}

	if err := channel.Send(ctx, d, n); err != nil {
		return fmt.Errorf("send %s via %s to %s %s: %w",
			n.Type(), d.Channel, d.Notifiable.NotifiableType(), d.Notifiable.NotifiableID(), err)
	}

	return nil
}
