/*
|--------------------------------------------------------------------------
| Session Streams
|--------------------------------------------------------------------------
|
| A session that ended must not keep listening. Its streams first hear
| that the session ended, so the messenger forgets it rather than trying
| again, and are then closed. Each stream connects under its session's
| public id, so closing every stream of a session is one call that
| reaches every replica. The notice goes out on the contact's channel
| without history, naming the session, so another device of the same
| visitor ignores it and a reconnect never replays it.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EventSessionRevoked is the notice a session's streams hear before they
close.
*/
const EventSessionRevoked = "widget.session_revoked"

/*
EndedSession is a session that just ended, with the contact whose
channel its streams listen on.
*/
type EndedSession struct {
	ID        ids.UUID
	ContactID ids.UUID
}

/*
Disconnector tells streams and closes every connection of a user id, on
every replica.
*/
type Disconnector interface {
	/*
		BroadcastEphemeral publishes the event on the channel without
		keeping it in history.
	*/
	BroadcastEphemeral(ctx context.Context, channel, event string, payload any) error

	/*
		Disconnect closes the connections of the user id.
	*/
	Disconnect(ctx context.Context, userID string) error
}

/*
BroadcastStreams closes sessions' streams through the realtime server.
*/
type BroadcastStreams struct {
	Server Disconnector
}

/*
CloseSessions tells every stream of the sessions that its session ended,
then disconnects it. A notice that fails still lets the stream close.
*/
func (b BroadcastStreams) CloseSessions(ctx context.Context, ended []EndedSession) error {
	var errs []error

	for _, s := range ended {
		public := models.PublicID(models.PrefixWidgetSession, s.ID)
		channel := appbroadcasting.WidgetContactChannel(s.ContactID)

		err := b.Server.BroadcastEphemeral(ctx, channel, EventSessionRevoked, map[string]string{"session_id": public})
		if err != nil {
			errs = append(errs, fmt.Errorf("tell widget session %s it ended: %w", s.ID, err))
		}

		if err := b.Server.Disconnect(ctx, public); err != nil {
			errs = append(errs, fmt.Errorf("disconnect widget session %s: %w", s.ID, err))
		}
	}

	return errors.Join(errs...)
}
