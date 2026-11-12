/*
|--------------------------------------------------------------------------
| Help Center Site Routes
|--------------------------------------------------------------------------
|
| Adding and deleting a workspace's help centers, the custom domain a
| help center answers on, and what its readers did. Adding one and its
| domain need help_centers:manage; deleting one is the owner's
| help_centers:delete, checked by the action with the name typed back.
| The routing probe a domain's check reaches answers on every host.
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
portalRoutes declares what a help center's portal holds beside its
articles: the changelog, feature requests, and the site itself.
*/
func portalRoutes(workspace *routing.Router, a *app.App) {
	changelogRoutes(workspace, a)
	feedbackRoutes(workspace, a)
	helpCenterSiteRoutes(workspace, a)
}

/*
helpCenterSiteRoutes declares help centers' lifecycle, domain and
analytics API.
*/
func helpCenterSiteRoutes(workspace *routing.Router, a *app.App) {
	acts := a.Actions
	if acts.HelpDomains == nil {
		return
	}
	c := &v1.HelpCenterSiteController{
		HelpCenters: acts.HelpCenters, Domains: acts.HelpDomains, Counts: acts.HelpAnalytics,
		Addresses: acts.HelpCenterAddresses, IDs: a.IDs,
	}
	one := "/help-centers/{help_center}"

	routing.Post(workspace, "/help-centers", "help-centers.store", c.Store).
		Ability("settings:write").Status(fhttp.StatusCreated)
	routing.Delete(workspace, one, "help-centers.destroy", c.Destroy).
		Ability("settings:write").Status(fhttp.StatusNoContent)
	routing.Get(workspace, one+"/domain", "help-centers.domain.show", c.ShowDomain).Ability("articles:read")
	routing.Patch(workspace, one+"/domain", "help-centers.domain.update", c.StoreDomain).Ability("settings:write")
	routing.Post(workspace, one+"/domain/checks", "help-centers.domain.checks.store", c.CheckDomain).
		Ability("settings:write").Status(fhttp.StatusOK)
	routing.Delete(workspace, one+"/domain", "help-centers.domain.destroy", c.DestroyDomain).
		Ability("settings:write").Status(fhttp.StatusNoContent)
	routing.Get(workspace, one+"/analytics", "help-centers.analytics.show", c.Analytics).Ability("articles:read")
}

/*
helpDomainProbe answers the routing probe of help centers' custom
domains, on any host.
*/
func helpDomainProbe(a *app.App) {
	if a.Actions.HelpDomains == nil {
		return
	}
	probe := a.Actions.HelpDomains.Probe.Handler()
	a.Router.Handle(fhttp.MethodGet, a.Actions.HelpDomains.Probe.Path+"/{token}", "help-domains.probe", probe)
}
