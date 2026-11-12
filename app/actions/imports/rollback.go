/*
|--------------------------------------------------------------------------
| Undoing Imports
|--------------------------------------------------------------------------
|
| A finished import can be undone for a week, once, by a teammate who
| holds imports:rollback. The undo runs in the background as that
| teammate, record by record over the changes the import kept: a record
| it made is removed, a record it changed gets its old values back field
| by field, and the tags it added come off. Anything someone has worked
| on since is kept, with why. The counts are saved after each batch, so
| the screen shows progress and a retried run picks up where it
| stopped.
|
*/

package imports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RollbackWindow is how long after it finishes an import can be undone,
and keptShown how many kept records an undo names.
*/
const (
	RollbackWindow = 7 * 24 * time.Hour
	keptShown      = 100
)

/*
The refusals of undoing an import.
*/
var (
	ErrNotFinished = fault.New(fault.Conflict, "import_not_finished",
		"This import hasn't finished yet.")
	ErrRollbackClosed = fault.New(fault.Conflict, "import_rollback_closed",
		"Imports can be undone for 7 days after they finish.")
	ErrRolledBack = fault.New(fault.Conflict, "import_rolled_back",
		"This import is already being undone or has been undone.")
	ErrNothingToUndo = fault.New(fault.Conflict, "import_nothing_to_undo",
		"This import has nothing left to undo.")
)

/*
RollbackArgs names the import whose undo a job runs.
*/
type RollbackArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ImportID    ids.UUID `json:"import_id"`
}

/*
Kind names the job.
*/
func (RollbackArgs) Kind() string { return "roll_back_import" }

/*
Options run one undo once at a time, retried a few times.
*/
func (RollbackArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: 3,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRetryable, queue.StateRunning, queue.StateScheduled,
		}},
	}
}

/*
mayRollBack refuses a caller who may not undo imports.
*/
func mayRollBack(m authz.Membership) error {
	if !m.Can("contacts:write") || !m.Can("imports:rollback") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
RollbackDeadline returns until when the import can be undone, nil when
it has not finished.
*/
func RollbackDeadline(finishedAt *time.Time) *time.Time {
	if finishedAt == nil {
		return nil
	}

	return new(finishedAt.Add(RollbackWindow))
}

/*
Rollback queues the undoing of a finished import.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[ErrNotFinished], [ErrRollbackClosed], [ErrRolledBack],
[ErrNothingToUndo], or a wrapped error.
*/
func (a *Imports) Rollback(ctx context.Context, in Input) (View, error) {
	if err := mayRollBack(in.Membership); err != nil {
		return View{}, err
	}

	var out View

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := find(ctx, q, in)
		if err != nil {
			return err
		}

		if row, err = a.requestRollback(ctx, q, row, in.Membership); err != nil {
			return err
		}

		if err := a.Queue.PushTx(ctx, tx, RollbackArgs{WorkspaceID: row.WorkspaceID, ImportID: row.ID}); err != nil {
			return fmt.Errorf("queue import rollback: %w", err)
		}

		out, err = describeImport(ctx, q, row)

		return err
	})

	return out, err
}

