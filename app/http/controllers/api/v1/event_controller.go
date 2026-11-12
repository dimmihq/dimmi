/*
|--------------------------------------------------------------------------
| Event Controller
|--------------------------------------------------------------------------
|
| POST /contact-events records one event, or up to 100 under "events",
| and answers what became of each. GET /event-definitions lists the
| workspace's event names; PATCH changes one's words, status or whether
| the messenger may send it. GET /contacts/{contact}/event-summaries
| says what one person did.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EventController serves events and their names.
*/
type EventController struct {
	Events *events.Events
	IDs    *ids.Registry
	Clock  clock.Clock
}

/*
TrackedEventsOutput is what became of the events sent.
*/
type TrackedEventsOutput struct {
	Body fhttp.Resource[resources.TrackedEvents]
}

/*
EventDefinitionsOutput is the workspace's event names.
*/
type EventDefinitionsOutput struct {
	Body fhttp.Collection[resources.EventDefinition]
}

/*
EventDefinitionOutput is one event name.
*/
type EventDefinitionOutput struct {
	Body fhttp.Resource[resources.EventDefinition]
}

/*
EventSummariesOutput is what one person did.
*/
type EventSummariesOutput struct {
	Body fhttp.Collection[resources.EventSummary]
}

/*
Track records events.
*/
func (c *EventController) Track(ctx context.Context, in *requests.TrackEventsRequest) (*TrackedEventsOutput, error) {
	sent := in.Body.Events
	if len(sent) == 0 {
		sent = []requests.EventSent{in.Body.EventSent}
	}

	batch := make([]events.Event, len(sent))
	for i, s := range sent {
		batch[i] = events.Event{
			Name: s.Name, ExternalID: s.ExternalID, Email: s.Email, OccurredAt: s.OccurredAt, Metadata: s.Metadata,
			IdempotencyKey: s.IdempotencyKey,
		}

		if s.ContactID != nil {
			id, err := c.IDs.Decode(string(models.PrefixContact), *s.ContactID)
			if err != nil {
				id = ids.Nil()
			}

			batch[i].ContactID = &id
		}
	}

	results, err := c.Events.Track(ctx, membership(ctx), batch)
	if err != nil {
		return nil, err
	}

	return &TrackedEventsOutput{Body: fhttp.NewResource(resources.NewTrackedEvents(results))}, nil
}

/*
Index lists the event names.
*/
func (c *EventController) Index(ctx context.Context, _ *requests.NoInput) (*EventDefinitionsOutput, error) {
	rows, err := c.Events.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.EventDefinition, len(rows))
	for i, r := range rows {
		items[i] = resources.NewEventDefinition(r)
	}

	page := pagination.Page[resources.EventDefinition]{Items: items, Limit: 1000}

	return &EventDefinitionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Update changes an event name's words or status.
*/
func (c *EventController) Update(ctx context.Context, in *requests.UpdateEventRequest,
) (*EventDefinitionOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixEvent), in.Event)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	b := in.Body
	d, err := c.Events.Update(ctx, events.UpdateInput{
		Membership: membership(ctx), ID: id, Label: b.Label, Description: b.Description, Status: b.Status,
		ClientWritable: b.ClientWritable,
	})
	if err != nil {
		return nil, err
	}

	return &EventDefinitionOutput{Body: fhttp.NewResource(resources.NewEventDefinition(d))}, nil
}

/*
Summaries says what one person did.
*/
func (c *EventController) Summaries(ctx context.Context, in *requests.ContactEventsRequest,
) (*EventSummariesOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixContact), in.Contact)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	rows, err := c.Events.Summaries(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	now := c.Clock.Now()
	items := make([]resources.EventSummary, len(rows))
	for i, r := range rows {
		items[i] = resources.NewEventSummary(r, now)
	}

	page := pagination.Page[resources.EventSummary]{Items: items, Limit: 1000}

	return &EventSummariesOutput{Body: fhttp.NewCollection(page)}, nil
}
