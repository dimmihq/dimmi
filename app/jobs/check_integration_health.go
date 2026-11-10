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

	"github.com/dimmihq/dimmi/framework/ids"
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

/*
CheckInstallationArgs names one installation to check now, such as one
whose token a provider just refused while sending.
*/
type CheckInstallationArgs struct {
	WorkspaceID    ids.UUID `json:"workspace_id"`
	InstallationID ids.UUID `json:"installation_id"`
}

/*
Kind names the job.
*/
func (CheckInstallationArgs) Kind() string {
	return "check_integration_installation"
}

/*
Options try the job a few times, once per installation at a time.
*/
func (CheckInstallationArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: healthAttempts, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
CheckInstallation checks one installation, so a refused token marks it
broken and alerts the admins at once instead of at the next sweep.
*/
type CheckInstallation struct {
	Check *integrations.CheckInstallationHealth
}

/*
Handle checks the installation the job names.
*/
func (w *CheckInstallation) Handle(ctx context.Context, job *queue.Job[CheckInstallationArgs]) error {
	return w.Check.Handle(ctx, job.Args.WorkspaceID, job.Args.InstallationID)
}
