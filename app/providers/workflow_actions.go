/*
|--------------------------------------------------------------------------
| Workflow Wiring
|--------------------------------------------------------------------------
|
| The actions behind a workspace's workflows.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/workflows"
)

/*
bootContent builds the help center and workflow actions.
*/
func bootContent(a *app.App) {
	bootHelpCenters(a)
	bootWorkflows(a)
}

/*
bootWorkflows builds the workflow actions.
*/
func bootWorkflows(a *app.App) {
	a.Actions.Workflows = &workflows.Workflows{DB: a.DB, Clock: a.Clock, IDs: a.IDs}
}
