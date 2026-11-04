package emaildns_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fdns "github.com/dimmihq/dimmi/framework/dns"

	"github.com/dimmihq/dimmi/app/integrations/email/dns"
)

func TestHosts(t *testing.T) {
	t.Parallel()

	hosts := dns.Hosts()

	t.Run("knows every host the guide covers", func(t *testing.T) {
		t.Parallel()

		keys := make([]string, 0, len(hosts))
		for _, host := range hosts {
			keys = append(keys, host.Key)
			assert.NotEmpty(t, host.Steps, host.Key)
		}

		assert.Equal(t, []string{
			"cloudflare", "route53", "godaddy", "namecheap", "squarespace", "vercel", "netlify", "porkbun", "hover",
			"other",
		}, keys)
	})

	t.Run("matches name servers to their host", func(t *testing.T) {
		t.Parallel()

		cases := map[string][]string{
			"cloudflare": {"ada.ns.cloudflare.com", "bob.ns.cloudflare.com"},
			"route53":    {"ns-1.awsdns-01.org"},
			"godaddy":    {"NS01.DOMAINCONTROL.COM"},
			"vercel":     {"ns1.vercel-dns.com"},
			"other":      {"ns1.example.test"},
		}
		for want, nameservers := range cases {
			assert.Equal(t, want, dns.MatchHost(hosts, nameservers), want)
		}
	})

	t.Run("reads the host from the domain or its registrable parent", func(t *testing.T) {
		t.Parallel()
		zone := fdns.NewFake().
			Set("acme.com", fdns.Records{NS: []string{"ada.ns.cloudflare.com"}}).
			Set("shop.test", fdns.Records{NS: []string{"ns1.vercel-dns.com"}})

		assert.Equal(t, "cloudflare", dns.HostHint(t.Context(), zone, hosts, "mail.acme.com"))
		assert.Equal(t, "vercel", dns.HostHint(t.Context(), zone, hosts, "shop.test"))
		assert.Equal(t, dns.HostOther, dns.HostHint(t.Context(), zone, hosts, "nowhere.com"))
	})

	t.Run("writes a name the way the host wants it", func(t *testing.T) {
		t.Parallel()
		cloudflare := dns.HostByKey(hosts, "cloudflare")
		route53 := dns.HostByKey(hosts, "route53")

		assert.Equal(t, "_dmarc", dns.HostedName(cloudflare, "_dmarc.acme.com", "acme.com"))
		assert.Equal(t, "@", dns.HostedName(cloudflare, "acme.com", "acme.com"))
		assert.Equal(t, "_dmarc.acme.com", dns.HostedName(route53, "_dmarc.acme.com", "acme.com"))
	})

	t.Run("links to the host's records page for the domain", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "https://dcc.godaddy.com/control/dnsmanagement?domainName=acme.com",
			dns.DeepLink(dns.HostByKey(hosts, "godaddy"), "acme.com"))
		assert.Empty(t, dns.DeepLink(dns.HostByKey(hosts, "unknown"), "acme.com"))
		assert.Equal(t, dns.HostOther, dns.HostByKey(hosts, "unknown").Key)
	})
}
