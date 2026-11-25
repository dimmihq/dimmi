/*
|--------------------------------------------------------------------------
| Build a Workspace Export
|--------------------------------------------------------------------------
|
| A workspace export is everything the workspace owns, so only a member
| who may export the workspace gets one, checked again when the build
| runs. The zip streams to the workspace's folder as it is written, and
| a build that fails takes its half-written file with it.
|
*/

package dataexports

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/models"
)

/*
buildArchive writes the workspace's archive to the disk and marks the
export ready.
*/
func (a *BuildDataExport) buildArchive(ctx context.Context, row models.DataExport) error {
	var ws models.Workspace

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		m, err := requester(ctx, models.New(tx), row)
		ws = m.Workspace

		return err
	})
	if err != nil {
		return err
	}

	disk, err := a.Storage.Disk(a.DiskName)
	if err != nil {
		return fmt.Errorf("open disk %s: %w", a.DiskName, err)
	}

	path := "workspaces/" + row.WorkspaceID.String() + "/data-exports/" + row.ID.String() + "/" + archiveName
	w := &archiver{db: a.DB, storage: a.Storage, disk: a.DiskName, ws: ws}

	size, err := a.streamArchive(ctx, disk, path, w)
	if err != nil {
		if gone := disk.Delete(ctx, path); gone != nil && !errors.Is(gone, filesystem.ErrNotFound) {
			err = errors.Join(err, fmt.Errorf("remove unfinished export: %w", gone))
		}

		return err
	}

	var warnings []string
	if n := len(w.manifest.SkippedFiles); n > 0 {
		warnings = append(warnings, strconv.Itoa(n)+" files were left out; manifest.json lists them.")
	}

	ttl := time.Duration(ws.Retention().ExportDays) * 24 * time.Hour

	return a.ready(ctx, row, finished{path: path, size: size, rows: w.records, ttl: ttl, warnings: warnings})
}

/*
streamArchive writes the archive into a pipe the disk stores from, so
the file is never held in memory, and returns its size.
*/
func (a *BuildDataExport) streamArchive(
	ctx context.Context, disk filesystem.Filesystem, path string, w *archiver,
) (int64, error) {
	reader, writer := io.Pipe()
	stored := make(chan error, 1)

	go func() {
		err := disk.PutStream(ctx, path, reader, filesystem.PutOptions{ContentType: contentTypes()["zip"]})
		stored <- errors.Join(err, reader.CloseWithError(err))
	}()

	counted := &countingWriter{to: writer}
	w.zip = zip.NewWriter(counted)

	err := w.write(ctx, a.Clock.Now())
	if err == nil {
		err = w.zip.Close()
	}

	if failed := errors.Join(writer.CloseWithError(err), <-stored); failed != nil {
		return 0, errors.Join(err, fmt.Errorf("store export: %w", failed))
	}

	return counted.n, err
}
