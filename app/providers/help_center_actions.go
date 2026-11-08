/*
|--------------------------------------------------------------------------
| Help Center Wiring
|--------------------------------------------------------------------------
|
| The actions behind a workspace's public docs.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/http/controllers/web"
)

/*
bootHelpCenters builds the help center actions.
*/
func bootHelpCenters(a *app.App) {
	acts := a.Actions
	acts.HelpCenters = &helpcenters.HelpCenters{DB: a.DB, Clock: a.Clock}
	acts.Collections = &helpcenters.Collections{DB: a.DB, Clock: a.Clock}
	acts.Articles = helpcenters.NewArticles(a.DB, a.Clock)
	acts.HelpPublic = helpcenters.NewPublic(a.DB, a.Clock)
	acts.HelpCenterAddresses = helpcenters.Addresses{
		BaseDomain: a.Config.HelpCenters.BaseDomain, AppURL: a.Application.Config.App.URL,
	}
}

/*
registerHelpCenters serves help centers on their own subdomains of the
help domain, when the install has one.
*/
func registerHelpCenters(a *app.App, fa *foundation.Application) {
	if a.Config.HelpCenters.BaseDomain == "" {
		return
	}
	fa.Kernel.Use(web.HostedHelpCenters(helpcenters.Addresses{
		BaseDomain: a.Config.HelpCenters.BaseDomain, AppURL: fa.Config.App.URL,
	}))
}
