/*
|--------------------------------------------------------------------------
| AI Trace Outputs
|--------------------------------------------------------------------------
|
| The run and usage response bodies the inbox's why panel and the
| settings' usage tab return.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
AIRunsOutput is a conversation's run list.
*/
type AIRunsOutput struct {
	Body fhttp.Collection[resources.AiRun]
}

/*
AIRunOutput is one run with its steps.
*/
type AIRunOutput struct {
	Body fhttp.Resource[resources.AiRun]
}

/*
AIUsageOutput is the month's usage view.
*/
type AIUsageOutput struct {
	Body fhttp.Resource[resources.AiUsage]
}
