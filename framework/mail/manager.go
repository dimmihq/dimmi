/*
|--------------------------------------------------------------------------
| Mailer Manager
|--------------------------------------------------------------------------
|
| Mail leaves through named mailers declared in configuration, each
| naming its driver with the driver's options beside it. Separate names
| keep separate streams of mail apart even on one provider account, so
| trouble on one stream never holds up another.
|
|   mail:
|     default: system
|     from: { name: Example, email: hello@example.com }
|     mailers:
|       system:
|         driver: ses
|         configuration_set: system
|
| A name missing from the configuration resolves to the default mailer;
| the caller sets its stream apart through the message: configuration
| set, tenant and tags.
|
*/

package mail

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
ErrFakesNotAllowed is what Fake panics with on a manager that does not
allow fakes.
*/
var ErrFakesNotAllowed = errors.New("mail: fakes are allowed only in tests")

/*
Config lists the named mailers, the default one's name, and the From
address mailables use when they set none. AllowFakes, set in code and
only when the application runs its tests, lets Fake replace the
mailers. Translator, set in code, is put in the context mailables render
with, so their content can call i18n.T.
*/
type Config struct {
	Default    string                    `default:"system" koanf:"default"`
	From       Address                   `koanf:"from"`
	Mailers    map[string]config.Section `koanf:"mailers"`
	AllowFakes bool                      `koanf:"-"`
	Translator *i18n.Translator          `koanf:"-"`
}

/*
Manager resolves named mailers and sends or queues mailables. Tests swap
in a fake with Fake, which every holder of the manager then uses.
*/
type Manager struct {
	mailers    *manager.Manager[Mailer]
	from       Address
	allowFakes bool
	translator *i18n.Translator
	fake       atomic.Pointer[Fake]

	mu        sync.Mutex
	queue     *queue.Queue
	mailables []registration
}

/*
NewManager returns a manager for the mailers in the configuration.
*/
func NewManager(cfg Config) *Manager {
	return &Manager{
		mailers:    manager.New[Mailer]("mailer", cfg.Mailers, cfg.Default),
		from:       cfg.From,
		allowFakes: cfg.AllowFakes,
		translator: cfg.Translator,
	}
}

/*
Extend registers the factory for a driver name.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[Mailer]) {
	m.mailers.Extend(driver, factory)
}

/*
Mailer returns the named mailer, or the default mailer when the name is
not configured.
*/
func (m *Manager) Mailer(name string) (Mailer, error) {
	if fake := m.fake.Load(); fake != nil {
		return fake.mailer(name), nil
	}

	if !slices.Contains(m.mailers.Names(), name) {
		return m.mailers.Default()
	}

	return m.mailers.Instance(name)
}

/*
Default returns the default mailer.
*/
func (m *Manager) Default() (Mailer, error) {
	return m.Mailer(m.mailers.DefaultName())
}

/*
Fake replaces every mailer with one fake that records messages instead
of sending them, and returns it. It panics with ErrFakesNotAllowed
unless the configuration allows fakes: a fake outside tests would
silently drop real mail, which is a programming error.
*/
func (m *Manager) Fake() *Fake {
	if !m.allowFakes {
		panic(ErrFakesNotAllowed)
	}

	fake := &Fake{}
	m.fake.Store(fake)

	return fake
}

/*
To starts a mailable for the recipients, sent on the default mailer.
*/
func (m *Manager) To(recipients ...Address) *PendingMail {
	return &PendingMail{manager: m, to: recipients}
}
