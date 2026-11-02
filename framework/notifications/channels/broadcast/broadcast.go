/*
|--------------------------------------------------------------------------
| Broadcast Channel
|--------------------------------------------------------------------------
|
| A notification broadcast reaches the recipient's open screens at once,
| on the channels the recipient routes it to. Every broadcast carries the
| delivery id and the notification type next to the data it rendered,
| so a screen that also lists stored notifications can tell they are the
| same one.
|
| The broadcast keeps its place in the channel's history, so a screen
| that reconnects still receives it.
|
*/

package broadcast

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/notifications"
)

/*
Message is what a notification broadcasts. Event names the broadcast
and defaults to the notification type; Data is encoded as JSON.
*/
type Message struct {
	Event string
	Data  any
}

/*
Notification is a notification that can be broadcast.
*/
type Notification interface {
	/*
		ToBroadcast returns the broadcast for the delivery.
	*/
	ToBroadcast(ctx context.Context, d notifications.Delivery) (Message, error)
}

/*
Channel publishes notifications through a broadcaster.
*/
type Channel struct {
	broadcaster broadcasting.Broadcaster
}

/*
payload is the data every notification broadcast carries.
*/
type payload struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data any    `json:"data"`
}

/*
Factory builds broadcast channels for the notification manager.
*/
func Factory(broadcaster broadcasting.Broadcaster) manager.Factory[notifications.Channel] {
	return func(string, config.Section) (notifications.Channel, error) {
		return New(broadcaster), nil
	}
}

/*
New returns a channel publishing through the broadcaster.
*/
func New(broadcaster broadcasting.Broadcaster) *Channel {
	return &Channel{broadcaster: broadcaster}
}

/*
Send publishes the notification on the channels the recipient routes it
to: one channel name, or a slice of them.
*/
func (c *Channel) Send(ctx context.Context, d notifications.Delivery, n notifications.Notification) error {
	renderer, ok := n.(Notification)
	if !ok {
		return fmt.Errorf("broadcast %s: %w", n.Type(), notifications.ErrUnsupported)
	}

	channels, err := route(d.Notifiable.RouteFor(d.Channel))
	if err != nil || len(channels) == 0 {
		return err
	}

	msg, err := renderer.ToBroadcast(ctx, d)
	if err != nil {
		return fmt.Errorf("render %s for broadcast: %w", n.Type(), err)
	}

	event := cmp.Or(msg.Event, n.Type())

	err = c.broadcaster.BroadcastMany(ctx, channels, event, payload{ID: d.ID, Type: n.Type(), Data: msg.Data})
	if errors.Is(err, broadcasting.ErrUnknownChannel) || errors.Is(err, broadcasting.ErrPayloadTooLarge) {
		return fmt.Errorf("broadcast %s: %w: %w", n.Type(), notifications.ErrPermanent, err)
	}

	if err != nil {
		return fmt.Errorf("broadcast %s: %w", n.Type(), err)
	}

	return nil
}

/*
route reads a route into channel names, skipping empty ones.
*/
func route(r any) ([]string, error) {
	var names []string

	switch r := r.(type) {
	case nil:
		return nil, nil
	case string:
		names = []string{r}
	case []string:
		names = r
	default:
		return nil, fmt.Errorf("broadcast route %T: %w", r, notifications.ErrInvalidRoute)
	}

	channels := make([]string, 0, len(names))
	for _, name := range names {
		if name != "" {
			channels = append(channels, name)
		}
	}

	return channels, nil
}
