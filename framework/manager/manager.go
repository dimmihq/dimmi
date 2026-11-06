/*
|--------------------------------------------------------------------------
| Driver Manager
|--------------------------------------------------------------------------
|
| Swappable services are configured as named instances, each naming its
| driver: two disks can both use s3 with different buckets. The manager
| builds an instance the first time it is asked for and returns the same
| one afterwards. Drivers are registered with Extend, never by package
| init, so the binary only carries the drivers the application wires up.
|
*/

package manager

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dimmihq/dimmi/framework/config"
)

/*
Errors from resolving an instance: no configuration for the name, or no
factory registered for the driver it names.
*/
var (
	ErrUnknownInstance = errors.New("manager: unknown instance")
	ErrUnknownDriver   = errors.New("manager: unknown driver")
)

/*
Factory builds one instance from its configuration section.
*/
type Factory[T any] func(name string, section config.Section) (T, error)

/*
Manager resolves and caches named instances.
*/
type Manager[T any] struct {
	kind        string
	sections    map[string]config.Section
	defaultName string

	mu        sync.Mutex
	factories map[string]Factory[T]
	instances map[string]T
}

/*
New returns a manager for one kind of instance, named in error messages,
configured by the sections and defaulting to the named instance.
*/
func New[T any](kind string, sections map[string]config.Section, defaultName string) *Manager[T] {
	return &Manager[T]{
		kind:        kind,
		sections:    sections,
		defaultName: defaultName,
		factories:   map[string]Factory[T]{},
		instances:   map[string]T{},
	}
}

/*
Extend registers the factory for a driver name, replacing any earlier
one. Instances already built keep their driver.
*/
func (m *Manager[T]) Extend(driver string, factory Factory[T]) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.factories[driver] = factory
}

/*
Instance returns the named instance, building it on first use.
*/
func (m *Manager[T]) Instance(name string) (T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if instance, ok := m.instances[name]; ok {
		return instance, nil
	}

	instance, err := m.configured(name)
	if err != nil {
		var zero T

		return zero, err
	}

	m.instances[name] = instance

	return instance, nil
}

/*
Default returns the default instance.
*/
func (m *Manager[T]) Default() (T, error) {
	return m.Instance(m.defaultName)
}

/*
DefaultName returns the name of the default instance.
*/
func (m *Manager[T]) DefaultName() string {
	return m.defaultName
}

/*
Names returns every configured instance name.
*/
func (m *Manager[T]) Names() []string {
	names := make([]string, 0, len(m.sections))
	for name := range m.sections {
		names = append(names, name)
	}

	return names
}

/*
Swap replaces the named instance, built or not. Every holder of the
manager sees the new instance on its next lookup.
*/
func (m *Manager[T]) Swap(name string, instance T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.instances[name] = instance
}

func (m *Manager[T]) configured(name string) (T, error) {
	section, ok := m.sections[name]
	if !ok {
		var zero T

		return zero, fmt.Errorf("%s %q: %w", m.kind, name, ErrUnknownInstance)
	}

	return m.make(name, section)
}

/*
Build returns a new instance from a section the configuration does not
list, through the driver it names, for instances whose settings are
data rather than configuration. The instance is not kept: every call
builds a fresh one.
*/
func (m *Manager[T]) Build(name string, section config.Section) (T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.make(name, section)
}

/*
make builds an instance from the section through its driver. The caller
holds the lock.
*/
func (m *Manager[T]) make(name string, section config.Section) (T, error) {
	var zero T

	driver := section.String("driver")

	factory, ok := m.factories[driver]
	if !ok {
		return zero, fmt.Errorf("%s %q driver %q: %w", m.kind, name, driver, ErrUnknownDriver)
	}

	instance, err := factory(name, section)
	if err != nil {
		return zero, fmt.Errorf("build %s %q: %w", m.kind, name, err)
	}

	return instance, nil
}
