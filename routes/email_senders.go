/*
|--------------------------------------------------------------------------
| Email Sender Routes
|--------------------------------------------------------------------------
|
| How the workspace's mail leaves: its sending domains. Reading them
| takes email:read, changing them email:manage, and a new signing key
| email:rotate_dkim; each action checks its own permission, the routes
| the token abilities. Checks are queued and answered at once.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/email"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
emailSenderRoutes declares the workspace's sending domains.
*/
func emailSenderRoutes(workspace *routing.Router, a *app.App) {
	c := emailDomainController(a)

	routing.Get(workspace, "/email-domains", "email-domains.index", c.Index).Ability("settings:read")
	routing.Post(workspace, "/email-domains", "email-domains.store", c.Store).
		Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, "/email-domains/{domain}", "email-domains.show", c.ShowDomain).Ability("settings:read")
	routing.Patch(workspace, "/email-domains/{domain}", "email-domains.update", c.Update).Ability("settings:write")
	routing.Delete(workspace, "/email-domains/{domain}", "email-domains.destroy", c.Destroy).
		Ability("settings:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, "/email-domains/{domain}/checks", "email-domains.checks.store", c.StoreCheck).
		Ability("settings:write").Status(fhttp.StatusAccepted)
	routing.Post(workspace, "/email-domains/{domain}/dns-setups", "email-domains.dns-setups.store",
		c.StoreDNSSetup).Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Post(workspace, "/email-domains/{domain}/dkim-rotations", "email-domains.dkim-rotations.store",
		c.StoreRotation).Ability("settings:write").Status(fhttp.StatusCreated)
}

/*
emailDomainController builds the controller of the workspace's sending
domains.
*/
func emailDomainController(a *app.App) *v1.EmailDomainController {
	d := a.EmailDomains

	var returns email.ReturnPathSetter
	if setter, ok := d.Provider.(email.ReturnPathSetter); ok {
		returns = setter
	}

	return &v1.EmailDomainController{
		Create: &email.CreateEmailDomain{Domains: d}, List: &email.ListEmailDomains{DB: a.DB},
		Show:   &email.ShowEmailDomain{DB: a.DB},
		Check:  &email.RequestEmailDomainCheck{DB: a.DB, Queue: a.Queue, Limiter: a.Limiter, Clock: a.Clock},
		Change: &email.UpdateEmailDomain{Domains: d, Returns: returns},
		Remove: &email.DeleteEmailDomain{Domains: d},
		Rotate: &email.RotateEmailDomainKey{Domains: d},
		Setups: &email.DNSSetups{Domains: d, HTTP: a.HTTP, Cache: a.Cache, Base: a.Application.Config.App.URL},
		Hosts:  d.HostHint, IDs: a.IDs, Base: a.Application.Config.App.URL,
	}
}
