/*
|--------------------------------------------------------------------------
| Email Domain DNS Setup Resource
|--------------------------------------------------------------------------
|
| A DNS host that can add a domain's records itself, as the domain page
| offers it, and the link to approve that at the host once asked for.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/email"

/*
DNSSetupOffer is a DNS host that can add the records itself: its name,
and the link to approve that there once someone asked for it.
*/
type DNSSetupOffer struct {
	Object   string `json:"object"`
	HostName string `json:"host_name"`
	ApplyURL string `json:"apply_url,omitempty"`
}

/*
NewDNSSetupOffer shapes a host's offer.
*/
func NewDNSSetupOffer(setup email.DNSSetup) *DNSSetupOffer {
	return &DNSSetupOffer{Object: "email_domain_dns_setup", HostName: setup.HostName, ApplyURL: setup.ApplyURL}
}
