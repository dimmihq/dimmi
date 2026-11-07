/*
|--------------------------------------------------------------------------
| Help Center Settings
|--------------------------------------------------------------------------
|
| Where readers reach public docs. With a base domain, each help center
| answers at its own subdomain of it, a domain apart from the app so a
| help center page never shares the app's cookies; the cloud uses
| dimmi.help. Without one, as most self-hosted installs run, help
| centers answer under the app's address at /hc/<subdomain>.
|
*/

package config

import "strings"

/*
cloudHelpDomain is the cloud's base domain for help centers.
*/
const cloudHelpDomain = "dimmi.help"

/*
HelpCentersConfig configures public docs. BaseDomain is the domain whose
subdomains serve help centers, empty to serve them under the app's
address.
*/
type HelpCentersConfig struct {
	BaseDomain string `koanf:"base_domain"`
}

func (c *HelpCentersConfig) settle(edition Edition) {
	if edition == EditionCloud && c.BaseDomain == "" {
		c.BaseDomain = cloudHelpDomain
	}
	c.BaseDomain = strings.ToLower(strings.Trim(strings.TrimSpace(c.BaseDomain), "."))
}
