/*
|--------------------------------------------------------------------------
| Contact Exports
|--------------------------------------------------------------------------
|
| A privacy request may ask for a copy of everything held about a
| customer. The copy is built in the background into a zip, which the
| member who may answer such requests downloads within a day. The
| team's internal notes stay out unless asked for, since the request is
| about what the customer gave and was told.
|
*/

package contacts

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
BuildExportArgs names the export a job builds.
*/
type BuildExportArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ExportID    ids.UUID `json:"export_id"`
}

/*
Kind names the job.
*/
func (BuildExportArgs) Kind() string { return "build_contact_export" }

/*
Options build one export at a time, retried a few times.
*/
func (BuildExportArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: 5,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRetryable, queue.StateRunning, queue.StateScheduled,
		}},
	}
}

/*
ContactExportInput is who asks about which customer's export: the
export to show or download, and for a new one whether to include notes.
*/
type ContactExportInput struct {
	Membership   authz.Membership
	ContactID    ids.UUID
	ExportID     ids.UUID
	IncludeNotes bool
}

/*
RequestContactExport starts building a customer's export.

Rules:
  - The caller needs contacts:erase.
  - One export per customer is built at a time.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrExportInProgress], or a wrapped error.
*/
type RequestContactExport struct {
	DB    *database.DB
	Clock clock.Clock
	Queue *queue.Queue
}

/*
Handle records the export and queues its build.
*/
func (a *RequestContactExport) Handle(ctx context.Context, in ContactExportInput) (models.ContactExport, error) {
	if !in.Membership.Can("contacts:erase") {
		return models.ContactExport{}, exceptions.ErrForbidden
	}

	var out models.ContactExport

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		contact, err := Survivor(ctx, q, ws, in.ContactID)
		if err != nil {
			return err
		}

		out, err = q.CreateContactExport(ctx, models.CreateContactExportParams{
			WorkspaceID: ws, ContactID: contact.ID, RequestedByMemberID: &in.Membership.Member.ID,
			IncludeNotes: in.IncludeNotes, Now: a.Clock.Now(),
		})
		if database.IsUniqueViolation(err, "contact_exports_running_key") {
			return exceptions.ErrExportInProgress
		}

		if err != nil {
			return fmt.Errorf("record export: %w", err)
		}

		if err := a.Queue.PushTx(ctx, tx, BuildExportArgs{WorkspaceID: ws, ExportID: out.ID}); err != nil {
			return fmt.Errorf("queue export: %w", err)
		}

		return nil
	})

	return out, err
}

/*
ShowContactExport returns an export and where it stands.

Rules:
  - The caller needs contacts:erase.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowContactExport struct {
	DB *database.DB
}

/*
Handle returns the export.
*/
func (a *ShowContactExport) Handle(ctx context.Context, in ContactExportInput) (models.ContactExport, error) {
	if !in.Membership.Can("contacts:erase") {
		return models.ContactExport{}, exceptions.ErrForbidden
	}

	var out models.ContactExport

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = findExport(ctx, models.New(tx), in)

		return err
	})

	return out, err
}

/*
DownloadContactExport returns the zip of a ready export.

Rules:
  - The caller needs contacts:erase.
  - Only a ready export can be downloaded.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrExportNotReady], or a wrapped error.
*/
type DownloadContactExport struct {
	DB      *database.DB
	Storage *filesystem.Manager
}

/*
Handle returns the zip's bytes.
*/
func (a *DownloadContactExport) Handle(ctx context.Context, in ContactExportInput) ([]byte, error) {
	row, err := (&ShowContactExport{DB: a.DB}).Handle(ctx, in)
	if err != nil {
		return nil, err
	}

	if row.Status != "ready" || row.Disk == nil || row.Path == nil {
		return nil, exceptions.ErrExportNotReady
	}

	disk, err := a.Storage.Disk(*row.Disk)
	if err != nil {
		return nil, fmt.Errorf("open disk %s: %w", *row.Disk, err)
	}

	zip, err := disk.Get(ctx, *row.Path)
	if err != nil {
		return nil, fmt.Errorf("read export: %w", err)
	}

	return zip, nil
}

/*
findExport returns the customer's export, or exceptions.ErrNotFound.
*/
func findExport(ctx context.Context, q *models.Queries, in ContactExportInput) (models.ContactExport, error) {
	row, err := q.FindContactExport(ctx, models.FindContactExportParams{
		WorkspaceID: in.Membership.Workspace.ID, ContactID: in.ContactID, ID: in.ExportID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find export: %w", err)
	}

	return row, nil
}
