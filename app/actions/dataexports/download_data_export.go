/*
|--------------------------------------------------------------------------
| Download a Data Export
|--------------------------------------------------------------------------
|
| A ready export's file goes only to the member who asked for it, under
| the name and type it was built with.
|
*/

package dataexports

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
Download is a ready export's file: its name, type and bytes.
*/
type Download struct {
	Name        string
	ContentType string
	Data        []byte
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
Handle returns the file.
*/
func (a *DownloadDataExport) Handle(ctx context.Context, in ExportInput) (Download, error) {
	row, err := (&ShowDataExport{DB: a.DB}).Handle(ctx, in)
	if err != nil {
		return Download{}, err
	}

	if row.Status != "ready" || row.Disk == nil || row.Path == nil {
		return Download{}, exceptions.ErrExportNotReady
	}

	disk, err := a.Storage.Disk(*row.Disk)
	if err != nil {
		return Download{}, fmt.Errorf("open disk %s: %w", *row.Disk, err)
	}

	contents, err := disk.Get(ctx, *row.Path)
	if err != nil {
		return Download{}, fmt.Errorf("read export: %w", err)
	}

	name := FileName(row)
	kind := contentTypes()[strings.TrimPrefix(path.Ext(name), ".")]

	return Download{Name: name, ContentType: kind, Data: contents}, nil
}
