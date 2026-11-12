/*
|--------------------------------------------------------------------------
| Widget Event Controller
|--------------------------------------------------------------------------
|
| POST /widget/v1/events records what the visitor did on the page, as the
| page told the messenger with Dimmi("track", name, metadata). Names the
| workspace has not opened to the messenger come back not_allowed.
|
*/

package widget

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/events"
	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
EventsOutput is what became of the events sent.
*/
type EventsOutput struct {
	Body fhttp.Resource[resources.WidgetTrackedEvents]
}

/*
EventController records the visitor's events.
*/
type EventController struct {
	Track *actions.TrackWidgetEvents
}

/*
Store records a batch of events on the visitor's contact.
*/
func (c *EventController) Store(ctx context.Context, in *requests.StoreWidgetEventsRequest) (*EventsOutput, error) {
	sent := make([]events.MessengerEvent, len(in.Body.Events))
	for i, e := range in.Body.Events {
		sent[i] = events.MessengerEvent{Name: e.Name, Metadata: e.Metadata, ID: e.ID}
	}

	results, err := c.Track.Handle(ctx, authenticated(ctx), sent)
	if err != nil {
		return nil, err
	}

	return &EventsOutput{Body: fhttp.NewResource(resources.NewWidgetTrackedEvents(results))}, nil
}
