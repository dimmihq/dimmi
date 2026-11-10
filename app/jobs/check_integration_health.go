/*
|--------------------------------------------------------------------------
| Check Integration Health
|--------------------------------------------------------------------------
|
| Installations break quietly: a token revoked by hand, an app removed
| while its uninstall event was lost. Every hour the sweep checks each
| installation not checked within the health interval, so a broken one
| is found within a day and teammates hear of it before a feature fails.
|
*/

package jobs

import (
	"context"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/integrations"
)

/*
healthAttempts is how often the sweep is tried; the next run catches up
anyway.
*/
const healthAttempts = 3

/*
CheckIntegrationHealthArgs carries nothing: every run checks everything
due.
*/
type CheckIntegrationHealthArgs struct{}

/*
Kind names the job.
*/
func (CheckIntegrationHealthArgs) Kind() string {
	return "check_integration_health"
}

/*
Options try the job a few times.
*/
func (CheckIntegrationHealthArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: healthAttempts}
}

/*
CheckIntegrationHealth checks the installations that are due.
*/
type CheckIntegrationHealth struct {
	Check *integrations.CheckInstallationHealth
}

/*
Handle runs the sweep.
*/
func (w *CheckIntegrationHealth) Handle(ctx context.Context, _ *queue.Job[CheckIntegrationHealthArgs]) error {
	return w.Check.Due(ctx)
}
