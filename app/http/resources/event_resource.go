/*
|--------------------------------------------------------------------------
| Event Resources
|--------------------------------------------------------------------------
|
| How event names, tracking results and a person's event summaries read
| over the API. A summary gives the counts of the last 7 and 30 days, as
| of now, rather than the raw daily counts.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/eventroll"
)

/*
EventDefinition is an event name.
*/
type EventDefinition struct {
	ID          string     `json:"id"`
	Object      string     `json:"object"`
	Name        string     `json:"name"`
	Label       string     `json:"label"`
	Description *string    `json:"description"`
	Origin      string     `json:"origin"`
	Status      string     `json:"status"`
	TotalCount  int64      `json:"total_count"`
	LastSeenAt  *time.Time `json:"last_seen_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

/*
NewEventDefinition shapes an event name.
*/
func NewEventDefinition(d models.EventDefinition) EventDefinition {
	return EventDefinition{
		ID: models.PublicID(models.PrefixEvent, d.ID), Object: "event_definition", Name: d.Name, Label: d.Label,
		Description: d.Description, Origin: d.Origin, Status: d.Status, TotalCount: d.TotalCount,
		LastSeenAt: d.LastSeenAt, CreatedAt: d.CreatedAt,
	}
}

/*
TrackedEvent is what became of one event sent.
*/
type TrackedEvent struct {
	Status    string  `json:"status"`
	ContactID *string `json:"contact_id"`
	Clamped   bool    `json:"occurred_at_clamped,omitempty"`
}

/*
TrackedEvents is what became of every event sent, in order.
*/
type TrackedEvents struct {
	Object   string         `json:"object"`
	Accepted int            `json:"accepted"`
	Events   []TrackedEvent `json:"events"`
}

/*
NewTrackedEvents shapes the results of tracking.
*/
func NewTrackedEvents(results []events.Result) TrackedEvents {
	out := TrackedEvents{Object: "contact_event_batch", Events: make([]TrackedEvent, 0, len(results))}
	for _, r := range results {
		e := TrackedEvent{Status: r.Status, Clamped: r.Clamped}
		if r.ContactID != ids.Nil() {
			id := models.PublicID(models.PrefixContact, r.ContactID)
			e.ContactID = &id
		}

		if r.Status == events.StatusAccepted {
			out.Accepted++
		}

		out.Events = append(out.Events, e)
	}

	return out
}

/*
EventSummary is what one person did of one event.
*/
type EventSummary struct {
	Object       string          `json:"object"`
	Name         string          `json:"name"`
	Label        string          `json:"label"`
	Count        int64           `json:"count"`
	Last7Days    int64           `json:"last_7_days"`
	Last30Days   int64           `json:"last_30_days"`
	FirstAt      time.Time       `json:"first_at"`
	LastAt       time.Time       `json:"last_at"`
	LastMetadata json.RawMessage `json:"last_metadata"`
}

/*
NewEventSummary shapes a summary as of now.
*/
func NewEventSummary(s events.Summary, now time.Time) EventSummary {
	roll := eventroll.Summary{Count: s.Count, FirstAt: s.FirstAt, LastAt: s.LastAt, Day: s.DailyDay, Daily: s.Daily}

	return EventSummary{
		Object: "contact_event_summary", Name: s.Name, Label: s.Label, Count: s.Count,
		Last7Days: eventroll.Within(roll, 7, now), Last30Days: eventroll.Within(roll, 30, now),
		FirstAt: s.FirstAt, LastAt: s.LastAt, LastMetadata: s.LastMetadata,
	}
}
