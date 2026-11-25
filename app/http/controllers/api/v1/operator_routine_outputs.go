/*
|--------------------------------------------------------------------------
| Operator Routine Outputs
|--------------------------------------------------------------------------
|
| What the routine routes answer besides a routine: the templates and
| the runs.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
OperatorRoutineTemplatesOutput is the routine templates.
*/
type OperatorRoutineTemplatesOutput struct {
	Body fhttp.Collection[resources.OperatorRoutineTemplate]
}

/*
OperatorRoutineRunsOutput is a routine's runs.
*/
type OperatorRoutineRunsOutput struct {
	Body fhttp.Collection[resources.OperatorRoutineRun]
}

/*
OperatorRoutineRunOutput is one run of a routine.
*/
type OperatorRoutineRunOutput struct {
	Body fhttp.Resource[resources.OperatorRoutineRun]
}
