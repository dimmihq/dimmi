/*
|--------------------------------------------------------------------------
| Undoing A Migration
|--------------------------------------------------------------------------
|
| An undo removes everything the migration created and nothing it only
| found: conversations, tickets, articles and contacts go to the trash,
| ticket setup and fields are archived, notes, replies, tags and files
| are deleted. It runs in the background, newest kinds first, so a
| conversation goes before its customer and an article before its
| collection. Something others now use stays: a contact with
| conversations the migration did not bring, a tag still on records, a
| schedule an inbox follows. Once done, the migration takes no more
| records, and a new one can start for the same account.
|
*/

package migrations

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
rollbackPage is how many records one transaction of an undo removes.
*/
const rollbackPage = 200

/*
RollbackArgs names the migration a job undoes.
*/
type RollbackArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	MigrationID ids.UUID `json:"migration_id"`
}

/*
Kind names the job.
*/
func (RollbackArgs) Kind() string { return "roll_back_migration" }

/*
Options run one undo of a migration at a time, retried a few times.
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
Rollback starts undoing the migration in the background.

Rules:
  - The caller needs imports:manage and imports:rollback.
  - A migration is undone once; a failed undo may be started again.

Events: none.

Errors: [exceptions.ErrForbidden], [ErrNotFound], [ErrRollingBack].
*/
func (s *Migrations) Rollback(ctx context.Context, m authz.Membership, id ids.UUID) (Migration, error) {
	if err := CanMigrate(m); err != nil {
		return Migration{}, err
	}

	if !m.Can("imports:rollback") {
		return Migration{}, exceptions.ErrForbidden
	}

	ws := m.Workspace.ID
	ctx = tenancy.With(ctx, ws.String())

	var row models.Migration

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if _, err := q.LockMigration(ctx, models.LockMigrationParams{WorkspaceID: ws, ID: id}); err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return ErrNotFound
			}

			return fmt.Errorf("hold migration: %w", err)
		}

		var err error

		row, err = q.RequestMigrationRollback(ctx, models.RequestMigrationRollbackParams{
			MemberID: m.Member.ID, Now: s.Clock.Now(), WorkspaceID: ws, ID: id,
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrRollingBack
		}

		if err != nil {
			return fmt.Errorf("request migration rollback: %w", err)
		}

		return wrap("queue migration rollback", s.Queue.PushTx(ctx, tx, RollbackArgs{WorkspaceID: ws, MigrationID: id}))
	})
	if err != nil {
		return Migration{}, err
	}

	return s.progress(ctx, row)
}

/*
undoOrder is the order an undo removes kinds in: the newest first.
*/
func undoOrder() []string {
	order := append(Kinds(), kindTicketField, kindTicketState, kindFile)
	slices.Reverse(order)

	out := []string{kindFile}

	for _, k := range order {
		if k != kindFile {
			out = append(out, k)
		}
	}

	return out
}

/*
RunRollback undoes the migration the job names.
*/
func (s *Migrations) RunRollback(ctx context.Context, a RollbackArgs) error {
	ctx = tenancy.With(ctx, a.WorkspaceID.String())

	row, err := s.find(ctx, a.WorkspaceID, a.MigrationID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	if row.Status != "rolling_back" {
		return nil
	}

	removed, runErr := s.undo(ctx, row)

	status, reason := "rolled_back", (*string)(nil)
	if runErr != nil {
		log.From(ctx).ErrorContext(ctx, "migration rollback failed", log.Err(runErr))

		status, reason = "rollback_failed", new("The undo stopped before it finished. Start it again.")
	}

	err = s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).FinishMigrationRollback(ctx, models.FinishMigrationRollbackParams{
			Status: status, Removed: int32(removed), Error: reason, Now: s.Clock.Now(), //nolint:gosec // counted rows
			WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
	})

	return errors.Join(runErr, wrap("finish migration rollback", err))
}

/*
undo removes the migration's created records kind by kind and counts
what it removed.
*/
func (s *Migrations) undo(ctx context.Context, row models.Migration) (int, error) {
	removed := 0

	for _, kind := range undoOrder() {
		for {
			n, more, err := s.undoPage(ctx, row, kind)
			removed += n

			if err != nil {
				return removed, err
			}

			if !more {
				break
			}
		}
	}

	return removed, nil
}

/*
undoPage removes one page of a kind's created records, dropping them
from the ledger, and reports whether more remain.
*/
func (s *Migrations) undoPage(ctx context.Context, row models.Migration, kind string) (int, bool, error) {
	removed, more := 0, false

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		recs, err := q.ListCreatedMigrationRecords(ctx, models.ListCreatedMigrationRecordsParams{
			MigrationID: row.ID, Kind: kind, RowLimit: rollbackPage,
		})
		if err != nil {
			return fmt.Errorf("list created records: %w", err)
		}

		more = len(recs) == rollbackPage

		for _, rec := range recs {
			n, err := q.RemoveMigratedRecord(ctx, models.RemoveMigratedRecordParams{
				Kind: undoKind(kind), ID: *rec.RecordID, WorkspaceID: row.WorkspaceID, Now: s.Clock.Now(),
			})
			if err != nil {
				return fmt.Errorf("remove migrated %s: %w", kind, err)
			}

			removed += int(n)

			if err := q.DeleteMigrationRecord(ctx, models.DeleteMigrationRecordParams{
				MigrationID: row.ID, ID: rec.ID,
			}); err != nil {
				return fmt.Errorf("drop ledger row: %w", err)
			}
		}

		return nil
	})

	return removed, more, err
}

/*
undoKind returns the kind the removal query knows a ledger kind by.
*/
func undoKind(kind string) string {
	return kind
}

/*
RollbackWorker undoes migrations a teammate asked to undo.
*/
type RollbackWorker struct {
	Migrations *Migrations
}

/*
Handle runs the undo.
*/
func (w *RollbackWorker) Handle(ctx context.Context, job *queue.Job[RollbackArgs]) error {
	return w.Migrations.RunRollback(ctx, job.Args)
}
