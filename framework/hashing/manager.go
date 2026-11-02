/*
|--------------------------------------------------------------------------
| Hasher Manager
|--------------------------------------------------------------------------
|
| Hashers are declared in configuration by name, each naming its driver.
| New passwords are hashed by the default one. A password stored by
| another configured hasher, such as bcrypt hashes brought over from an
| import, still checks, and NeedsRehash then asks for a hash made by the
| default.
|
|   hashing:
|     default: argon2id
|     concurrency: 4
|     hashers:
|       argon2id:
|         driver: argon2id
|       imported:
|         driver: bcrypt
|
| Every password is NFKC-normalized before hashing, so the same password
| typed on two keyboards that encode accents differently still matches.
| Password hashing is deliberately expensive in CPU and memory, so only a
| bounded number of hashes run at once and a bounded number wait; past
| that, callers get ErrBusy instead of piling up until the server runs
| out of memory.
|
*/

package hashing

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"

	"golang.org/x/sync/semaphore"
	"golang.org/x/text/unicode/norm"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Config lists the named hashers and which one makes new hashes.

Concurrency is how many hashes may run at once; zero means one per CPU.
Queue is how many more callers may wait for a slot; zero means four per
running slot.
*/
type Config struct {
	Default     string                    `koanf:"default"`
	Concurrency int                       `koanf:"concurrency"`
	Queue       int                       `koanf:"queue"`
	Hashers     map[string]config.Section `koanf:"hashers"`
}

/*
Manager resolves named hashers and is itself a Hasher that makes hashes
with the default one.
*/
type Manager struct {
	hashers *manager.Manager[Hasher]
	limit   *limiter
}

var _ Hasher = (*Manager)(nil)

/*
NewManager returns a manager for the hashers in the configuration.
*/
func NewManager(cfg Config) *Manager {
	running := cfg.Concurrency
	if running <= 0 {
		running = runtime.GOMAXPROCS(0)
	}

	queue := cfg.Queue
	if queue <= 0 {
		queue = 4 * running
	}

	return &Manager{
		hashers: manager.New[Hasher]("hasher", cfg.Hashers, cfg.Default),
		limit: &limiter{
			admitted: semaphore.NewWeighted(int64(running + queue)),
			running:  semaphore.NewWeighted(int64(running)),
		},
	}
}

/*
Extend registers the factory for a driver name.
*/
func (m *Manager) Extend(driver string, factory manager.Factory[Hasher]) {
	m.hashers.Extend(driver, factory)
}

/*
Hasher returns the named hasher, normalizing passwords and sharing the
manager's concurrency limit.
*/
func (m *Manager) Hasher(name string) (Hasher, error) {
	hasher, err := m.hashers.Instance(name)
	if err != nil {
		return nil, err
	}

	return &guarded{hasher: hasher, limit: m.limit}, nil
}

/*
Default returns the default hasher.
*/
func (m *Manager) Default() (Hasher, error) {
	return m.Hasher(m.hashers.DefaultName())
}

/*
Make hashes the password with the default hasher.
*/
func (m *Manager) Make(ctx context.Context, password string) (string, error) {
	hasher, err := m.Default()
	if err != nil {
		return "", err
	}

	return hasher.Make(ctx, password)
}

/*
Check reports whether the password matches the hash. A hash the default
hasher cannot read is tried with every other configured hasher, in name
order.
*/
func (m *Manager) Check(ctx context.Context, password, hash string) (bool, error) {
	names := m.hashers.Names()
	slices.Sort(names)
	names = slices.Insert(slices.DeleteFunc(names, m.isDefault), 0, m.hashers.DefaultName())

	for _, name := range names {
		hasher, err := m.Hasher(name)
		if err != nil {
			return false, err
		}

		ok, err := hasher.Check(ctx, password, hash)
		if errors.Is(err, ErrInvalidHash) {
			continue
		}

		return ok, err
	}

	return false, ErrInvalidHash
}

/*
NeedsRehash reports whether the hash was not made by the default hasher
with its current settings. It reports false when the default hasher
cannot be built, since no better hash could be made.
*/
func (m *Manager) NeedsRehash(hash string) bool {
	hasher, err := m.Default()
	if err != nil {
		return false
	}

	return hasher.NeedsRehash(hash)
}

func (m *Manager) isDefault(name string) bool {
	return name == m.hashers.DefaultName()
}

/*
guarded wraps a driver with NFKC normalization and the shared limit.
*/
type guarded struct {
	hasher Hasher
	limit  *limiter
}

/*
Make hashes the normalized password once a slot is free.
*/
func (g *guarded) Make(ctx context.Context, password string) (string, error) {
	var hash string

	err := g.limit.run(ctx, func() error {
		var err error
		hash, err = g.hasher.Make(ctx, norm.NFKC.String(password))

		return err
	})

	return hash, err
}

/*
Check checks the normalized password once a slot is free.
*/
func (g *guarded) Check(ctx context.Context, password, hash string) (bool, error) {
	var ok bool

	err := g.limit.run(ctx, func() error {
		var err error
		ok, err = g.hasher.Check(ctx, norm.NFKC.String(password), hash)

		return err
	})

	return ok, err
}

/*
NeedsRehash asks the driver; reading a hash needs no slot.
*/
func (g *guarded) NeedsRehash(hash string) bool {
	return g.hasher.NeedsRehash(hash)
}

/*
limiter admits a bounded number of callers and lets a smaller number of
them run at once. A caller that finds no room returns ErrBusy at once.
*/
type limiter struct {
	admitted *semaphore.Weighted
	running  *semaphore.Weighted
}

func (l *limiter) run(ctx context.Context, fn func() error) error {
	if !l.admitted.TryAcquire(1) {
		return ErrBusy
	}
	defer l.admitted.Release(1)

	if err := l.running.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("wait for a hashing slot: %w", err)
	}
	defer l.running.Release(1)

	return fn()
}
