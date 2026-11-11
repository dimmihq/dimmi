/*
|--------------------------------------------------------------------------
| Tracking Events
|--------------------------------------------------------------------------
|
| The team's systems report what customers did: one event or up to 100
| at once, each naming its person by id, external id or email. Each is
| added to that person's summary of the event in one transaction, so a
| rule sees it on its next evaluation. An event may have happened up to
| 30 days ago; one dated earlier, or more than an hour ahead, is counted
| at the nearest allowed time and says so. A retried event with the same
| idempotency key within 48 hours is counted once. A new name is defined
| on its first event, up to 1,000 per workspace; an archived name takes
| none. Events are never metered.
|
*/

package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	fevents "github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/eventroll"
)

/*
Limits of tracking.
*/
const (
	MaxBatch       = 100
	MaxDefinitions = 1000
	maxMetadata    = 20
	maxMetadataLen = 2048
	backLimit      = 30 * 24 * time.Hour
	aheadLimit     = time.Hour
	idempotency    = 48 * time.Hour
)

/*
The outcomes of one event.
*/
const (
	StatusAccepted        = "accepted"
	StatusDuplicate       = "duplicate"
	StatusContactNotFound = "contact_not_found"
	StatusArchived        = "archived"
	StatusLimitReached    = "event_limit_reached"
)

var eventName = regexp.MustCompile(`^[a-z0-9_.:-]{1,64}$`)

/*
The failures of tracking.
*/
var (
	ErrContactNotFound = fault.New(fault.NotFound, "contact_not_found", "No contact matches this event.")
	ErrInvalid         = fault.New(fault.Invalid, "event_invalid", "Some events cannot be recorded as sent.")
)

/*
Events records events and manages their names. Dispatcher, when set,
hears about every event recorded.
*/
type Events struct {
	DB         *database.DB
	Cache      *cache.Store
	Clock      clock.Clock
	IDs        *ids.Registry
	Dispatcher *fevents.Dispatcher
}

/*
Event is one thing a customer did. Exactly one of ContactID, ExternalID
and Email names the person.
*/
type Event struct {
	Name           string
	ContactID      *ids.UUID
	ExternalID     *string
	Email          *string
	OccurredAt     *time.Time
	Metadata       map[string]any
	IdempotencyKey string
}

/*
Result is what became of one event, in the order sent. Clamped is set
when its time was moved into the allowed range.
*/
type Result struct {
	Status    string
	ContactID ids.UUID
	Clamped   bool
}

