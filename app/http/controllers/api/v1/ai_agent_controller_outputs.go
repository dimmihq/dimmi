/*
|--------------------------------------------------------------------------
| AI Agent Controller Outputs
|--------------------------------------------------------------------------
|
| What the agents API answers with.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
AIAgentsOutput is the workspace's agents.
*/
type AIAgentsOutput struct {
	Body fhttp.Collection[resources.AIAgent]
}

/*
AIAgentTemplatesOutput is the templates a new agent starts from.
*/
type AIAgentTemplatesOutput struct {
	Body fhttp.Collection[resources.AIAgentTemplate]
}

/*
AIAgentVersionsOutput is a page of an agent's versions.
*/
type AIAgentVersionsOutput struct {
	Body fhttp.Collection[resources.AIAgentVersion]
}

/*
AIAgentVersionOutput is one version of an agent.
*/
type AIAgentVersionOutput struct {
	Body fhttp.Resource[resources.AIAgentVersion]
}

/*
AISettingsOutput is the workspace's shared AI settings.
*/
type AISettingsOutput struct {
	Body fhttp.Resource[resources.AISettings]
}
