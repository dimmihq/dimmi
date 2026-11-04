/*
|--------------------------------------------------------------------------
| DNS Hosts
|--------------------------------------------------------------------------
|
| Every DNS host has its own screen and its own quirks: some append the
| domain to a name and some want it whole. Whoever hosts a domain is read
| from its name servers, so the page can open the right screen and show a
| record's name exactly as that host wants it typed. A wrong guess is
| harmless: the link lands in a dashboard, and the customer can pick the
| right host. When nothing matches, the plain records still work.
|
*/

package dns

import (
	"context"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/customdomain"
	fdns "github.com/dimmihq/dimmi/framework/dns"
)

/*
HostOther names the guide for a host we have no instructions for.
*/
const HostOther = "other"

/*
How a host wants a record's name typed: without the domain, or whole.
*/
const (
	hostFormRelative = "relative"
	hostFormFQDN     = "fqdn"
)

/*
Host is one DNS host's instructions. NS holds the name server suffixes
that identify it. DeepLink opens the host's records page, {domain} filled
in. Steps and Quirks are language keys. HostForm says how the host wants a
record's name: relative to the domain, or fully qualified.
*/
type Host struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	NS       []string `json:"ns"`
	DeepLink string   `json:"deep_link"`
	Steps    []string `json:"steps"`
	Quirks   []string `json:"quirks"`
	HostForm string   `json:"host_form"`
}

/*
Hosts returns the known DNS hosts, the generic guide last. Every guide
opens the host's records page, adds the records and checks them; quirks
are what trips people up at that host.
*/
func Hosts() []Host {
	return []Host{
		guide(Host{
			Key: "cloudflare", Name: "Cloudflare", HostForm: hostFormRelative, NS: []string{".ns.cloudflare.com"},
			DeepLink: "https://dash.cloudflare.com/?to=/:account/{domain}/dns/records",
		}, "quirk_proxy"),
		guide(Host{
			Key: "route53", Name: "Amazon Route 53", HostForm: hostFormFQDN, NS: []string{".awsdns-"},
			DeepLink: "https://console.aws.amazon.com/route53/v2/hostedzones",
		}, "quirk_quote"),
		guide(Host{
			Key: "godaddy", Name: "GoDaddy", HostForm: hostFormRelative, NS: []string{".domaincontrol.com"},
			DeepLink: "https://dcc.godaddy.com/control/dnsmanagement?domainName={domain}",
		}, "quirk_root"),
		guide(Host{
			Key: "namecheap", Name: "Namecheap", HostForm: hostFormRelative, NS: []string{".registrar-servers.com"},
			DeepLink: "https://ap.www.namecheap.com/Domains/DomainControlPanel/{domain}/advancedns",
		}, "quirk_name"),
		guide(Host{
			Key: "squarespace", Name: "Squarespace", HostForm: hostFormRelative,
			NS:       []string{".squarespacedns.com", ".googledomains.com"},
			DeepLink: "https://account.squarespace.com/domains/managed/{domain}/dns/dns-settings",
		}),
		guide(Host{
			Key: "vercel", Name: "Vercel", HostForm: hostFormRelative, NS: []string{".vercel-dns.com"},
			DeepLink: "https://vercel.com/dashboard/domains/{domain}",
		}),
		guide(Host{
			Key: "netlify", Name: "Netlify", HostForm: hostFormRelative, NS: []string{".nsone.net", ".netlifydns.com"},
			DeepLink: "https://app.netlify.com/teams/domains/{domain}",
		}),
		guide(Host{
			Key: "porkbun", Name: "Porkbun", HostForm: hostFormRelative, NS: []string{".porkbun.com"},
			DeepLink: "https://porkbun.com/account/domainsSpeedy",
		}),
		guide(Host{
			Key: "hover", Name: "Hover", HostForm: hostFormRelative, NS: []string{".hover.com"},
			DeepLink: "https://www.hover.com/control_panel/domain/{domain}/dns",
		}),
		guide(Host{
			Key: HostOther, Name: "Your DNS host", HostForm: hostFormRelative,
			DeepLink: "",
		}),
	}
}

/*
guide completes a host's instructions with its steps and quirks, as
language keys under email_domains.hosts.
*/
func guide(host Host, quirks ...string) Host {
	prefix := "email_domains.hosts." + host.Key + "."

	host.Quirks = make([]string, 0, len(quirks))
	for _, quirk := range quirks {
		host.Quirks = append(host.Quirks, prefix+quirk)
	}

	host.Steps = []string{
		prefix + "step_open", "email_domains.hosts.common.step_add", "email_domains.hosts.common.step_check",
	}

	return host
}

/*
HostByKey returns the named host, the generic guide when the name is not
known.
*/
func HostByKey(hosts []Host, key string) Host {
	index := slices.IndexFunc(hosts, func(h Host) bool { return h.Key == key })
	if index < 0 {
		return guide(Host{Key: HostOther, Name: "Your DNS host", HostForm: hostFormRelative})
	}

	return hosts[index]
}

/*
MatchHost returns the key of the host the name servers belong to.
*/
func MatchHost(hosts []Host, nameservers []string) string {
	for _, host := range hosts {
		for _, suffix := range host.NS {
			if slices.ContainsFunc(nameservers, func(ns string) bool {
				return strings.Contains(strings.ToLower(ns), suffix)
			}) {
				return host.Key
			}
		}
	}

	return HostOther
}

/*
HostHint looks up who hosts the domain's DNS, trying the domain and then
each parent up to the registrable one. A failed lookup is no error, only
no guess.
*/
func HostHint(ctx context.Context, r fdns.Resolver, hosts []Host, domain string) string {
	for name := domain; strings.Contains(name, "."); name = name[strings.Index(name, ".")+1:] {
		nameservers, err := r.NS(ctx, name)
		if err == nil && len(nameservers) > 0 {
			return MatchHost(hosts, nameservers)
		}

		if customdomain.IsApex(name) {
			break
		}
	}

	return HostOther
}

/*
HostedName returns a record's name as the host wants it typed. A name
equal to the domain itself becomes the host's mark for the root.
*/
func HostedName(host Host, fqdn, domain string) string {
	if host.HostForm != hostFormRelative {
		return fqdn
	}

	if strings.EqualFold(fqdn, domain) {
		return "@"
	}

	return strings.TrimSuffix(fqdn, "."+domain)
}

/*
DeepLink returns the host's link for the domain, empty when it has none.
*/
func DeepLink(host Host, domain string) string {
	return strings.ReplaceAll(host.DeepLink, "{domain}", domain)
}
