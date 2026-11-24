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

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/image"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
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
	DB    *database.DB
	Clock clock.Clock
}

/*
Store redraws the upload and keeps it, returning the address it is
served at.
*/
func (i *Images) Store(ctx context.Context, m authz.Membership, upload io.Reader) (string, error) {
	if !m.Can("articles:write") {
		return "", exceptions.ErrForbidden
	}

	return i.Keep(ctx, m.Workspace.ID, upload)
}

/*
Keep redraws a picture and keeps it for the workspace, returning the
address it is served at, for callers that checked who asked, such as an
import. The workspace is noted beside the file, so purging the workspace
deletes the picture.
*/
func (i *Images) Keep(ctx context.Context, workspaceID ids.UUID, upload io.Reader) (string, error) {
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

	path := ImageDir + "/" + ids.New().String() + "." + imageExtension(cfg.Format)

	if err := i.record(ctx, workspaceID, path); err != nil {
		return "", err
	}

	err = disk.Put(ctx, path, picture, filesystem.PutOptions{ContentType: cfg.Format.ContentType()})
	if err != nil {
		return "", fmt.Errorf("store image: %w", err)
	}

	return "/" + path, nil
}

/*
record notes which workspace keeps the picture, before the file is
written, so no stored picture is ever without its workspace.
*/
func (i *Images) record(ctx context.Context, workspaceID ids.UUID, path string) error {
	scoped := tenancy.With(ctx, workspaceID.String())

	err := i.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).RecordHelpCenterImage(ctx, models.RecordHelpCenterImageParams{
			WorkspaceID: workspaceID, Path: path, Now: i.Clock.Now(),
		})
	})
	if err != nil {
		return fmt.Errorf("record image: %w", err)
	}

	return nil
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
