/*
|--------------------------------------------------------------------------
| Help Center Domains Wiring
|--------------------------------------------------------------------------
|
| The custom domains help centers answer on, checked through DNS and the
| routing probe and given certificates by the install's TLS setup once
| verified, and the counts of what readers read and search for.
|
*/

package providers

import (
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/customdomain"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
)

/*
bootHelpCenterDomains builds the domain actions and the analytics.
*/
func bootHelpCenterDomains(a *app.App) {
	acts := a.Actions
	target := a.Config.HelpCenters.CustomDomainTarget
	appHost := hostOf(a.Application.Config.App.URL)
	if target == "" {
		target = appHost
	}
	probe := customdomain.Probe{Path: helpcenters.DomainProbePath, Key: a.DerivedKey("help.domain-probe")}
	acts.HelpDomains = &helpcenters.Domains{
		DB: a.DB, Clock: a.Clock, Verifier: customdomain.NewVerifier(a.DNS, a.HTTP, probe), Target: target,
		Reserved: []string{appHost, a.Config.HelpCenters.BaseDomain, target}, Schedule: customdomain.DefaultSchedule(),
		Probe: probe, Resolver: a.DNS,
	}
	if a.Domains != nil {
		acts.HelpDomains.TLS = a.Domains
	}
	acts.HelpDomainHosts = helpcenters.NewDomainHosts(a.DB)
	acts.HelpAnalytics = &helpcenters.Analytics{DB: a.DB, Clock: a.Clock}
}

/*
hostOf returns the host of an address, without its port.
*/
func hostOf(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
