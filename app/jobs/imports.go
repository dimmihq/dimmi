/*
|--------------------------------------------------------------------------
| Import Jobs
|--------------------------------------------------------------------------
|
| A started import of customers or companies runs in the background,
| scoped to its workspace, and is retried when it fails; a retry picks
| up after the last saved batch.
|
*/

package jobs

import (
	"context"

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
