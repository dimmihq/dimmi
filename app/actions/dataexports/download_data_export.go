/*
|--------------------------------------------------------------------------
| Download a Data Export
|--------------------------------------------------------------------------
|
| A ready export's file goes only to the member who asked for it, under
| the name and type it was built with. A workspace export can be far
| larger than memory, so it is never read here: its member gets a signed
| link that lasts a few minutes and downloads straight from the disk.
|
*/

package dataexports

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
linkTTL is how long a signed link to an export's file lasts.
*/
const linkTTL = 10 * time.Minute

/*
Download is a ready export's file: its name, type and bytes. Redirect,
when set, is a signed link the file downloads from instead of Data.
*/
type Download struct {
	Name        string
	ContentType string
	Data        []byte
	Redirect    string
}

/*
Link is a signed link to a ready export's file and when it stops
working.
*/
type Link struct {
	URL       string
	ExpiresAt time.Time
}

/*
LinkDataExport signs a link to the file of one of the caller's ready
exports.

Errors: [exceptions.ErrNotFound], [exceptions.ErrExportNotReady], or a
wrapped error, one wrapping [filesystem.ErrUnsupported] when the disk
cannot sign links.
*/
type LinkDataExport struct {
	DB      *database.DB
	Storage *filesystem.Manager
	Clock   clock.Clock
}

/*
Handle returns the link.
*/
func (a *LinkDataExport) Handle(ctx context.Context, in ExportInput) (Link, error) {
	row, err := (&ShowDataExport{DB: a.DB}).Handle(ctx, in)
	if err != nil {
		return Link{}, err
	}

	disk, err := readyDisk(a.Storage, row)
	if err != nil {
		return Link{}, err
	}

	url, err := disk.DownloadURL(ctx, *row.Path, filesystem.DownloadURLOptions{TTL: linkTTL, Filename: FileName(row)})
	if err != nil {
		return Link{}, fmt.Errorf("sign export link: %w", err)
	}

	return Link{URL: url, ExpiresAt: a.Clock.Now().Add(linkTTL)}, nil
}

/*
readyDisk returns the disk a ready export's file is on.
*/
func readyDisk(storage *filesystem.Manager, row models.DataExport) (filesystem.Filesystem, error) {
	if row.Status != "ready" || row.Disk == nil || row.Path == nil {
		return nil, exceptions.ErrExportNotReady
	}

	disk, err := storage.Disk(*row.Disk)
	if err != nil {
		return nil, fmt.Errorf("open disk %s: %w", *row.Disk, err)
	}

	return disk, nil
}

/*
DownloadDataExport returns the file of one of the caller's ready
exports.

Errors: [exceptions.ErrNotFound], [exceptions.ErrExportNotReady], or a
wrapped error.
*/
type DownloadDataExport struct {
	DB      *database.DB
	Storage *filesystem.Manager
}

/*
Handle returns the file, or for a workspace export a signed link to
it; only a disk that cannot sign links has such an export read whole.
*/
func (a *DownloadDataExport) Handle(ctx context.Context, in ExportInput) (Download, error) {
	row, err := (&ShowDataExport{DB: a.DB}).Handle(ctx, in)
	if err != nil {
		return Download{}, err
	}

	disk, err := readyDisk(a.Storage, row)
	if err != nil {
		return Download{}, err
	}

	name := FileName(row)
	kind := contentTypes()[strings.TrimPrefix(path.Ext(name), ".")]

	if row.Kind == KindWorkspace {
		url, err := disk.DownloadURL(ctx, *row.Path, filesystem.DownloadURLOptions{TTL: linkTTL, Filename: name})
		if err == nil {
			return Download{Name: name, ContentType: kind, Redirect: url}, nil
		}

		if !errors.Is(err, filesystem.ErrUnsupported) {
			return Download{}, fmt.Errorf("sign export link: %w", err)
		}
	}

	contents, err := disk.Get(ctx, *row.Path)
	if err != nil {
		return Download{}, fmt.Errorf("read export: %w", err)
	}

	return Download{Name: name, ContentType: kind, Data: contents}, nil
}
