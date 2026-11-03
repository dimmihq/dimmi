/*
|--------------------------------------------------------------------------
| Fetch User Avatar
|--------------------------------------------------------------------------
|
| Someone who signs up through a provider brings a picture along. It is
| copied once into the application's own storage, so the dashboard never
| shows a provider's address, which would tell the provider who looks at
| it and could change under us at any time.
|
| The picture is fetched after the sign-up answers, through the guarded
| HTTP client, and only kept once it decodes as an image within the
| limits: it is written again from its pixels, so nothing but the
| picture is stored. A user who chose an avatar in the meantime keeps
| theirs. A picture that is gone or not an image is given up on, while
| a provider that does not answer is tried again a few times.
|
*/

package jobs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/image"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on a fetched picture: how long the provider has to send it, how
many bytes and pixels it may have, and how often the job is tried.
*/
const (
	avatarTimeout   = 10 * time.Second
	avatarMaxBytes  = 5 << 20
	avatarMaxPixels = 4096 * 4096
	avatarAttempts  = 3
)

/*
errAvatarStatus means the provider answered without the picture.
*/
var errAvatarStatus = errors.New("fetch avatar: unexpected status")

/*
FetchUserAvatarArgs names the user and the picture to copy.
*/
type FetchUserAvatarArgs struct {
	UserID ids.UUID `json:"user_id"`
	URL    string   `json:"url"`
}

/*
Kind names the job.
*/
func (FetchUserAvatarArgs) Kind() string {
	return "fetch_user_avatar"
}

/*
Options try the job a few times only: a picture is not worth more.
*/
func (FetchUserAvatarArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: avatarAttempts}
}

/*
FetchUserAvatar copies a provider's picture of a user into storage.
*/
type FetchUserAvatar struct {
	DB    *database.DB
	Clock clock.Clock
	HTTP  *httpclient.Client
	Disk  filesystem.Filesystem
}

/*
Handle fetches, checks and stores the picture, and gives it to the user
unless they have one.
*/
func (w *FetchUserAvatar) Handle(ctx context.Context, job *queue.Job[FetchUserAvatarArgs]) error {
	res, err := w.HTTP.New().Timeout(avatarTimeout).MaxBody(avatarMaxBytes).Get(ctx, job.Args.URL)
	if errors.Is(err, httpclient.ErrBlockedHost) || errors.Is(err, httpclient.ErrTooLarge) {
		return queue.Cancel(err)
	}

	if err != nil {
		return fmt.Errorf("fetch avatar: %w", err)
	}

	if res.Status < 200 || res.Status > 299 {
		return queue.Cancel(fmt.Errorf("%w %d", errAvatarStatus, res.Status))
	}

	picture, _, err := image.Reencode(bytes.NewReader(res.Body), image.Options{
		Format: image.PNG, Limits: image.Limits{MaxBytes: avatarMaxBytes, MaxPixels: avatarMaxPixels},
	})
	if err != nil {
		return queue.Cancel(fmt.Errorf("decode avatar: %w", err))
	}

	return w.store(ctx, job.Args.UserID, picture)
}

/*
store writes the picture and records it on the user, removing it again
when the user chose another avatar first.
*/
func (w *FetchUserAvatar) store(ctx context.Context, userID ids.UUID, picture []byte) error {
	path := "avatars/" + ids.New().String() + ".png"

	err := w.Disk.Put(ctx, path, picture, filesystem.PutOptions{ContentType: image.PNG.ContentType()})
	if err != nil {
		return fmt.Errorf("store avatar: %w", err)
	}

	set, err := models.New(w.DB).SetUserAvatarPath(ctx, models.SetUserAvatarPathParams{
		ID: userID, AvatarPath: &path, Now: w.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("record avatar: %w", err)
	}

	if set == 0 {
		return w.Disk.Delete(ctx, path)
	}

	return nil
}
