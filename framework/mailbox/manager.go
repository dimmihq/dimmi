/*
|--------------------------------------------------------------------------
| Source Manager
|--------------------------------------------------------------------------
|
| Inbound sources are declared in configuration by name, each naming its
| driver with the driver's options beside it. The provider is a deploy
| time choice; the application only ever asks for a source by name.
|
|   mailbox:
|     default: ses
|     sources:
|       ses:
|         driver: ses
|         topic_arns: [arn:aws:sns:us-east-1:123456789012:inbound]
|         disk: ses_inbound
|
| A source named after its driver may leave the driver out.
|
*/

package mailbox

import (
	"fmt"
	"maps"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Config lists the named sources and which one is the default.
*/
type Config struct {
	Default string                    `koanf:"default"`
	Sources map[string]config.Section `koanf:"sources"`
}

/*
Manager resolves named sources.
*/
type Manager struct {
	sources *manager.Manager[Source]
}

/*
NewManager returns a manager for the sources in the configuration.
*/
func NewManager(cfg Config) *Manager {
	sections := make(map[string]config.Section, len(cfg.Sources))
	for name, section := range cfg.Sources {
		sections[name] = withDriver(name, section)
	}

	return &Manager{sources: manager.New[Source]("mailbox source", sections, cfg.Default)}
}

/*
withDriver names the driver of a section that leaves it out.
*/
func withDriver(name string, section config.Section) config.Section {
	if section.String("driver") != "" {
		return section
	}

	named := make(config.Section, len(section)+1)
	maps.Copy(named, section)
	named["driver"] = name

	return named
}

/*
Extend registers the factory for a driver name.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[Source]) {
	m.sources.Extend(driver, factory)
}

/*
Source returns the named source.
*/
func (m *Manager) Source(name string) (Source, error) {
	return m.sources.Instance(name)
}

/*
Default returns the default source.
*/
func (m *Manager) Default() (Source, error) {
	return m.sources.Default()
}

/*
Receiver returns the named source when it takes webhook deliveries.
*/
func (m *Manager) Receiver(name string) (Receiver, error) {
	source, err := m.Source(name)
	if err != nil {
		return nil, err
	}

	receiver, ok := source.(Receiver)
	if !ok {
		return nil, fmt.Errorf("mailbox source %q: %w", name, ErrNotReceiver)
	}

	return receiver, nil
}
