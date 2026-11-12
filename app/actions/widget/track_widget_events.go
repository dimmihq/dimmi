/*
|--------------------------------------------------------------------------
| Track Widget Events
|--------------------------------------------------------------------------
|
| The page tells the messenger what the visitor did, Dimmi("track", name,
| metadata), and the messenger sends it here in batches of up to 20 once
| the visitor has a session. The session names the person. Each batch
| counts against the session's and the network's limits, the same way
| messages do, and the events go to the workspace's event summaries.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/app/actions/events"
)

/*
TrackWidgetEvents records the events a visitor's messenger sent.
*/
type TrackWidgetEvents struct {
	Events *events.Events
	Limits *VisitorLimits
}

/*
Handle records the batch on the session's contact.
*/
func (a *TrackWidgetEvents) Handle(
	ctx context.Context, auth Authenticated, sent []events.MessengerEvent,
) ([]events.Result, error) {
	if err := a.Limits.Events(ctx, auth); err != nil {
		return nil, err
	}

	s := auth.Session

	return a.Events.FromMessenger(ctx, events.Visitor{
		WorkspaceID: s.WorkspaceID, ContactID: s.ContactID, SessionID: s.ID,
	}, sent)
}
