/*
|--------------------------------------------------------------------------
| Event Settings Controller
|--------------------------------------------------------------------------
|
| GET and PATCH /event-settings read and set whether the messenger may
| send any event name.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
EventSettingsOutput is how the workspace takes events from the messenger.
*/
type EventSettingsOutput struct {
	Body fhttp.Resource[resources.EventSettings]
}

/*
Settings reads whether the messenger may send any event name.
*/
func (c *EventController) Settings(ctx context.Context, _ *requests.NoInput) (*EventSettingsOutput, error) {
	open, err := c.Events.MessengerSetting(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &EventSettingsOutput{Body: fhttp.NewResource(resources.NewEventSettings(open))}, nil
}

/*
UpdateSettings sets whether the messenger may send any event name.
*/
func (c *EventController) UpdateSettings(ctx context.Context, in *requests.UpdateEventSettingsRequest,
) (*EventSettingsOutput, error) {
	open := in.Body.MessengerNewEvents
	if err := c.Events.SetMessengerSetting(ctx, membership(ctx), open); err != nil {
		return nil, err
	}

	return &EventSettingsOutput{Body: fhttp.NewResource(resources.NewEventSettings(open))}, nil
}
