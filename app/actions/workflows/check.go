/*
|--------------------------------------------------------------------------
| Checking A Working Copy
|--------------------------------------------------------------------------
|
| Everything publishing would refuse or warn about, for a definition that
| is not saved yet: the definition's own errors and warnings, every team,
| member, tag, schedule, connection and called workflow it names, and the
| agent its dimmi step hands to. Operator checks each edit it proposes
| here, so what it suggests is never something publishing would refuse.
|
*/

package workflows

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
CheckDraft reports what publishing a definition on the workflow would
find. The workflow may be a new one not stored yet: its workspace and
trigger are all the check reads of it.
*/
func (w *Workflows) CheckDraft(ctx context.Context, m authz.Membership, wf models.Workflow, spec workflowspec.Spec,
) (workflowspec.Report, error) {
	if !m.Can("workflows:read") {
		return workflowspec.Report{}, exceptions.ErrForbidden
	}

	wf.WorkspaceID = m.Workspace.ID
	report := workflowspec.Check(spec)
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		missing, err := w.references(ctx, q, wf, spec)
		if err != nil {
			return err
		}

		report.Errors = append(report.Errors, missing...)
		agentErrs, warnings, err := w.agentWarnings(ctx, q, wf, spec)
		if err != nil {
			return err
		}

		report.Errors = append(report.Errors, agentErrs...)
		report.Warnings = append(report.Warnings, warnings...)

		return nil
	})

	return report, err
}
