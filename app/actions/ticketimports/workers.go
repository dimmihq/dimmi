/*
|--------------------------------------------------------------------------
| Ticket Import Workers
|--------------------------------------------------------------------------
|
| Undoing an import of tickets runs in the background, scoped to its
| workspace, and is retried when it fails.
|
*/

package ticketimports

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"
)

/*
RollbackWorker undoes ticket imports a member asked to undo.
*/
type RollbackWorker struct {
	Importer *Importer
}

/*
Handle runs the undo.
*/
func (w *RollbackWorker) Handle(ctx context.Context, job *queue.Job[RollbackArgs]) error {
	return w.Importer.RunRollback(ctx, job.Args)
}
