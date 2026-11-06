/*
|--------------------------------------------------------------------------
| Managing Data Exports
|--------------------------------------------------------------------------
|
| An export belongs to the member who asked for it: only they list it,
| see it, download it and delete it. Deleting removes the file at once,
| so data a member no longer needs does not wait for the export period.
|
*/

package dataexports

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listLimit is how many of a member's exports the list shows.
*/
const listLimit = 50

/*
ExportInput is who asks about which export.
*/
type ExportInput struct {
	Membership authz.Membership
	ExportID   ids.UUID
}

/*
ListDataExports lists the caller's own exports, newest first.

Errors: a wrapped database error.
*/
type ListDataExports struct {
	DB *database.DB
}

/*
Handle returns the exports.
*/
func (a *ListDataExports) Handle(ctx context.Context, m authz.Membership) ([]models.DataExport, error) {
	var out []models.DataExport

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = models.New(tx).ListMemberDataExports(ctx, models.ListMemberDataExportsParams{
			WorkspaceID: m.Workspace.ID, MemberID: &m.Member.ID, RowLimit: listLimit,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list exports: %w", err)
	}

	return out, nil
}

/*
ShowDataExport returns one of the caller's exports.

Errors: [exceptions.ErrNotFound], or a wrapped database error.
*/
type ShowDataExport struct {
	DB *database.DB
}

/*
Handle returns the export.
*/
func (a *ShowDataExport) Handle(ctx context.Context, in ExportInput) (models.DataExport, error) {
	var out models.DataExport

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = own(ctx, models.New(tx), in)

		return err
	})

	return out, err
}

/*
DeleteDataExport deletes one of the caller's exports and its file.

Errors: [exceptions.ErrNotFound], or a wrapped error.
*/
type DeleteDataExport struct {
	DB      *database.DB
	Events  *events.Dispatcher
	Clock   clock.Clock
	Storage *filesystem.Manager
}

/*
Handle removes the file, then the export.
*/
func (a *DeleteDataExport) Handle(ctx context.Context, in ExportInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := own(ctx, q, in)
		if err != nil {
			return err
		}

		if err := removeFile(ctx, a.Storage, row); err != nil {
			return err
		}

		err = q.DeleteDataExport(ctx, models.DeleteDataExportParams{WorkspaceID: row.WorkspaceID, ID: row.ID})
		if err != nil {
			return fmt.Errorf("delete export: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.DataExportDeleted{
			Base: base(in.Membership, a.Clock), ExportID: row.ID,
		})
	})
}

/*
FileName is the name an export's file downloads under: the stored name
for a ready export, what it will be called otherwise.
*/
func FileName(row models.DataExport) string {
	if row.Path != nil {
		return path.Base(*row.Path)
	}

	return "export." + row.Format
}

/*
own returns the caller's export, or exceptions.ErrNotFound for one that
does not exist or belongs to someone else.
*/
func own(ctx context.Context, q *models.Queries, in ExportInput) (models.DataExport, error) {
	row, err := q.FindDataExport(ctx, models.FindDataExportParams{
		WorkspaceID: in.Membership.Workspace.ID, ID: in.ExportID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find export: %w", err)
	}

	if row.RequestedByMemberID == nil || *row.RequestedByMemberID != in.Membership.Member.ID {
		return row, exceptions.ErrNotFound
	}

	return row, nil
}

/*
removeFile deletes an export's file, if it has one, a file gone already
included.
*/
func removeFile(ctx context.Context, storage *filesystem.Manager, row models.DataExport) error {
	if row.Disk == nil || row.Path == nil {
		return nil
	}

	disk, err := storage.Disk(*row.Disk)
	if err != nil {
		return fmt.Errorf("open disk %s: %w", *row.Disk, err)
	}

	if err := disk.Delete(ctx, *row.Path); err != nil && !errors.Is(err, filesystem.ErrNotFound) {
		return fmt.Errorf("delete export file: %w", err)
	}

	return nil
}
