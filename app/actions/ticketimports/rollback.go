/*
|--------------------------------------------------------------------------
| Undoing A Ticket Import
|--------------------------------------------------------------------------
|
| A finished import can be undone once, by a member who sets tickets
| up. The undo runs in the background over the records the import
| made: its tracker links come off, its tickets go to the trash, where
| they can still be restored, and the fields, types and states it added
| are archived when no ticket left uses them. Tickets and setup that
| were already in the workspace are never touched. Like the import, the
| undo sets nothing off.
|
*/

package ticketimports

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrRolledBack refuses an import that is unfinished, or undone already.
*/
var ErrRolledBack = fault.New(fault.Conflict, "ticket_import_not_undoable",
	"Only a finished import that has not been undone can be undone.")

/*
RollbackArgs names the ticket import whose undo a job runs.
*/
type RollbackArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ImportID    ids.UUID `json:"import_id"`
}

/*
Kind names the job.
*/
func (RollbackArgs) Kind() string { return "roll_back_ticket_import" }

/*
Options run one undo at a time, retried a few times.
*/
func (RollbackArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
Rollback queues the undoing of a finished import.
*/
func (i *Importer) Rollback(ctx context.Context, m authz.Membership, id ids.UUID) (models.TicketImport, error) {
	if err := canManage(m); err != nil {
		return models.TicketImport{}, err
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	var out models.TicketImport

	err := i.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		_, err := q.FindTicketImport(ctx, models.FindTicketImportParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil {
			return exceptions.ErrNotFound
		}

		out, err = q.RequestTicketImportRollback(ctx, models.RequestTicketImportRollbackParams{
			MemberID: m.Member.ID, Now: i.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrRolledBack
		}

		if err != nil {
			return fmt.Errorf("request ticket import rollback: %w", err)
		}

		return i.Queue.PushTx(ctx, tx, RollbackArgs{WorkspaceID: m.Workspace.ID, ImportID: id})
	})

	return out, err
}

/*
RunRollback undoes a queued import to the end; one gone or not queued
is left alone.
*/
func (i *Importer) RunRollback(ctx context.Context, args RollbackArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())
	q := models.New(i.DB)

	row, err := q.StartTicketImportRollback(ctx, models.StartTicketImportRollbackParams{
		Now: i.Clock.Now(), WorkspaceID: args.WorkspaceID, ID: args.ImportID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("start ticket import rollback: %w", err)
	}

	var u undo

	err = i.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		u = undo{importer: i, row: row, tx: tx, q: models.New(tx)}

		return u.all(ctx)
	})

	params := models.FinishTicketImportRollbackParams{
		Status: "completed", Trashed: u.trashed, Archived: u.archived, Now: i.Clock.Now(),
		WorkspaceID: row.WorkspaceID, ID: row.ID,
	}
	if err != nil {
		message := "The undo stopped unexpectedly."
		params.Status, params.Error, params.Trashed, params.Archived = "failed", &message, 0, 0
	}

	return errors.Join(err, q.FinishTicketImportRollback(ctx, params))
}

/*
undo is one undo under way.
*/
type undo struct {
	importer          *Importer
	row               models.TicketImport
	tx                *database.Tx
	q                 *models.Queries
	trashed, archived int32
}

/*
all undoes the records the import made, links first, setup last.
*/
func (u *undo) all(ctx context.Context) error {
	records, err := u.q.ListTicketImportRecords(ctx, models.ListTicketImportRecordsParams{
		WorkspaceID: u.row.WorkspaceID, ImportID: u.row.ID,
	})
	if err != nil {
		return fmt.Errorf("list ticket import records: %w", err)
	}

	for _, kind := range []string{"link", "ticket", "field", "type", "state"} {
		for _, rec := range records {
			if rec.RecordKind != kind {
				continue
			}

			if err := u.one(ctx, rec); err != nil {
				return err
			}
		}
	}

	return nil
}

