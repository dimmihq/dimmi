/*
|--------------------------------------------------------------------------
| Delete Avatar
|--------------------------------------------------------------------------
|
| Removing a picture puts the person's initials back everywhere at once.
| Removing it twice changes nothing.
|
*/

package users

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
DeleteAvatar removes the caller's own picture.

Events: [appevents.UserUpdated] when there was one.

Errors: a wrapped error.
*/
type DeleteAvatar struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
	Queue  *queue.Queue
}

/*
Handle removes the picture.
*/
func (a *DeleteAvatar) Handle(ctx context.Context, userID ids.UUID) error {
	writer := avatarWriter{DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue}

	_, err := writer.replace(ctx, userID, nil)

	return err
}
