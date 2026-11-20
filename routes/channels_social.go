/*
|--------------------------------------------------------------------------
| Messenger And Instagram Channel Routes
|--------------------------------------------------------------------------
|
| The dashboard's routes for the two channels, the same set under each
| platform's name. All are internal: they serve the settings pages and
| the composer, and are not part of the public API.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/channels"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
socialChannelRoutes declares the Messenger and Instagram routes.
*/
func socialChannelRoutes(workspace *routing.Router, a *app.App) {
	platforms := []struct {
		name    string
		channel *channels.Social
	}{{"facebook", a.Actions.Facebook}, {"instagram", a.Actions.Instagram}}

	for _, p := range platforms {
		c := &v1.SocialChannelController{Channel: p.channel, IDs: a.IDs}
		base, name := "/channels/"+p.name, "channels."+p.name

		routing.Get(workspace, base, name+".index", c.Index).Internal().Ability("settings:read")
		routing.Get(workspace, base+"/signup-config", name+".signup-config", c.SignupConfig).Internal().
			Ability("settings:read")
		routing.Post(workspace, base+"/signups", name+".signups", c.Signup).Internal().
			Ability("settings:write").Middleware("verified").Status(fhttp.StatusOK)
		routing.Post(workspace, base, name+".store", c.Store).Internal().
			Ability("settings:write").Middleware("verified").Status(fhttp.StatusCreated)
		routing.Patch(workspace, base+"/{channel}", name+".update", c.Update).Internal().Ability("settings:write")
		routing.Get(workspace, base+"/{channel}/check", name+".check", c.Check).Internal().Ability("settings:read")
		routing.Delete(workspace, base+"/{channel}", name+".destroy", c.Destroy).Internal().
			Ability("settings:write").Status(fhttp.StatusNoContent)
		routing.Get(workspace, "/conversations/{conversation}/"+p.name, "conversations."+p.name+".show",
			c.Conversation).Internal().Ability("conversations:read")
	}
}
