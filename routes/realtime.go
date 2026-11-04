/*
|--------------------------------------------------------------------------
| Realtime Routes
|--------------------------------------------------------------------------
|
| The API side of realtime. Clients never publish on a channel, so
| whatever they want others to see happen live, such as their user
| typing, goes through a route and the server relays it.
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
realtimeRoutes declares the realtime routes inside the workspace group.
*/
func realtimeRoutes(workspace *routing.Router, a *app.App) {
	typing := &v1.ConversationTypingIndicatorController{RecordTyping: a.Actions.RecordTyping, IDs: a.IDs}

	routing.Post(workspace, "/conversations/{conversation}/typing-indicators",
		"conversations.typing-indicators.store", typing.Store).
		Ability("conversations:write").Status(fhttp.StatusAccepted)
}
