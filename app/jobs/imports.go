/*
|--------------------------------------------------------------------------
| Import Jobs
|--------------------------------------------------------------------------
|
| A started import of customers or companies runs in the background,
| scoped to its workspace, and is retried when it fails; a retry picks
| up after the last saved batch. Undoing an import runs the same way,
| and a nightly sweep empties old files.
|
*/

package jobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/imports"
)

/*
RunImport runs started imports.
*/
type RunImport struct {
	Imports *imports.Imports
}

/*
Handle runs the import.
*/
func (w *RunImport) Handle(ctx context.Context, job *queue.Job[imports.RunArgs]) error {
	return w.Imports.Run(ctx, job.Args)
}

/*
RollBackImport undoes imports a teammate asked to undo.
*/
type RollBackImport struct {
	Imports *imports.Imports
}

/*
Handle runs the undo.
*/
func (w *RollBackImport) Handle(ctx context.Context, job *queue.Job[imports.RollbackArgs]) error {
	return w.Imports.RunRollback(ctx, job.Args)
}

/*
ExpireImportsArgs names the nightly sweep of import files; it takes no
arguments.
*/
type ExpireImportsArgs struct{}

/*
Kind names the job.
*/
func (ExpireImportsArgs) Kind() string { return "expire_imports" }

/*
Options run one round a day, tried three times.
*/
func (ExpireImportsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 3, Unique: queue.UniqueOpts{ByPeriod: 24 * time.Hour}}
}

/*
ExpireImports empties old import files and drops changes past undoing.
*/
type ExpireImports struct {
	Expire *imports.ExpireImports
}

/*
Handle expires a batch, and runs again at once while a batch did any
work.
*/
func (w *ExpireImports) Handle(ctx context.Context, _ *queue.Job[ExpireImportsArgs]) error {
	for {
		done, err := w.Expire.Handle(ctx)
		if err != nil || done == 0 {
			return err
		}
	}
}
