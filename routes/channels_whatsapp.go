/*
|--------------------------------------------------------------------------
| WhatsApp Channel Routes
|--------------------------------------------------------------------------
|
| Connecting WhatsApp numbers and managing them are settings: tokens
| need settings:write, the actions check channels:manage, and reaching
| Meta needs a verified email where the install asks for one. In the
| inbox, where a conversation stands on WhatsApp is read with
| conversations:read and a template is sent with conversations:write.
| Every route is the dashboard's own, kept out of the public API.
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
whatsappChannelRoutes declares the routes of WhatsApp numbers.
*/
func whatsappChannelRoutes(workspace *routing.Router, a *app.App) {
	c := &v1.WhatsAppChannelController{
		WhatsApp: a.Actions.WhatsApp, Reply: a.Actions.Inbox.Reply, Service: a.Config.Services.WhatsApp,
		Version: a.Config.Channels.WhatsApp.GraphVersion, IDs: a.IDs,
	}

	routing.Get(workspace, "/channels/whatsapp", "channels.whatsapp.index", c.Index).Internal().
		Ability("settings:read")
	routing.Get(workspace, "/channels/whatsapp/signup-config", "channels.whatsapp.signup-config.index", c.SignupConfig).
		Internal().Ability("settings:read")
	routing.Post(workspace, "/channels/whatsapp/accounts", "channels.whatsapp.accounts.store", c.Account).Internal().
		Ability("settings:write").Middleware("verified").Status(fhttp.StatusOK)
	routing.Post(workspace, "/channels/whatsapp", "channels.whatsapp.store", c.Store).Internal().
		Ability("settings:write").Middleware("verified").Status(fhttp.StatusCreated)
	routing.Post(workspace, "/channels/whatsapp/signups", "channels.whatsapp.signups.store", c.Signup).Internal().
		Ability("settings:write").Middleware("verified").Status(fhttp.StatusCreated)
	routing.Patch(workspace, "/channels/whatsapp/{channel}", "channels.whatsapp.update", c.Update).Internal().
		Ability("settings:write")
	routing.Delete(workspace, "/channels/whatsapp/{channel}", "channels.whatsapp.destroy", c.Destroy).Internal().
		Ability("settings:write").Status(fhttp.StatusNoContent)
	routing.Get(workspace, "/channels/whatsapp/{channel}/templates", "channels.whatsapp.templates.index", c.Templates).
		Internal().Ability("conversations:read")
	routing.Post(workspace, "/channels/whatsapp/{channel}/templates", "channels.whatsapp.templates.store",
		c.StoreTemplate).Internal().Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Patch(workspace, "/channels/whatsapp/{channel}/templates/{template}", "channels.whatsapp.templates.update",
		c.UpdateTemplate).Internal().Ability("settings:write")
	routing.Delete(workspace, "/channels/whatsapp/{channel}/templates/{template}",
		"channels.whatsapp.templates.destroy", c.DestroyTemplate).Internal().Ability("settings:write").
		Status(fhttp.StatusNoContent)
	routing.Get(workspace, "/conversations/{conversation}/whatsapp", "conversations.whatsapp.show",
		c.Conversation).Internal().Ability("conversations:read")
	routing.Post(workspace, "/conversations/{conversation}/whatsapp-templates",
		"conversations.whatsapp-templates.store", c.SendTemplate).Internal().Ability("conversations:write")
}
