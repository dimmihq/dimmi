/*
|--------------------------------------------------------------------------
| Disk Manager
|--------------------------------------------------------------------------
|
| Disks are declared in configuration by name, each naming its driver.
| The manager opens a disk the first time it is asked for and hands back
| the same disk afterwards. The framework registers the local and s3
| drivers; any other store plugs in through Extend.
|
|   filesystems:
|     default: attachments
|     disks:
|       attachments:
|         driver: s3
|         bucket: uploads
|
*/

package filesystem

import (
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Config lists the named disks and which one is the default.
*/
type Config struct {
	Default string                    `koanf:"default"`
	Disks   map[string]config.Section `koanf:"disks"`
}

/*
Manager resolves named disks.
*/
type Manager struct {
	disks *manager.Manager[Filesystem]
}

/*
NewManager returns a manager for the disks in the configuration.
*/
func NewManager(cfg Config) *Manager {
	return &Manager{disks: manager.New[Filesystem]("disk", cfg.Disks, cfg.Default)}
}

/*
Extend registers the factory for a driver name.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[Filesystem]) {
	m.disks.Extend(driver, factory)
}

/*
Disk returns the named disk.
*/
func (m *Manager) Disk(name string) (Filesystem, error) {
	return m.disks.Instance(name)
}

/*
Default returns the default disk.
*/
func (m *Manager) Default() (Filesystem, error) {
	return m.disks.Default()
}

/*
Build returns a fresh disk from a section the configuration does not
list, such as a directory named in a setting. The disk is not kept;
the caller closes it.
*/
func (m *Manager) Build(name string, section config.Section) (Filesystem, error) {
	return m.disks.Build(name, section)
}
