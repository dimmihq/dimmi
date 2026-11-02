/*
|--------------------------------------------------------------------------
| KEK Manager
|--------------------------------------------------------------------------
|
| KEKs are declared in configuration by name, each naming its driver. One
| is current and wraps every data key from now on; the previous ones only
| unwrap, until RewrapAll has moved every stored key to the current one.
| The application registers the local driver with its app key and, in
| the cloud, the awskms driver.
|
|   encryption:
|     kek: kms
|     previous: [local]
|     keks:
|       kms:
|         driver: awskms
|         key_arn: arn:aws:kms:eu-west-1:123456789012:key/abc
|       local:
|         driver: local
|
*/

package encryption

import (
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Config names the current KEK, the previous ones and every KEK's
section.
*/
type Config struct {
	KEK      string                    `koanf:"kek"`
	Previous []string                  `koanf:"previous"`
	KEKs     map[string]config.Section `koanf:"keks"`
}

/*
Manager resolves named KEKs.
*/
type Manager struct {
	keks     *manager.Manager[KEK]
	previous []string
}

/*
NewManager returns a manager for the KEKs in the configuration.
*/
func NewManager(cfg Config) *Manager {
	return &Manager{keks: manager.New[KEK]("kek", cfg.KEKs, cfg.KEK), previous: cfg.Previous}
}

/*
Extend registers the factory for a driver name.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[KEK]) {
	m.keks.Extend(driver, factory)
}

/*
KEK returns the named KEK.
*/
func (m *Manager) KEK(name string) (KEK, error) {
	return m.keks.Instance(name)
}

/*
Keys returns a key manager that wraps with the current KEK, unwraps with
it or any previous one, and keeps its keysets in the store.
*/
func (m *Manager) Keys(store KeysetStore) (*Keys, error) {
	current, err := m.keks.Default()
	if err != nil {
		return nil, err
	}

	previous := make([]KEK, 0, len(m.previous))

	for _, name := range m.previous {
		kek, err := m.keks.Instance(name)
		if err != nil {
			return nil, err
		}

		previous = append(previous, kek)
	}

	return NewKeys(KeysOptions{KEK: current, Previous: previous, Store: store})
}