/*
Issue is why one event of a batch cannot be recorded.
*/
type Issue struct {
	Index   int    `json:"index"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

/*
Track records events. A single event naming nobody the workspace knows
is refused; in a batch, it is reported in its result.
*/
func (e *Events) Track(ctx context.Context, m authz.Membership, batch []Event) ([]Result, error) {
	if !m.Can("contacts:write") {
		return nil, exceptions.ErrForbidden
	}

	if issues := check(batch); len(issues) > 0 {
		return nil, ErrInvalid.With("errors", issues)
	}

	now := e.Clock.Now()
	results := make([]Result, len(batch))
	fresh, err := e.firstSeen(ctx, m.Workspace.ID, batch)
	if err != nil {
		return nil, err
	}

	err = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		r := run{
			q: models.New(tx), tx: tx, dispatcher: e.Dispatcher, ws: m.Workspace.ID, now: now,
			defs: map[string]*models.EventDefinition{},
		}
		for i, ev := range batch {
			if !fresh[i] {
				results[i] = Result{Status: StatusDuplicate}
				continue
			}

			if results[i], err = r.record(ctx, ev); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		e.forget(ctx, m.Workspace.ID, batch)
		return nil, err
	}

	if len(batch) == 1 && results[0].Status == StatusContactNotFound {
		return nil, ErrContactNotFound
	}

	return results, nil
}

/*
check refuses names, people and details that cannot be recorded.
*/
func check(batch []Event) []Issue {
	var issues []Issue
	add := func(i int, field, message string) {
		issues = append(issues, Issue{Index: i, Field: field, Message: message})
	}

	for i, ev := range batch {
		switch {
		case eventName.MatchString(ev.Name):
		case strings.ToLower(ev.Name) != ev.Name && eventName.MatchString(strings.ToLower(ev.Name)):
			add(i, "name", "Event names are lowercase: send "+strings.ToLower(ev.Name)+".")
		default:
			add(i, "name", "Event names use 1 to 64 lowercase letters, digits and _ . : -.")
		}

		if people(ev) != 1 {
			add(i, "contact", "Name the person by exactly one of contact_id, external_id and email.")
		}

		if !flat(ev.Metadata) {
			add(i, "metadata", "Metadata holds at most 20 keys of text, numbers or yes/no, 2 KB in all.")
		}
	}

	return issues
}

func people(ev Event) int {
	n := 0
	for _, set := range []bool{ev.ContactID != nil, ev.ExternalID != nil, ev.Email != nil} {
		if set {
			n++
		}
	}

	return n
}

/*
flat reports whether metadata is small and holds only plain values.
*/
func flat(meta map[string]any) bool {
	if len(meta) > maxMetadata {
		return false
	}

	for _, v := range meta {
		switch v.(type) {
		case string, float64, json.Number, bool, nil:
		default:
			return false
		}
	}

	raw, _ := json.Marshal(meta)

	return len(raw) <= maxMetadataLen
}

/*
firstSeen marks the events whose idempotency key was not used in the
last 48 hours. Events without a key are always new.
*/
func (e *Events) firstSeen(ctx context.Context, ws ids.UUID, batch []Event) ([]bool, error) {
	out := make([]bool, len(batch))
	for i, ev := range batch {
		if ev.IdempotencyKey == "" || e.Cache == nil {
			out[i] = true
			continue
		}

		added, err := e.Cache.Add(ctx, idempotencyKey(ws, ev.IdempotencyKey), []byte("1"), idempotency)
		if err != nil {
			return nil, fmt.Errorf("check event idempotency: %w", err)
		}

		out[i] = added
	}

	return out, nil
}

/*
forget releases the idempotency keys of a batch that was not recorded,
so a retry counts it.
*/
func (e *Events) forget(ctx context.Context, ws ids.UUID, batch []Event) {
	if e.Cache == nil {
		return
	}

	for _, ev := range batch {
		if ev.IdempotencyKey != "" {
			_ = e.Cache.Delete(ctx, idempotencyKey(ws, ev.IdempotencyKey))
		}
	}
}

func idempotencyKey(ws ids.UUID, key string) string {
	return "events:idempotency:" + ws.String() + ":" + key
}

/*
run is a batch being recorded in one transaction.
*/
type run struct {
	q          *models.Queries
	tx         *database.Tx
	dispatcher *fevents.Dispatcher
	ws         ids.UUID
	now        time.Time
	defs       map[string]*models.EventDefinition
}

/*
record adds one event to its person's summary.
*/
func (r run) record(ctx context.Context, ev Event) (Result, error) {
	contact, found, err := r.person(ctx, ev)
	if err != nil || !found {
		return Result{Status: StatusContactNotFound}, err
	}

	def, status, err := r.definition(ctx, ev.Name)
	if err != nil || status != "" {
		return Result{Status: status, ContactID: contact}, err
	}

	at, clamped := r.when(ev.OccurredAt)
	if err := r.summarize(ctx, entry{contact: contact, def: def, at: at, meta: ev.Metadata}); err != nil {
		return Result{}, err
	}

	err = r.q.CountEventDefinition(ctx, models.CountEventDefinitionParams{
		Added: 1, Seen: at, WorkspaceID: r.ws, ID: def.ID,
	})
	if err != nil {
		return Result{}, fmt.Errorf("count event: %w", err)
	}

	if err := r.tracked(ctx, contact, ev); err != nil {
		return Result{}, err
	}

	return Result{Status: StatusAccepted, ContactID: contact, Clamped: clamped}, nil
}

/*
tracked tells the rest of the app the contact did the event.
*/
func (r run) tracked(ctx context.Context, contact ids.UUID, ev Event) error {
	if r.dispatcher == nil {
		return nil
	}

	ref := ev.IdempotencyKey
	if ref == "" {
		ref = ids.New().String()
	}

	return r.dispatcher.Dispatch(ctx, r.tx, appevents.ContactEventTracked{
		WorkspaceID: r.ws, ContactID: contact, Name: ev.Name, Ref: ref,
	})
}

/*
when is the event's time, moved into the allowed range.
*/
func (r run) when(at *time.Time) (time.Time, bool) {
	switch {
	case at == nil:
		return r.now, false
	case at.Before(r.now.Add(-backLimit)):
		return r.now.Add(-backLimit), true
	case at.After(r.now.Add(aheadLimit)):
		return r.now, true
	}

	return *at, false
}

/*
person finds the live contact an event names.
*/
func (r run) person(ctx context.Context, ev Event) (ids.UUID, bool, error) {
	var c models.Contact
	var err error

	switch {
	case ev.ContactID != nil:
		c, err = r.q.FindContact(ctx, models.FindContactParams{WorkspaceID: r.ws, ID: *ev.ContactID})
	case ev.ExternalID != nil:
		c, err = r.q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: r.ws, ExternalID: ev.ExternalID,
		})
	default:
		email := strings.ToLower(strings.TrimSpace(*ev.Email))
		c, err = r.q.FindEventContactByEmail(ctx, models.FindEventContactByEmailParams{
			WorkspaceID: r.ws, Email: &email,
		})
	}

	if errors.Is(err, database.ErrNotFound) {
		return ids.Nil(), false, nil
	}

	if err != nil {
		return ids.Nil(), false, fmt.Errorf("find event contact: %w", err)
	}

	if c.MergedIntoID != nil {
		return *c.MergedIntoID, true, nil
	}

	return c.ID, c.DeletedAt == nil, nil
}

/*
definition finds an event's name, defining it on its first event.
*/
func (r run) definition(ctx context.Context, name string) (models.EventDefinition, string, error) {
	if def, ok := r.defs[name]; ok {
		return *def, statusOf(*def), nil
	}

	def, err := r.q.FindEventDefinitionByName(ctx, models.FindEventDefinitionByNameParams{
		WorkspaceID: r.ws, Name: name,
	})
	if errors.Is(err, database.ErrNotFound) {
		return r.define(ctx, name)
	}

	if err != nil {
		return def, "", fmt.Errorf("find event definition: %w", err)
	}

	r.defs[name] = &def

	return def, statusOf(def), nil
}

/*
define makes a name's definition on its first event.
*/
func (r run) define(ctx context.Context, name string) (models.EventDefinition, string, error) {
	active, err := r.q.CountActiveEventDefinitions(ctx, r.ws)
	if err != nil {
		return models.EventDefinition{}, "", fmt.Errorf("count event definitions: %w", err)
	}

	if active >= MaxDefinitions {
		return models.EventDefinition{}, StatusLimitReached, nil
	}

	def, err := r.q.CreateEventDefinition(ctx, models.CreateEventDefinitionParams{
		ID: ids.NewV7At(r.now), WorkspaceID: r.ws, Name: name, Label: attributedefs.Label(labelKey(name)),
		Origin: "observed", Now: r.now,
	})
	if err != nil {
		return def, "", fmt.Errorf("define event %s: %w", name, err)
	}

	r.defs[name] = &def

	return def, "", nil
}

func labelKey(name string) string {
	return strings.NewReplacer(".", " ", ":", " ", "-", " ").Replace(name)
}

func statusOf(def models.EventDefinition) string {
	if def.Status == "archived" {
		return StatusArchived
	}

	return ""
}

/*
entry is one event ready to be summed up: whose, which, when and its
details.
*/
type entry struct {
	contact ids.UUID
	def     models.EventDefinition
	at      time.Time
	meta    map[string]any
}

/*
summarize adds the event to the person's summary, keeping the details of
the newest event.
*/
func (r run) summarize(ctx context.Context, e entry) error {
	contact, def, at, meta := e.contact, e.def, e.at, e.meta
	row, err := r.q.LockEventSummary(ctx, models.LockEventSummaryParams{
		WorkspaceID: r.ws, ContactID: contact, EventDefinitionID: def.ID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("lock event summary: %w", err)
	}

	before := eventroll.Summary{
		Count: row.Count, FirstAt: row.FirstAt, LastAt: row.LastAt, Day: row.DailyDay, Daily: row.Daily,
	}
	s := eventroll.Add(before, at, 1, r.now)

	details := row.LastMetadata
	if len(details) == 0 || !at.Before(row.LastAt) {
		details, _ = json.Marshal(nonNil(meta))
	}

	err = r.q.SaveEventSummary(ctx, models.SaveEventSummaryParams{
		WorkspaceID: r.ws, ContactID: contact, EventDefinitionID: def.ID, Count: s.Count, FirstAt: s.FirstAt,
		LastAt: s.LastAt, DailyDay: s.Day, Daily: s.Daily, LastMetadata: details, Now: r.now,
	})

	return wrap("save event summary", err)
}

func nonNil(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}

	return meta
}

/*
wrap names what failed, leaving no error alone.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}