/*
one undoes a record by its kind.
*/
func (u *undo) one(ctx context.Context, rec models.TicketImportRecord) error {
	ws, now := u.row.WorkspaceID, u.importer.Clock.Now()

	switch rec.RecordKind {
	case "link":
		return u.unlink(ctx, rec.RecordID)
	case "ticket":
		return u.trash(ctx, rec.RecordID)
	case "field":
		u.archived++

		return wrap("archive imported field", u.q.ArchiveTicketField(ctx, models.ArchiveTicketFieldParams{
			Now: &now, WorkspaceID: ws, ID: rec.RecordID,
		}))
	case "type":
		return u.archiveType(ctx, rec.RecordID)
	case "state":
		return u.archiveState(ctx, rec.RecordID)
	}

	return nil
}

/*
unlink removes a tracker link the import made.
*/
func (u *undo) unlink(ctx context.Context, id ids.UUID) error {
	link, err := u.q.FindTicketLink(ctx, models.FindTicketLinkParams{WorkspaceID: u.row.WorkspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find imported link: %w", err)
	}

	w, err := u.writer(ctx)
	if err != nil {
		return err
	}

	return wrap("remove imported link", w.Unlink(ctx, link))
}

/*
trash moves a ticket the import made to the trash, logging it.
*/
func (u *undo) trash(ctx context.Context, id ids.UUID) error {
	now := u.importer.Clock.Now()

	t, err := u.q.TrashTicket(ctx, models.TrashTicketParams{Now: &now, WorkspaceID: u.row.WorkspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("trash imported ticket: %w", err)
	}

	u.trashed++

	_, err = u.q.CreateTicketEvent(ctx, models.CreateTicketEventParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID, Kind: "deleted", ActorType: "integration",
		Data: []byte(`{"reason":"import_undone"}`), Now: now,
	})

	return wrap("log trashed ticket", err)
}

/*
archiveType archives a type the import added once no ticket uses it.
*/
func (u *undo) archiveType(ctx context.Context, id ids.UUID) error {
	n, err := u.q.CountTicketsUsingType(ctx, models.CountTicketsUsingTypeParams{
		WorkspaceID: u.row.WorkspaceID, TypeID: id,
	})
	if err != nil || n > 0 {
		return wrap("count tickets of imported type", err)
	}

	now := u.importer.Clock.Now()
	u.archived++

	return wrap("archive imported type", u.q.ArchiveTicketType(ctx, models.ArchiveTicketTypeParams{
		Now: &now, WorkspaceID: u.row.WorkspaceID, ID: id,
	}))
}

/*
archiveState archives a state the import added once no ticket is in
it.
*/
func (u *undo) archiveState(ctx context.Context, id ids.UUID) error {
	n, err := u.q.CountTicketsUsingState(ctx, models.CountTicketsUsingStateParams{
		WorkspaceID: u.row.WorkspaceID, StateID: id,
	})
	if err != nil || n > 0 {
		return wrap("count tickets in imported state", err)
	}

	now := u.importer.Clock.Now()
	u.archived++

	return wrap("archive imported state", u.q.ArchiveTicketState(ctx, models.ArchiveTicketStateParams{
		Now: &now, WorkspaceID: u.row.WorkspaceID, ID: id,
	}))
}

/*
writer returns a ticket writer acting as the import, setting nothing
off.
*/
func (u *undo) writer(ctx context.Context) (*tickets.Writer, error) {
	r := &run{importer: u.importer, row: u.row, src: named(u.row.Source)}

	return r.writer(ctx, u.tx, u.importer.Clock.Now())
}

/*
wrap adds what failed to an error, nil for none.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}

/*
named is a source known only by its name, for writing as it after the
import ran.
*/
type named string

/*
Name returns the source's name.
*/
func (n named) Name() string { return string(n) }

/*
Catalog has nothing to read.
*/
func (n named) Catalog(context.Context) (Catalog, error) { return Catalog{}, nil }

/*
Tickets has nothing to read.
*/
func (n named) Tickets(context.Context, func(Ticket) error) error { return nil }
