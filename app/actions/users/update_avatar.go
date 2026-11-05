/*
|--------------------------------------------------------------------------
| Update Avatar
|--------------------------------------------------------------------------
|
| A person's picture is shown to everyone they work with. It is kept the
| way every picture is: a small square redrawn from the upload, under a
| new name each time. The old file goes after the change commits, by a
| job that can try again, and a storage hiccup never shows.
|
*/

package users

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/pictures"
)

/*
AvatarMaxBytes is the largest upload accepted.
*/
const AvatarMaxBytes = pictures.MaxBytes

/*
UpdateAvatar replaces the caller's own picture.

Events: [appevents.UserUpdated].

Errors: [exceptions.ErrInvalidAvatar], [exceptions.ErrAvatarTooLarge], or
a wrapped error.
*/
type UpdateAvatar struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
	Disk   filesystem.Filesystem
	Queue  *queue.Queue
}

/*
Handle stores the picture and returns the user as they now are.
*/
func (a *UpdateAvatar) Handle(ctx context.Context, userID ids.UUID, upload io.Reader) (models.User, error) {
	path, err := pictures.Store(ctx, a.Disk, upload)
	if err != nil {
		return models.User{}, err
	}

	writer := avatarWriter{DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue}

	user, err := writer.replace(ctx, userID, &path)
	if err != nil {
		return models.User{}, errors.Join(err, a.Disk.Delete(ctx, path))
	}

	return user, nil
}

/*
avatarWriter records which picture a user has.
*/
type avatarWriter struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
	Queue  *queue.Queue
}

/*
replace gives the user the picture at the path, or none, queues the old
file's removal and tells teammates, in one transaction.
*/
func (w avatarWriter) replace(ctx context.Context, userID ids.UUID, path *string) (models.User, error) {
	var updated models.User

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		user, err := q.LockUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("lock user to change avatar: %w", err)
		}

		updated, err = q.ReplaceUserAvatarPath(ctx, models.ReplaceUserAvatarPathParams{
			ID: userID, AvatarPath: path, Now: w.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("record avatar: %w", err)
		}

		if user.AvatarPath == nil && path == nil {
			return nil
		}

		if user.AvatarPath != nil {
			err = w.Queue.PushTx(ctx, tx, jobs.DeleteUserAvatarArgs{Path: *user.AvatarPath})
			if err != nil {
				return fmt.Errorf("queue removal of old avatar: %w", err)
			}
		}

		memberships, err := membershipsOf(ctx, q, userID)
		if err != nil {
			return err
		}

		return w.Events.Dispatch(ctx, tx, appevents.UserUpdated{
			UserID: userID, Changes: []string{"avatar"}, Memberships: memberships,
		})
	})

	return updated, err
}
