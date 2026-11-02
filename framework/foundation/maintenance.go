/*
|--------------------------------------------------------------------------
| Maintenance Mode
|--------------------------------------------------------------------------
|
| An operator takes the application down for maintenance with one
| command, on whichever server they are logged in to, and every replica
| starts answering 503 within moments. The state is kept in a file under
| the data directory, so a server that restarts stays down, and every
| change is announced over Valkey, where each replica updates its own
| copy and its own file. Reading the state never leaves the process, so
| checking it on every request costs nothing.
|
*/

package foundation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/log"
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
MaintenanceStore keeps the state in a file and announces changes on the
bus. It is safe for concurrent use. Close it to stop listening.
*/
type MaintenanceStore struct {
	path        string
	bus         *pubsub.Bus
	clock       clock.Clock
	logger      log.Logger
	unsubscribe pubsub.Unsubscribe

	mu    sync.RWMutex
	state MaintenanceState
}

var _ Maintenance = (*MaintenanceStore)(nil)

/*
NewMaintenanceStore reads the state from the file at the path and
starts listening for changes announced on the bus.
*/
func NewMaintenanceStore(
	ctx context.Context, path string, bus *pubsub.Bus, clk clock.Clock,
) (*MaintenanceStore, error) {
	s := &MaintenanceStore{path: path, bus: bus, clock: clk, logger: log.From(ctx)}

	state, err := s.read()
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
change applies the state here and announces it to every replica.
*/
func (s *MaintenanceStore) change(ctx context.Context, state MaintenanceState) error {
	if err := s.apply(state); err != nil {
		return err
	}

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
receive applies a state another replica announced. The bus hands over no
context, so a failure is logged without request fields.
*/
func (s *MaintenanceStore) receive(payload []byte) {
	var state MaintenanceState
	if err := json.Unmarshal(payload, &state); err != nil {
		return
	}

	if err := s.apply(state); err != nil {
		s.logger.ErrorContext(context.Background(), "maintenance state not stored", log.Err(err))
	}
}

/*
apply keeps the state in memory and in the file, removing the file when
the application is up.
*/
func (s *MaintenanceStore) apply(state MaintenanceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = state

	if !state.Enabled {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove maintenance file: %w", err)
		}

		return nil
	}

	return s.write(state)
}

/*
write replaces the file in one step through a temporary file of its
own, so processes sharing the data directory never read half a state.
*/
func (s *MaintenanceStore) write(state MaintenanceState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode maintenance state: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create maintenance directory: %w", err)
	}

	temp, err := os.CreateTemp(dir, ".down-*")
	if err != nil {
		return fmt.Errorf("write maintenance file: %w", err)
	}

	_, err = temp.Write(data)
	if err = errors.Join(err, temp.Close()); err != nil {
		return fmt.Errorf("write maintenance file: %w", errors.Join(err, os.Remove(temp.Name())))
	}

	if err := os.Rename(temp.Name(), s.path); err != nil {
		return fmt.Errorf("replace maintenance file: %w", err)
	}

	return nil
}

/*
read returns the state stored in the file, or up when there is no file.
*/
func (s *MaintenanceStore) read() (MaintenanceState, error) {
	data, err := os.ReadFile(filepath.Clean(s.path))
	if errors.Is(err, fs.ErrNotExist) {
		return MaintenanceState{}, nil
	}

	if err != nil {
		return MaintenanceState{}, fmt.Errorf("read maintenance file: %w", err)
	}

	var state MaintenanceState
	if err := json.Unmarshal(data, &state); err != nil {
		return MaintenanceState{}, fmt.Errorf("decode maintenance file: %w", err)
	}

	return state, nil
}
