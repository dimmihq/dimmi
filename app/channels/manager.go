/*
|--------------------------------------------------------------------------
| Channel Manager
|--------------------------------------------------------------------------
|
| The inbox finds the driver of a connection by the connection's type.
| Drivers are registered while the application boots, one per type; a
| second driver for a type is a mistake that stops the boot rather than
| silently replacing the first. Tests add a fake driver that records
| what it sends; it exists only in the testing environment.
|
*/

package channels

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/fake"
)

/*
Errors the manager returns.

ErrDuplicateDriver means a driver for the type is already registered.
ErrUnknownChannel means no driver serves the connection type.
ErrFakesNotAllowed refuses the fake driver outside the testing
environment.
*/
var (
	ErrDuplicateDriver = errors.New("channels: driver already registered")
	ErrUnknownChannel  = errors.New("channels: no driver for the connection type")
	ErrFakesNotAllowed = errors.New("channels: fake drivers only exist in the testing environment")
)

/*
Manager holds the registered channel drivers by connection type, and
whether the fake driver may be added.
*/
type Manager struct {
	mu         sync.RWMutex
	drivers    map[string]contracts.Channel
	allowFakes bool
}

/*
NewManager returns a manager with no drivers. Only a manager of the
testing environment hands out the fake driver.
*/
func NewManager(allowFakes bool) *Manager {
	return &Manager{drivers: map[string]contracts.Channel{}, allowFakes: allowFakes}
}

/*
Fake registers the fake driver for the fake connection type and returns
it, the same one on every call.
*/
func (m *Manager) Fake() (*fake.Driver, error) {
	if !m.allowFakes {
		return nil, ErrFakesNotAllowed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if driver, ok := m.drivers[fake.Type].(*fake.Driver); ok {
		return driver, nil
	}

	driver := &fake.Driver{}
	m.drivers[fake.Type] = driver

	return driver, nil
}

/*
Register adds the drivers, refusing a type that already has one.
*/
func (m *Manager) Register(drivers ...contracts.Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, driver := range drivers {
		if _, taken := m.drivers[driver.Type()]; taken {
			return fmt.Errorf("%w: %s", ErrDuplicateDriver, driver.Type())
		}

		m.drivers[driver.Type()] = driver
	}

	return nil
}

/*
Driver returns the driver of the connection type.
*/
func (m *Manager) Driver(connectionType string) (contracts.Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	driver, ok := m.drivers[connectionType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownChannel, connectionType)
	}

	return driver, nil
}
