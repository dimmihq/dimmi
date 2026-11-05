/*
|--------------------------------------------------------------------------
| Update Avatar
|--------------------------------------------------------------------------
|
| A person's picture is shown to everyone they work with, so it is never
| stored as it arrived: it must decode as a picture of an accepted kind
| within the limits, and only a small square redrawn from its pixels is
| kept, under a new name each time. The old file goes after the change
| commits, by a job that can try again, and a storage hiccup never shows.
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
	"github.com/dimmihq/dimmi/framework/image"
	"github.com/dimmihq/dimmi/framework/queue"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The largest upload accepted and the size a picture is kept at.
*/
const (
	AvatarMaxBytes  = 2 << 20
	avatarSize      = 256
	avatarMaxPixels = 4096 * 4096
)

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
	picture, err := avatarFrom(upload)
	if err != nil {
		return models.User{}, err
	}

	path := "avatars/" + ids.New().String() + ".png"

	err = a.Disk.Put(ctx, path, picture, filesystem.PutOptions{ContentType: image.PNG.ContentType()})
	if err != nil {
		return models.User{}, fmt.Errorf("store avatar: %w", err)
	}

	writer := avatarWriter{DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue}

	user, err := writer.replace(ctx, userID, &path)
	if err != nil {
		return models.User{}, errors.Join(err, a.Disk.Delete(ctx, path))
	}

	return user, nil
}

/*
avatarFrom reads the upload and returns the picture to keep.
*/
func avatarFrom(upload io.Reader) ([]byte, error) {
	limits := image.Limits{MaxBytes: AvatarMaxBytes, MaxPixels: avatarMaxPixels}

	picture, cfg, err := image.Thumbnail(upload, avatarSize, limits)
	if errors.Is(err, image.ErrTooLarge) {
		return nil, exceptions.ErrAvatarTooLarge
	}

	if err != nil {
		return nil, exceptions.ErrInvalidAvatar
	}

	if cfg.Format == image.ICO {
		return nil, exceptions.ErrInvalidAvatar
	}

	return picture, nil
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
