/*
|--------------------------------------------------------------------------
| Article Images
|--------------------------------------------------------------------------
|
| Pictures in articles are shown to every reader, so an upload is never
| kept as it arrived: it must decode as an image within the limits and is
| written again from its pixels, which drops camera details and anything
| hidden in the file. Each is kept under a fresh random name, so its
| address can be cached for good and never guessed, and is served from
| this server on every host a help center answers on.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/image"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
MaxImageBytes is the largest picture accepted, and ImageDir the folder
pictures are kept in on the disk and served from.
*/
const (
	MaxImageBytes  = 10 << 20
	ImageDir       = "help-images"
	maxImagePixels = 50_000_000
)

/*
ErrInvalidImage refuses an upload that is not a picture, and
ErrImageTooLarge one over the size limits.
*/
var (
	ErrInvalidImage = fault.New(fault.Invalid, "invalid_image",
		"Choose a PNG, JPEG, WebP or GIF image.").OnField("image")
	ErrImageTooLarge = fault.New(fault.Invalid, "image_too_large",
		"The image must be 10 MB or smaller.").OnField("image")
)

/*
ImageDisks finds the disk pictures are kept on.
*/
type ImageDisks interface {
	/*
		Default returns the application's default disk.
	*/
	Default() (filesystem.Filesystem, error)
}

/*
Images keeps the pictures placed in articles.

Rules:
  - The caller needs articles:write.

Errors: [exceptions.ErrForbidden], [ErrInvalidImage], [ErrImageTooLarge],
or a wrapped storage error.
*/
type Images struct {
	Disks ImageDisks
}

/*
Store redraws the upload and keeps it, returning the address it is
served at.
*/
func (i *Images) Store(ctx context.Context, m authz.Membership, upload io.Reader) (string, error) {
	if !m.Can("articles:write") {
		return "", exceptions.ErrForbidden
	}

	picture, cfg, err := image.Reencode(upload, image.Options{
		Limits: image.Limits{MaxBytes: MaxImageBytes, MaxPixels: maxImagePixels},
	})
	if errors.Is(err, image.ErrTooLarge) {
		return "", ErrImageTooLarge
	}

	if err != nil {
		return "", ErrInvalidImage
	}

	disk, err := i.Disks.Default()
	if err != nil {
		return "", fmt.Errorf("image disk: %w", err)
	}

	name := ids.New().String() + "." + imageExtension(cfg.Format)

	err = disk.Put(ctx, ImageDir+"/"+name, picture, filesystem.PutOptions{ContentType: cfg.Format.ContentType()})
	if err != nil {
		return "", fmt.Errorf("store image: %w", err)
	}

	return "/" + ImageDir + "/" + name, nil
}

/*
imageExtension names a written format's file extension.
*/
func imageExtension(f image.Format) string {
	if f == image.JPEG {
		return "jpg"
	}

	return string(f)
}
