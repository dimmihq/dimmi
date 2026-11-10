/*
|--------------------------------------------------------------------------
| Slack Channel Routes
|--------------------------------------------------------------------------
|
| Connecting a workspace's own Slack app and setting its secrets. Both
| are settings, so tokens need settings:write and the action checks
| channels:manage. Connecting reaches another service, so it needs a
| verified email where the install asks for one.
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
slackChannelRoutes declares the routes that connect Slack apps.
*/
func slackChannelRoutes(workspace *routing.Router, a *app.App) {
	c := &v1.SlackChannelController{Slack: a.Actions.Slack.Channel, IDs: a.IDs}

	routing.Post(workspace, "/channels/slack", "channels.slack.store", c.Store).
		Ability("settings:write").Middleware("verified").Status(fhttp.StatusCreated)
	routing.Post(workspace, "/channels/{channel}/credentials", "channels.credentials.store", c.StoreCredentials).
		Ability("settings:write").Middleware("verified").Status(fhttp.StatusOK)
}
