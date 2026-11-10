/*
|--------------------------------------------------------------------------
| Slack Room Routes
|--------------------------------------------------------------------------
|
| The Slack channels a connection watches and how each is watched. Both
| are settings: reading takes settings:read, changing a room takes
| settings:write, and the action checks channels:manage.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
slackRoomRoutes declares the routes of a Slack connection's rooms.
*/
func slackRoomRoutes(workspace *routing.Router, a *app.App) {
	c := &v1.SlackRoomController{Slack: a.Actions.Slack.Channel, IDs: a.IDs}

	routing.Get(workspace, "/channels/{channel}/rooms", "channels.rooms.index", c.Index).Ability("settings:read")
	routing.Patch(workspace, "/channels/{channel}/rooms/{room}", "channels.rooms.update", c.Update).
		Ability("settings:write")
}
