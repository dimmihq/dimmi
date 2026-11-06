/*
|--------------------------------------------------------------------------
| AI Controller Outputs
|--------------------------------------------------------------------------
|
| The typed response bodies the agent's endpoints return.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
AIAgentOutput is the agent view for the settings page.
*/
type AIAgentOutput struct {
	Body fhttp.Resource[resources.AIAgentView]
}

/*
AIKeysOutput is the connection list.
*/
type AIKeysOutput struct {
	Body fhttp.Collection[resources.AIProviderKey]
}

/*
AIKeyOutput is one connection.
*/
type AIKeyOutput struct {
	Body fhttp.Resource[resources.AIProviderKey]
}

/*
AIConnectOutput is the authorize answer.
*/
type AIConnectOutput struct {
	Body fhttp.Resource[ai.StartAnswer]
}

/*
AIConnectDoneOutput is the completed connection.
*/
type AIConnectDoneOutput struct {
	Body fhttp.Resource[resources.AIChatGPTConnection]
}