/*
requestRollback marks the import as waiting to be undone, once it is
sure there is something to undo and it still can be.
*/
func (a *Imports) requestRollback(
	ctx context.Context, q *models.Queries, row models.Import, m authz.Membership,
) (models.Import, error) {
	if err := a.undoable(row); err != nil {
		return row, err
	}

	total, err := q.CountImportChangedRecords(ctx, models.CountImportChangedRecordsParams{
		WorkspaceID: row.WorkspaceID, ImportID: row.ID,
	})
	if err != nil {
		return row, fmt.Errorf("count import changes: %w", err)
	}

	if total == 0 {
		return row, ErrNothingToUndo
	}

	row, err = q.RequestImportRollback(ctx, models.RequestImportRollbackParams{
		Total: total, MemberID: &m.Member.ID, Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, ErrRolledBack
	}

	if err != nil {
		return row, fmt.Errorf("request import rollback: %w", err)
	}

	return row, nil
}

/*
undoable refuses an import that is still running, was undone already,
or finished too long ago.
*/
func (a *Imports) undoable(row models.Import) error {
	if row.FinishedAt == nil || (row.Status != "completed" && row.Status != "failed") {
		return ErrNotFinished
	}

	if row.RollbackStatus != nil && *row.RollbackStatus != "failed" {
		return ErrRolledBack
	}

	if a.Clock.Now().After(*RollbackDeadline(row.FinishedAt)) {
		return ErrRollbackClosed
	}

	return nil
}

/*
RunRollback undoes a queued import to the end. An undo gone or already
over is left alone; one whose teammate may no longer undo fails.
*/
func (a *Imports) RunRollback(ctx context.Context, args RollbackArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())
	q := models.New(a.DB)

	row, err := q.StartImportRollback(ctx, models.StartImportRollbackParams{
		Now: a.Clock.Now(), WorkspaceID: args.WorkspaceID, ID: args.ImportID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("start import rollback: %w", err)
	}

	if err := a.undoRecords(ctx, q, row); err != nil {
		message := reason(err)
		if message == "could not be saved" {
			message = "The undo stopped unexpectedly."
		}

		return errors.Join(err, q.FailImportRollback(ctx, models.FailImportRollbackParams{
			Error: &message, Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		}))
	}

	err = q.FinishImportRollback(ctx, models.FinishImportRollbackParams{
		Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if err != nil {
		return fmt.Errorf("finish import rollback: %w", err)
	}

	return nil
}

/*
undoRecords undoes the records not yet saved, a batch at a time.
*/
func (a *Imports) undoRecords(ctx context.Context, q *models.Queries, row models.Import) error {
	if row.RollbackByMemberID == nil {
		return exceptions.ErrForbidden
	}

	m, err := memberOf(ctx, q, row.WorkspaceID, *row.RollbackByMemberID)
	if err != nil {
		return err
	}

	if err := mayRollBack(m); err != nil {
		return err
	}

	changes, err := q.ListImportChanges(ctx, models.ListImportChangesParams{
		WorkspaceID: row.WorkspaceID, ImportID: row.ID,
	})
	if err != nil {
		return fmt.Errorf("list import changes: %w", err)
	}

	records, err := group(changes)
	if err != nil {
		return err
	}

	u := undoer{imports: a, m: m, kind: row.Kind}
	kept := int(row.RollbackKept)

	for start := int(row.RollbackProcessed); start < len(records); start += batch {
		t := tally{processed: min(start+batch, len(records)), keptBefore: kept}

		for _, r := range records[start:min(start+batch, len(records))] {
			if err := t.add(ctx, u, r); err != nil {
				return err
			}
		}

		kept += t.kept
		if err := a.saveRollback(ctx, q, row, t); err != nil {
			return err
		}
	}

	return nil
}

/*
KeptRecord is a record an undo left as it is, and why.
*/
type KeptRecord struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

/*
tally is what one batch of an undo came to.
*/
type tally struct {
	removed, reverted, kept int
	records                 []KeptRecord
	processed, keptBefore   int
}

/*
add undoes one record and counts it.
*/
func (t *tally) add(ctx context.Context, u undoer, r undoRecord) error {
	o, err := u.undo(ctx, r)
	if err != nil {
		return err
	}

	switch o.outcome {
	case undoRemoved:
		t.removed++
	case undoReverted:
		t.reverted++
	case undoKept:
		t.kept++
		if t.keptBefore+len(t.records) < keptShown {
			t.records = append(t.records, KeptRecord{ID: u.publicID(r.id), Name: o.name, Reason: o.reason})
		}
	}

	return nil
}

/*
saveRollback adds a batch's counts and kept records to the import.
*/
func (a *Imports) saveRollback(ctx context.Context, q *models.Queries, row models.Import, t tally) error {
	kept, err := json.Marshal(append([]KeptRecord{}, t.records...))
	if err != nil {
		return fmt.Errorf("encode kept records: %w", err)
	}

	err = q.RecordImportRollbackProgress(ctx, models.RecordImportRollbackProgressParams{
		Processed: int32Of(t.processed), Removed: int32Of(t.removed), Reverted: int32Of(t.reverted),
		Kept: int32Of(t.kept), KeptRecords: kept, Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if err != nil {
		return fmt.Errorf("record import rollback progress: %w", err)
	}

	return nil
}
