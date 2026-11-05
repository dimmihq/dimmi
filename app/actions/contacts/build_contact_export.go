/*
|--------------------------------------------------------------------------
| Build a Contact Export
|--------------------------------------------------------------------------
|
| The zip a privacy request asked for: the customer's details and the
| addresses they reached the team from, each conversation as data and as
| a readable transcript, the files they and the team exchanged, and one
| section per registered exporter. A file the malware scan flagged is
| left out and named instead. The zip sits in the workspace's folder, so
| a workspace purge removes it with everything else.
|
*/

package contacts

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BuildContactExport builds a requested export.
*/
type BuildContactExport struct {
	DB        *database.DB
	Events    *events.Dispatcher
	Clock     clock.Clock
	Storage   *filesystem.Manager
	DiskName  string
	TTL       time.Duration
	Exporters *Exporters
}

/*
Handle builds the export, records a failure before returning it so
the member sees it, and leaves an export already built or gone alone.
*/
func (a *BuildContactExport) Handle(ctx context.Context, args BuildExportArgs) error {
	q := models.New(a.DB)

	row, err := q.StartContactExport(ctx, models.StartContactExportParams{
		Now: a.Clock.Now(), WorkspaceID: args.WorkspaceID, ID: args.ExportID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("start export: %w", err)
	}

	if err := a.build(ctx, row); err != nil {
		failed := q.FailContactExport(ctx, models.FailContactExportParams{
			Error: new(err.Error()), Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		})

		return fmt.Errorf("build export: %w", errors.Join(err, failed))
	}

	return nil
}

/*
build writes the zip, stores it and marks the export ready.
*/
func (a *BuildContactExport) build(ctx context.Context, row models.ContactExport) error {
	var buf bytes.Buffer

	w := zip.NewWriter(&buf)
	if err := a.write(ctx, w, row); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}

	disk, err := a.Storage.Disk(a.DiskName)
	if err != nil {
		return fmt.Errorf("open disk %s: %w", a.DiskName, err)
	}

	path := "workspaces/" + row.WorkspaceID.String() + "/contact-exports/" + row.ID.String() + ".zip"
	if err := disk.Put(ctx, path, buf.Bytes(), filesystem.PutOptions{ContentType: "application/zip"}); err != nil {
		return fmt.Errorf("store export: %w", err)
	}

	return a.ready(ctx, row, path, int64(buf.Len()))
}

/*
ready marks the export ready to download until it expires and tells the
member who asked.
*/
func (a *BuildContactExport) ready(ctx context.Context, row models.ContactExport, path string, size int64) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		now := a.Clock.Now()

		if _, err := models.New(tx).FinishContactExport(ctx, models.FinishContactExportParams{
			Disk: &a.DiskName, Path: &path, SizeBytes: &size, ExpiresAt: new(now.Add(a.TTL)), Now: now,
			WorkspaceID: row.WorkspaceID, ID: row.ID,
		}); err != nil {
			return fmt.Errorf("finish export: %w", err)
		}

		if row.RequestedByMemberID == nil {
			return nil
		}

		return a.Events.Dispatch(ctx, tx, appevents.ContactExportReady{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: row.WorkspaceID, OccurredAt: now,
				Actor: appevents.Actor{Type: "system", ID: ids.Nil()},
			},
			ExportID: row.ID, ContactID: row.ContactID, RequestedBy: *row.RequestedByMemberID,
		})
	})
}

/*
write adds every section to the zip.
*/
func (a *BuildContactExport) write(ctx context.Context, w *zip.Writer, row models.ContactExport) error {
	run := exportRun{q: models.New(a.DB), storage: a.Storage, w: w, row: row}

	if err := run.writeContact(ctx); err != nil {
		return err
	}

	if err := run.writeConversations(ctx); err != nil {
		return err
	}

	sections, err := a.Exporters.Run(ctx, row.WorkspaceID, row.ContactID)
	if err != nil {
		return err
	}

	for name, section := range sections {
		if err := writeJSON(w, name+".json", section.Data); err != nil {
			return err
		}

		for _, file := range section.Files {
			if err := writeFile(w, name+"/"+file.Name, file.Contents); err != nil {
				return err
			}
		}
	}

	return nil
}

/*
writeJSON adds a JSON file to the zip.
*/
func writeJSON(w *zip.Writer, name string, data any) error {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}

	return writeFile(w, name, encoded)
}

/*
writeFile adds a file to the zip.
*/
func writeFile(w *zip.Writer, name string, contents []byte) error {
	f, err := w.Create(name)
	if err != nil {
		return fmt.Errorf("add %s: %w", name, err)
	}

	if _, err := f.Write(contents); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}

	return nil
}
