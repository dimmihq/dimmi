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
address. CustomDomainTarget is the hostname a workspace's own domain,
such as help.acme.com, points at with a CNAME; empty means custom.<base
domain>, or the app's own host without a base domain.
*/
type HelpCentersConfig struct {
	BaseDomain         string `koanf:"base_domain"`
	CustomDomainTarget string `koanf:"custom_domain_target"`
}

func (c *HelpCentersConfig) settle(edition Edition) {
	if edition == EditionCloud && c.BaseDomain == "" {
		c.BaseDomain = cloudHelpDomain
	}
	c.BaseDomain = strings.ToLower(strings.Trim(strings.TrimSpace(c.BaseDomain), "."))
	c.CustomDomainTarget = strings.ToLower(strings.Trim(strings.TrimSpace(c.CustomDomainTarget), "."))
	if c.CustomDomainTarget == "" && c.BaseDomain != "" {
		c.CustomDomainTarget = "custom." + c.BaseDomain
	}
}
