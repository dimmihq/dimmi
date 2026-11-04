/*
|--------------------------------------------------------------------------
| Session Streams
|--------------------------------------------------------------------------
|
| A session that ended must not keep listening. Each stream connects under
| its session's public id, so ending every stream of a session is one call
| that reaches every replica.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Disconnector closes every connection of a user id, on every replica.
*/
type Disconnector interface {
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
CloseSessions disconnects every stream of the sessions.
*/
func (b BroadcastStreams) CloseSessions(ctx context.Context, sessionIDs []ids.UUID) error {
	var errs []error

	for _, id := range sessionIDs {
		err := b.Server.Disconnect(ctx, models.PublicID(models.PrefixWidgetSession, id))
		if err != nil {
			errs = append(errs, fmt.Errorf("disconnect widget session %s: %w", id, err))
		}
	}

	return errors.Join(errs...)
}
