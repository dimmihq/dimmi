package emaildomains_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail/dkim"

	"github.com/dimmihq/dimmi/tests"
)

/*
The test DNS host's API and our template there.
*/
const (
	hostAPI     = "https://api.dc.test/v2"
	templateAPI = hostAPI + "/domainTemplates/providers/trydimmi.com/services/email"
)

/*
connectEnv turns automatic DNS setup on with a fresh signing key; the
signature itself is proven where links are signed.
*/
func connectEnv(t *testing.T) tests.Option {
	t.Helper()

	key, err := dkim.GenerateKey()
	require.NoError(t, err)

	return tests.WithEnv(
		"DIMMI_CHANNELS__EMAIL__SENDING__DOMAIN_CONNECT__ENABLED=true",
		"DIMMI_CHANNELS__EMAIL__SENDING__DOMAIN_CONNECT__PROVIDER_ID=trydimmi.com",
		"DIMMI_CHANNELS__EMAIL__SENDING__DOMAIN_CONNECT__SERVICE_ID=email",
		"DIMMI_CHANNELS__EMAIL__SENDING__DOMAIN_CONNECT__KEY_HOST=_dck1",
		"DIMMI_CHANNELS__EMAIL__SENDING__DOMAIN_CONNECT__PRIVATE_KEY="+key.PrivatePEM,
	)
}

/*
stubHost answers the DNS host's API beside SES, the template answering
with the status: 200 when the host knows it.
*/
func stubHost(tc *tests.TestCase, zone *dns.Fake, status int) {
	tc.T.Helper()

	zone.Set("_domainconnect.acme.test", dns.Records{TXT: []string{"api.dc.test"}})

	stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"}).
		Stub("GET "+hostAPI+"/acme.test/settings", httpclient.Response{Body: []byte(
			`{"providerName":"Cloudflare","urlSyncUX":"https://dash.dc.test","urlAPI":"https://api.dc.test"}`)}).
		Stub("GET "+templateAPI, httpclient.Response{Status: status, Body: []byte("{}")})
}

func TestDNSSetup(t *testing.T) {
	t.Parallel()

	t.Run("hands a signed link to apply the records at a host that knows the template", func(t *testing.T) {
		t.Parallel()
		env := connectEnv(t)
		tc, zone := domainCase(t, sesEnv(), env)
		stubHost(tc, zone, 200)
		owner := tc.ActingAsOwner()
		created := owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).AssertCreated()
		created.AssertJSONPath("data.dns_setup.host_name", "Cloudflare")
		id := created.String("data.id")

		res := owner.PostJSON("/api/v1/email-domains/"+id+"/dns-setups", nil).AssertCreated()

		res.AssertJSONPath("data.object", "email_domain_dns_setup")
		link := res.String("data.apply_url")
		prefix := "https://dash.dc.test/v2/domainTemplates/providers/trydimmi.com/services/email/apply?"
		require.True(t, strings.HasPrefix(link, prefix), link)
		query, tail, _ := strings.Cut(strings.TrimPrefix(link, prefix), "&sig=")
		_, keyHost, _ := strings.Cut(tail, "&key=")
		assert.Equal(t, "_dck1", keyHost)

		values, err := url.ParseQuery(query)
		require.NoError(t, err)
		assert.Equal(t, "acme.test", values.Get("domain"))
		assert.Equal(t, "tok2", values.Get("dkim2"))
		assert.Equal(t, "bounce", values.Get("mailfrom"))
		assert.Equal(t, "eu-west-1", values.Get("region"))
		assert.Equal(t, created.String("data.records.0.value"), "dimmi-verification="+values.Get("token"))
		back := "/d/settings/email/domains/" + id + "?dns_setup=applied"
		assert.True(t, strings.HasSuffix(values.Get("redirect_uri"), back))
	})

	t.Run("hides the button where the host does not know the template", func(t *testing.T) {
		t.Parallel()
		env := connectEnv(t)
		tc, zone := domainCase(t, sesEnv(), env)
		stubHost(tc, zone, 404)
		owner := tc.ActingAsOwner()

		created := owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).AssertCreated()

		created.AssertJSONPath("data.dns_setup", nil)
		owner.PostJSON("/api/v1/email-domains/"+created.String("data.id")+"/dns-setups", nil).
			AssertStatus(422).AssertJSONPath("code", "dns_setup_unavailable")
	})

	t.Run("offers nothing while it is turned off or the records are not ses's", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		zone.Set("_domainconnect.acme.test", dns.Records{TXT: []string{"api.dc.test"}})
		owner := tc.ActingAsOwner()

		created := owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).AssertCreated()

		created.AssertJSONPath("data.dns_setup", nil)
		owner.PostJSON("/api/v1/email-domains/"+created.String("data.id")+"/dns-setups", nil).AssertStatus(422)
		member := addMember(tc, "member")
		tc.ActingAsToken(member, "settings:write").
			PostJSON("/api/v1/email-domains/"+created.String("data.id")+"/dns-setups", nil).AssertForbidden()
	})
}
