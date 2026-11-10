/*
|--------------------------------------------------------------------------
| Purge Workspace
|--------------------------------------------------------------------------
|
| A workspace whose time in the trash ran out is deleted for good: its
| files, then its rows a batch at a time, then the workspace itself, and
| its organization is marked deleted once it holds no live workspace.
| The retention guard is asked first; a held workspace stays in the
| trash, untouched, and is asked about again on the next daily round.
|
| Once the purge begins, the workspace can no longer be restored, even
| if this run stops halfway. A run that has worked for a while hands
| over to a fresh run of itself, which carries on where it stopped, so
| a large workspace never holds a worker or a lock for long, and a run
| that crashed is simply run again.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on one run of a purge: the rows one statement deletes, how long
a run works before handing over to the next, how long the next waits,
and how often a failing purge is tried.
*/
const (
	purgeBatchRows = 5000
	purgeRunBudget = 20 * time.Second
	purgeHandOver  = time.Second
	purgeAttempts  = 10
)

/*
WorkspaceStoragePrefix returns the folder of the default disk that holds
every file of the workspace, which a purge deletes with it.
*/
func WorkspaceStoragePrefix(workspaceID ids.UUID) string {
	return "workspaces/" + workspaceID.String() + "/"
}

/*
PurgeWorkspaceArgs names the workspace to purge.
*/
type PurgeWorkspaceArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
}

/*
Kind names the job.
*/
func (PurgeWorkspaceArgs) Kind() string {
	return "purge_workspace"
}

/*
Options run one purge per workspace at a time, on the maintenance
queue, retried for a while when the database or the disk fails.
*/
func (PurgeWorkspaceArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: purgeAttempts,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRetryable, queue.StateRunning, queue.StateScheduled,
		}},
	}
}

/*
PurgeWorkspace deletes a workspace in the trash for good. Revoke, when
set, gives the workspace's integration tokens back to their providers
before any row goes.
*/
type PurgeWorkspace struct {
	DB        *database.DB
	Clock     clock.Clock
	Disk      filesystem.Filesystem
	Retention contracts.RetentionGuard
	Log       log.Logger
	Revoke    func(ctx context.Context, workspaceID ids.UUID) error
}

/*
Handle purges the workspace, or as much of it as one run may, and hands
over to the next run when there is more.
*/
func (w *PurgeWorkspace) Handle(ctx context.Context, job *queue.Job[PurgeWorkspaceArgs]) error {
	id := job.Args.WorkspaceID
	ctx = tenancy.With(ctx, id.String())

	due, err := w.begin(ctx, id)
	if err != nil || !due {
		return err
	}

	if err := w.revoke(ctx, id); err != nil {
		return handOver(err)
	}

	deadline := w.Clock.Now().Add(purgeRunBudget)

	done, err := w.deleteFiles(ctx, id, deadline)
	if err != nil || !done {
		return handOver(err)
	}

	done, err = w.deleteRows(ctx, id, deadline)
	if err != nil || !done {
		return handOver(err)
	}

	return w.finish(ctx, id)
}

/*
Purge purges the workspace in one go, for an operator who waits for it
at the command line, and reports whether it is gone. A workspace that
is not due, or that the retention guard holds, is left as it is.
*/
func (w *PurgeWorkspace) Purge(ctx context.Context, id ids.UUID) (bool, error) {
	ctx = tenancy.With(ctx, id.String())

	due, err := w.begin(ctx, id)
	if err != nil || !due {
		return false, err
	}

	if err := w.revoke(ctx, id); err != nil {
		return false, err
	}

	for _, step := range []func(context.Context, ids.UUID, time.Time) (bool, error){w.deleteFiles, w.deleteRows} {
		for done := false; !done; {
			done, err = step(ctx, id, w.Clock.Now().Add(purgeRunBudget))
			if err != nil {
				return false, err
			}
		}
	}

	return true, w.finish(ctx, id)
}

