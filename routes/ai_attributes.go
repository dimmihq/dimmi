/*
|--------------------------------------------------------------------------
| Conversation Attribute Routes
|--------------------------------------------------------------------------
|
| The API of conversation attributes. Reading takes the
| conversations:read ability; defining them takes attributes:write and
| setting a conversation's values conversations:write, and the actions
| check the member may. The AI conversation attribute routes came first
| and keep working until their sunset; the conversation attribute
| routes replace them.
|
*/

package routes

import (
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
aiAttributesDeprecation is when the AI conversation attribute routes
gave way to the conversation attribute routes, and when they end.
*/
func aiAttributesDeprecation() routing.Deprecation {
	return routing.Deprecation{
		Since:  time.Date(2026, time.November, 26, 0, 0, 0, 0, time.UTC),
		Sunset: time.Date(2027, time.December, 1, 0, 0, 0, 0, time.UTC),
		Link:   "https://trydimmi.com/docs/inbox/conversation-attributes",
	}
}

/*
aiAttributeRoutes declares the conversation attributes API.
*/
func aiAttributeRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.AIAttributes == nil {
		return
	}

	conversationAttributeRoutes(workspace, a)

	c := &v1.AIAttributeController{Attributes: a.Actions.AIAttributes, IDs: a.IDs}
	all, one := "/ai/conversation-attributes", "/ai/conversation-attributes/{attribute}"

	routing.Get(workspace, all, "ai.conversation-attributes.index", c.Index).Ability("conversations:read").
		Deprecated(aiAttributesDeprecation())
	routing.Post(workspace, all, "ai.conversation-attributes.store", c.Store).Ability("ai:write").
		Status(fhttp.StatusCreated).Deprecated(aiAttributesDeprecation())
	routing.Patch(workspace, one, "ai.conversation-attributes.update", c.Update).Ability("ai:write").
		Deprecated(aiAttributesDeprecation())
	routing.Get(workspace, "/conversations/{conversation}/ai-attributes", "conversations.ai-attributes.index",
		c.Conversation).Ability("conversations:read").Deprecated(aiAttributesDeprecation())
}

/*
conversationAttributeRoutes declares the conversation attribute routes.
*/
func conversationAttributeRoutes(workspace *routing.Router, a *app.App) {
	c := &v1.ConversationAttributeController{Attributes: a.Actions.AIAttributes, IDs: a.IDs}
	all, one := "/conversation-attributes", "/conversation-attributes/{attribute}"
	values := "/conversations/{conversation}/attributes"

	routing.Get(workspace, all, "conversation-attributes.index", c.Index).Ability("conversations:read")
	routing.Post(workspace, all, "conversation-attributes.store", c.Store).Ability("attributes:write").
		Status(fhttp.StatusCreated)
	routing.Patch(workspace, one, "conversation-attributes.update", c.Update).Ability("attributes:write")
	routing.Get(workspace, values, "conversations.attributes.index", c.Values).Ability("conversations:read")
	routing.Patch(workspace, values, "conversations.attributes.update", c.SetValues).
		Ability("conversations:write")
}
