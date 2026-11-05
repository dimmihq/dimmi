/*
|--------------------------------------------------------------------------
| Maintenance Mode
|--------------------------------------------------------------------------
|
| An operator takes the application down for maintenance with one
| command, on whichever server they are logged in to, and every replica
| starts answering 503 within moments. The state is kept in PostgreSQL,
| so a replica that starts or restarts stays down, whatever disk it has,
| and every change is announced over Valkey, where each replica updates
| its own copy. Reading the state never leaves the process, so checking
| it on every request costs nothing.
|
*/

package foundation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/pubsub"
)

/*
maintenanceTopic is the topic every replica hears maintenance changes
on.
*/
const maintenanceTopic = "maintenance"

/*
MaintenanceConfig configures maintenance mode. RetryAfter is how long
clients are told to wait before trying again.
*/
type MaintenanceConfig struct {
	RetryAfter time.Duration `default:"60s" koanf:"retry_after"`
}

/*
MaintenanceState is whether the application is down, why, and since
when.
*/
type MaintenanceState struct {
	Enabled bool      `json:"enabled"`
	Message string    `json:"message,omitempty"`
	Since   time.Time `json:"since"`
}

/*
Maintenance holds the maintenance state of every replica. Active
satisfies the HTTP maintenance middleware.
*/
type Maintenance interface {
	/*
		Active reports whether the application is down.
	*/
	Active(ctx context.Context) (bool, error)

	/*
		State returns the current state.
	*/
	State(ctx context.Context) (MaintenanceState, error)

	/*
		Down takes every replica down with the message.
	*/
	Down(ctx context.Context, message string) error

	/*
		Up brings every replica back.
	*/
	Up(ctx context.Context) error
}

/*
MaintenanceStore keeps the state in the database and announces changes
on the bus. It is safe for concurrent use. Close it to stop listening.
*/
type MaintenanceStore struct {
	db          *database.DB
	bus         *pubsub.Bus
	clock       clock.Clock
	unsubscribe pubsub.Unsubscribe

	mu    sync.RWMutex
	state MaintenanceState
}

var _ Maintenance = (*MaintenanceStore)(nil)

/*
NewMaintenanceStore reads the state from the database and starts
listening for changes announced on the bus.
*/
func NewMaintenanceStore(
	ctx context.Context, db *database.DB, bus *pubsub.Bus, clk clock.Clock,
) (*MaintenanceStore, error) {
	s := &MaintenanceStore{db: db, bus: bus, clock: clk}

	state, err := s.read(ctx)
	if err != nil {
		return nil, err
	}

	s.state = state

	s.unsubscribe, err = bus.Subscribe(context.WithoutCancel(ctx), maintenanceTopic, s.receive)
	if err != nil {
		return nil, fmt.Errorf("listen for maintenance changes: %w", err)
	}

	return s, nil
}

/*
Active reports whether the application is down.
*/
func (s *MaintenanceStore) Active(context.Context) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.state.Enabled, nil
}

/*
State returns the current state.
*/
func (s *MaintenanceStore) State(context.Context) (MaintenanceState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.state, nil
}

/*
Down takes every replica down with the message.
*/
func (s *MaintenanceStore) Down(ctx context.Context, message string) error {
	return s.change(ctx, MaintenanceState{Enabled: true, Message: message, Since: s.clock.Now().UTC()})
}

/*
Up brings every replica back.
*/
func (s *MaintenanceStore) Up(ctx context.Context) error {
	return s.change(ctx, MaintenanceState{})
}

/*
Close stops listening for changes.
*/
func (s *MaintenanceStore) Close() {
	s.unsubscribe()
}

/*
change stores the state, applies it here and announces it to every
replica, which only update their copy.
*/
func (s *MaintenanceStore) change(ctx context.Context, state MaintenanceState) error {
	since := &state.Since
	if !state.Enabled {
		since = nil
	}

	_, err := s.db.Exec(ctx, `INSERT INTO maintenance (id, enabled, message, since) VALUES (true, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET enabled = EXCLUDED.enabled, message = EXCLUDED.message, since = EXCLUDED.since`,
		state.Enabled, state.Message, since)
	if err != nil {
		return fmt.Errorf("store maintenance state: %w", err)
	}

	s.apply(state)

	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode maintenance state: %w", err)
	}

	if err := s.bus.Publish(ctx, maintenanceTopic, payload); err != nil {
		return fmt.Errorf("announce maintenance state: %w", err)
	}

	return nil
}

/*
receive applies a state another replica announced and stored.
*/
func (s *MaintenanceStore) receive(payload []byte) {
	var state MaintenanceState
	if err := json.Unmarshal(payload, &state); err != nil {
		return
	}

	s.apply(state)
}

/*
apply keeps the state in memory.
*/
func (s *MaintenanceStore) apply(state MaintenanceState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = state
}

/*
read returns the stored state, or up when none was ever stored. A
database not yet migrated has no maintenance table and so was never
taken down; the process still boots, so an operator can migrate it.
*/
func (s *MaintenanceStore) read(ctx context.Context) (MaintenanceState, error) {
	var (
		state   MaintenanceState
		since   *time.Time
		created bool
	)

	err := s.db.QueryRow(ctx, "SELECT to_regclass('maintenance') IS NOT NULL").Scan(&created)
	if err != nil {
		return MaintenanceState{}, fmt.Errorf("find maintenance state: %w", err)
	}

	if !created {
		return MaintenanceState{}, nil
	}

	err = s.db.QueryRow(ctx, "SELECT enabled, message, since FROM maintenance WHERE id").
		Scan(&state.Enabled, &state.Message, &since)
	if errors.Is(err, database.ErrNotFound) {
		return MaintenanceState{}, nil
	}

	if err != nil {
		return MaintenanceState{}, fmt.Errorf("read maintenance state: %w", err)
	}

	if since != nil {
		state.Since = since.UTC()
	}

	return state, nil
}
