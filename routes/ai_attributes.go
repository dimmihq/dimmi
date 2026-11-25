/*
|--------------------------------------------------------------------------
| AI Conversation Attribute Routes
|--------------------------------------------------------------------------
|
| The API of AI conversation attributes. Reading takes the
| conversations:read ability; changing takes ai:write, and the actions
| check the member may manage the workspace's AI.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
aiAttributeRoutes declares the AI conversation attributes API.
*/
func aiAttributeRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.AIAttributes == nil {
		return
	}

	c := &v1.AIAttributeController{Attributes: a.Actions.AIAttributes, IDs: a.IDs}
	all, one := "/ai/conversation-attributes", "/ai/conversation-attributes/{attribute}"

	routing.Get(workspace, all, "ai.conversation-attributes.index", c.Index).Ability("conversations:read")
	routing.Post(workspace, all, "ai.conversation-attributes.store", c.Store).Ability("ai:write").
		Status(fhttp.StatusCreated)
	routing.Patch(workspace, one, "ai.conversation-attributes.update", c.Update).Ability("ai:write")
	routing.Get(workspace, "/conversations/{conversation}/ai-attributes", "conversations.ai-attributes.index",
		c.Conversation).Ability("conversations:read")
}
