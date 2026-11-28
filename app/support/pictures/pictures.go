/*
|--------------------------------------------------------------------------
| Pictures
|--------------------------------------------------------------------------
|
| A picture people upload is shown to everyone they work with and to
| customers, so it is never stored as it arrived: it must decode as an
| image of an accepted kind within the limits, and only a small square
| PNG redrawn from its pixels is kept, under a fresh random name each
| time, so its address can be cached for good and never guessed.
|
*/

package pictures

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/image"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
The largest upload accepted, the size a picture is kept at and the most
pixels an upload may decode to.
*/
const (
	MaxBytes  = 2 << 20
	size      = 256
	maxPixels = 4096 * 4096
)

/*
DeleteArgs names a stored picture to remove once the change that let go
of it commits. The job's kind keeps its first name so queued jobs still
run.
*/
type DeleteArgs struct {
	Path string `json:"path"`
}

/*
Kind names the job.
*/
func (DeleteArgs) Kind() string {
	return "delete_user_avatar"
}

/*
Store redraws the upload as a square PNG and puts it on the disk under a
new name, returning the path it is stored at.

Errors: [exceptions.ErrInvalidAvatar], [exceptions.ErrAvatarTooLarge],
or a wrapped storage error.
*/
func Store(ctx context.Context, disk filesystem.Filesystem, upload io.Reader) (string, error) {
	picture, err := square(upload)
	if err != nil {
		return "", err
	}

	path := "avatars/" + ids.New().String() + ".png"

	err = disk.Put(ctx, path, picture, filesystem.PutOptions{ContentType: image.PNG.ContentType()})
	if err != nil {
		return "", fmt.Errorf("store picture: %w", err)
	}

	return path, nil
}

/*
Inline redraws the upload as a square PNG of the size on each side and
returns it as a data address, for a picture shown once and never kept.

Errors: [exceptions.ErrInvalidAvatar], [exceptions.ErrAvatarTooLarge].
*/
func Inline(upload io.Reader, side int) (string, error) {
	picture, err := draw(upload, side)
	if err != nil {
		return "", err
	}

	return "data:" + image.PNG.ContentType() + ";base64," + base64.StdEncoding.EncodeToString(picture), nil
}

/*
square reads the upload and returns the picture to keep.
*/
func square(upload io.Reader) ([]byte, error) {
	return draw(upload, size)
}

/*
draw reads the upload and returns it as a square PNG of the size.
*/
func draw(upload io.Reader, side int) ([]byte, error) {
	limits := image.Limits{MaxBytes: MaxBytes, MaxPixels: maxPixels}

	picture, cfg, err := image.Thumbnail(upload, side, limits)
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