/*
revoke gives the workspace's integration tokens back, when the purge
was given a way to.
*/
func (w *PurgeWorkspace) revoke(ctx context.Context, id ids.UUID) error {
	if w.Revoke == nil {
		return nil
	}

	if err := w.Revoke(ctx, id); err != nil {
		return fmt.Errorf("revoke integrations: %w", err)
	}

	return nil
}

/*
begin reports whether the workspace is due for purging and may be
destroyed, and marks its purge as started so it can no longer be
restored. A workspace gone already, restored, or not yet due is left
alone; one the retention guard holds is logged as held.
*/
func (w *PurgeWorkspace) begin(ctx context.Context, id ids.UUID) (bool, error) {
	q := models.New(w.DB)

	ws, err := q.FindWorkspace(ctx, id)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find workspace: %w", err)
	}

	if ws.DeletedAt == nil || ws.PurgeAfter == nil || ws.PurgeAfter.After(w.Clock.Now()) {
		return false, nil
	}

	subject := contracts.RetentionSubject{Kind: contracts.RetentionWorkspace, ID: id}

	allowed, err := w.Retention.MayDestroy(ctx, id, subject)
	if err != nil {
		return false, fmt.Errorf("ask retention guard: %w", err)
	}

	if !allowed {
		w.Log.InfoContext(ctx, "workspace purge held",
			log.Bool("retention_held", true), log.String("workspace_id", id.String()))

		return false, nil
	}

	started := models.MarkWorkspacePurgeStartedParams{ID: id, Now: w.Clock.Now()}
	if _, err := q.MarkWorkspacePurgeStarted(ctx, started); err != nil {
		return false, fmt.Errorf("mark purge started: %w", err)
	}

	return true, nil
}

/*
deleteFiles deletes the workspace's files until none are left, or the
run's time is up, and reports whether none are left.
*/
func (w *PurgeWorkspace) deleteFiles(ctx context.Context, id ids.UUID, deadline time.Time) (bool, error) {
	for entry, err := range w.Disk.List(ctx, WorkspaceStoragePrefix(id)) {
		if err != nil {
			return false, fmt.Errorf("list workspace files: %w", err)
		}

		if err := w.Disk.Delete(ctx, entry.Path); err != nil {
			return false, fmt.Errorf("delete workspace file: %w", err)
		}

		if w.Clock.Now().After(deadline) {
			return false, nil
		}
	}

	return true, nil
}

/*
deleteRows deletes the workspace's rows a batch at a time until none
are left, or the run's time is up, and reports whether none are left.
*/
func (w *PurgeWorkspace) deleteRows(ctx context.Context, id ids.UUID, deadline time.Time) (bool, error) {
	q := models.New(w.DB)

	for {
		removed, err := q.PurgeWorkspaceRows(ctx, models.PurgeWorkspaceRowsParams{
			WorkspaceID: id, BatchSize: purgeBatchRows,
		})
		if err != nil {
			return false, fmt.Errorf("purge workspace rows: %w", err)
		}

		if removed == 0 {
			return true, nil
		}

		if w.Clock.Now().After(deadline) {
			return false, nil
		}
	}
}

/*
finish deletes the workspace row, and with it whatever still points at
it, and retires its organization when no live workspace is left in it.
*/
func (w *PurgeWorkspace) finish(ctx context.Context, id ids.UUID) error {
	return w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		ws, err := q.LockWorkspace(ctx, id)
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		if err := q.DeleteWorkspace(ctx, id); err != nil {
			return fmt.Errorf("delete workspace: %w", err)
		}

		err = q.RetireOrganization(ctx, models.RetireOrganizationParams{ID: ws.OrganizationID, Now: w.Clock.Now()})
		if err != nil {
			return fmt.Errorf("retire organization: %w", err)
		}

		return nil
	})
}

/*
handOver returns the error of a failed run, or asks for a fresh run to
carry on when this one ran out of time.
*/
func handOver(err error) error {
	if err != nil {
		return err
	}

	return queue.Postpone(purgeHandOver)
}
