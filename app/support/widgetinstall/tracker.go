/*
|--------------------------------------------------------------------------
| Install Tracker
|--------------------------------------------------------------------------
|
| "Installed, last seen on https://app.acme.com" must work without a request
| of its own, and every page load already asks for the config, so the origin
| check notes each request here. That note is a map write: the config's
| millisecond budget has no room for a database. A replica writes down what
| it saw once a minute and when it shuts down, so a thousand page loads are
| one write, and a crash loses a minute of what is only a hint.
|
| Sites seen and blocked live in the cache, not a table: they are rebuilt by
| the next request, and old ones simply expire. Requests with no Origin count
| as the origin "app", which is what native apps send.
|
*/

package widgetinstall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
AppOrigin is what a request without an Origin header counts as.
*/
const AppOrigin = "app"

/*
Sites keep this many origins each for this long.
*/
const (
	maxSites = 50
	sitesTTL = 7 * 24 * time.Hour
)

/*
seen is what a replica noticed about one widget since its last flush.
*/
type seen struct {
	workspaceID ids.UUID
	at          time.Time
	origin      string
	allowed     map[string]struct{}
	blocked     map[string]struct{}
}

/*
Tracker collects sightings in memory and flushes them.
*/
type Tracker struct {
	db    *database.DB
	cache *cache.Store
	clock clock.Clock
	mu    sync.Mutex
	seen  map[ids.UUID]*seen
}

/*
New returns an empty tracker.
*/
func New(db *database.DB, store *cache.Store, clk clock.Clock) *Tracker {
	return &Tracker{db: db, cache: store, clock: clk, seen: map[ids.UUID]*seen{}}
}

/*
Visit is one request to a widget: the site it came from, blank for a client
without one, and whether the widget let it in.
*/
type Visit struct {
	WorkspaceID ids.UUID
	WidgetID    ids.UUID
	Origin      string
	Allowed     bool
}

/*
Record notes one visit in memory.
*/
func (t *Tracker) Record(v Visit) {
	workspaceID, widgetID, origin := v.WorkspaceID, v.WidgetID, v.Origin
	if origin == "" {
		origin = AppOrigin
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.seen[widgetID]
	if !ok {
		entry = &seen{workspaceID: workspaceID, allowed: map[string]struct{}{}, blocked: map[string]struct{}{}}
		t.seen[widgetID] = entry
	}

	if !v.Allowed {
		entry.blocked[origin] = struct{}{}

		return
	}

	entry.at, entry.origin = t.clock.Now(), origin
	entry.allowed[origin] = struct{}{}
}

/*
Flush writes down what was noticed. What fails to write is logged and
dropped, since it is only a hint and the next request brings it back.
*/
func (t *Tracker) Flush(ctx context.Context) error {
	t.mu.Lock()
	batch := t.seen
	t.seen = map[ids.UUID]*seen{}
	t.mu.Unlock()

	var errs []error

	for id, entry := range batch {
		errs = append(errs, t.write(ctx, id, entry))
	}

	return errors.Join(errs...)
}

/*
write records one widget's sightings.
*/
func (t *Tracker) write(ctx context.Context, id ids.UUID, entry *seen) error {
	if err := t.mergeSites(ctx, seenKey(id), entry.allowed); err != nil {
		return err
	}

	if err := t.mergeSites(ctx, blockedKey(id), entry.blocked); err != nil {
		return err
	}

	if entry.origin == "" {
		return nil
	}

	scoped := tenancy.With(ctx, entry.workspaceID.String())

	err := t.db.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).UpdateWidgetLastBoot(ctx, models.UpdateWidgetLastBootParams{
			SeenAt: entry.at, Origin: entry.origin, WorkspaceID: entry.workspaceID, ID: id,
		})
	})
	if err != nil {
		return fmt.Errorf("record widget last seen: %w", err)
	}

	return nil
}

/*
mergeSites adds origins to the front of a widget's list.
*/
func (t *Tracker) mergeSites(ctx context.Context, key string, origins map[string]struct{}) error {
	if len(origins) == 0 {
		return nil
	}

	current, err := t.readSites(ctx, key)
	if err != nil {
		return err
	}

	next := make([]string, 0, len(origins)+len(current))
	for origin := range origins {
		next = append(next, origin)
	}

	slices.Sort(next)

	for _, origin := range current {
		if !slices.Contains(next, origin) {
			next = append(next, origin)
		}
	}

	if len(next) > maxSites {
		next = next[:maxSites]
	}

	data, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("encode widget sites: %w", err)
	}

	if err := t.cache.Set(ctx, key, data, sitesTTL); err != nil {
		return fmt.Errorf("store widget sites: %w", err)
	}

	return nil
}

/*
readSites returns a widget's list, empty when there is none.
*/
func (t *Tracker) readSites(ctx context.Context, key string) ([]string, error) {
	data, err := t.cache.Get(ctx, key)
	if errors.Is(err, cache.ErrMiss) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read widget sites: %w", err)
	}

	var sites []string
	if err := json.Unmarshal(data, &sites); err != nil {
		return nil, fmt.Errorf("read widget sites: %w", err)
	}

	return sites, nil
}

/*
Sites returns the origins a widget was seen on and those it blocked, newest
first.
*/
func (t *Tracker) Sites(ctx context.Context, id ids.UUID) (seenOn, blocked []string, err error) {
	if seenOn, err = t.readSites(ctx, seenKey(id)); err != nil {
		return nil, nil, err
	}

	if blocked, err = t.readSites(ctx, blockedKey(id)); err != nil {
		return nil, nil, err
	}

	return seenOn, blocked, nil
}

/*
Allow drops an origin from a widget's blocked list, once it has been added to
the allowed sites.
*/
func (t *Tracker) Allow(ctx context.Context, id ids.UUID, origins []string) error {
	key := blockedKey(id)

	current, err := t.readSites(ctx, key)
	if err != nil || len(current) == 0 {
		return err
	}

	kept := slices.DeleteFunc(current, func(origin string) bool { return slices.Contains(origins, origin) })

	data, err := json.Marshal(kept)
	if err != nil {
		return fmt.Errorf("encode widget sites: %w", err)
	}

	if err := t.cache.Set(ctx, key, data, sitesTTL); err != nil {
		return fmt.Errorf("store widget sites: %w", err)
	}

	return nil
}

/*
Start flushes on the interval until the returned function is called, which
stops the timer and flushes once more.
*/
func (t *Tracker) Start(ctx context.Context, interval time.Duration) func(context.Context) error {
	done, stopped := make(chan struct{}), make(chan struct{})

	go func() {
		defer close(stopped)

		ticker := t.clock.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.Chan():
				if err := t.Flush(ctx); err != nil {
					log.From(ctx).WarnContext(ctx, "flush widget install tracker", log.Err(err))
				}
			}
		}
	}()

	return func(ctx context.Context) error {
		close(done)
		<-stopped

		return t.Flush(ctx)
	}
}

/*
seenKey names a widget's list of sites it was used from.
*/
func seenKey(id ids.UUID) string {
	return "widget:origins:" + id.String()
}

/*
blockedKey names a widget's list of sites it refused.
*/
func blockedKey(id ids.UUID) string {
	return "widget:origins_blocked:" + id.String()
}
