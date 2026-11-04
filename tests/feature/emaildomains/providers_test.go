package emaildomains_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
postmarkEnv sends the customer stream through Postmark, set up by hand
for the listed domains.
*/
func postmarkEnv(listed string) tests.Option {
	return tests.WithEnv(
		"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=postmark",
		"DIMMI_MAIL__MAILERS__CUSTOMER__TOKEN=server-token",
		"DIMMI_CHANNELS__EMAIL__SENDING__PROVIDER_VERIFIED_DOMAINS="+listed,
	)
}

/*
resendEnv sends the customer stream through Resend, with its domain API.
*/
func resendEnv() tests.Option {
	return tests.WithEnv(
		"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=resend",
		"DIMMI_MAIL__MAILERS__CUSTOMER__API_KEY=re_key",
		"DIMMI_CHANNELS__EMAIL__SENDING__RESEND__DOMAIN_API_TOKEN="+drivertest.DomainToken,
	)
}

func TestManualProviders(t *testing.T) {
	t.Parallel()

	t.Run("trusts a domain the operator listed once its ownership shows", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, postmarkEnv("acme.test"))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "mail.acme.test"}).
			AssertCreated())
		require.Equal(t, "info", created.Data.Records[1].Status)
		owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).AssertJSONPath("data.provider_trusted", true)
		zone.Set(created.Data.Records[0].Name, dns.Records{TXT: []string{created.Data.Records[0].Value}})

		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()
		assert.Equal(t, "verified", status(owner, created.Data.ID))

		tc.App.EmailDomains.Config.Sending.ProviderVerifiedDomains = nil
		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)
		tc.Clock.Travel(time.Hour)
		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)

		owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).
			AssertJSONPath("data.status", "degraded").AssertJSONPath("data.provider_trusted", false)
	})

	t.Run("waits for the operator before trusting an unlisted domain", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, postmarkEnv("other.test"))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		zone.Set(created.Data.Records[0].Name, dns.Records{TXT: []string{created.Data.Records[0].Value}})

		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()

		owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).
			AssertJSONPath("data.status", "verifying").AssertJSONPath("data.provider_trusted", false)
	})
}

func TestProviderDomainAPI(t *testing.T) {
	t.Parallel()

	t.Run("sets the domain up at the provider and verifies it when the provider does", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, resendEnv())
		api := drivertest.ResendDomains()
		fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests().Stub(api.Create.Pattern, api.Create.Answer)
		for _, route := range api.Tracking {
			fake.Stub(route.Pattern, route.Answer)
		}
		owner := tc.ActingAsOwner()

		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "example.com"}).
			AssertCreated())
		require.Len(t, created.Data.Records, 5)
		assert.Equal(t, "resend._domainkey.example.com", created.Data.Records[1].Name)
		fake.AssertSent(t, api.Credential)

		zone.Set(created.Data.Records[0].Name, dns.Records{TXT: []string{created.Data.Records[0].Value}})
		verify := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		for _, route := range api.Verify {
			verify.Stub(route.Pattern, route.Answer)
		}
		verify.Stub(api.Delete.Pattern, api.Delete.Answer)

		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()
		assert.Equal(t, "verified", status(owner, created.Data.ID))

		owner.Delete("/api/v1/email-domains/" + created.Data.ID + "?force=true").AssertNoContent()
		verify.AssertSent(t, func(req httpclienttest.SentRequest) bool { return req.Method == fhttp.MethodDelete })
	})
}

func TestRemovingDomains(t *testing.T) {
	t.Parallel()

	t.Run("asks before moving addresses off a verified domain", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		tc.ActingAsOwner().PostJSON("/api/v1/email-addresses", map[string]any{"address": "help@acme.test"}).
			AssertCreated()
		owner, created := verifiedRelayDomain(tc, zone)

		owner.Delete("/api/v1/email-domains/"+created.Data.ID).
			AssertStatus(409).AssertJSONPath("code", "domain_in_use").AssertJSONPath("addresses.0", "help@acme.test")

		owner.Delete("/api/v1/email-domains/" + created.Data.ID + "?force=true").AssertNoContent()

		owner.GetJSON("/api/v1/email-domains/" + created.Data.ID).AssertNotFound()
		assert.Equal(t, 0, linkedAddressCount(tc))
	})

	t.Run("removes a domain nothing sends from at once", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))

		owner.Delete("/api/v1/email-domains/" + created.Data.ID).AssertNoContent()

		owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).AssertCreated()
	})
}

/*
linkedAddressCount counts the workspace's addresses tied to any domain.
*/
func linkedAddressCount(tc *tests.TestCase) int {
	tc.T.Helper()

	ws := tc.Workspace.Workspace.ID
	rows := query(tc, ws, func(ctx context.Context, q *models.Queries) ([]models.ListEmailAddressesRow, error) {
		return q.ListEmailAddresses(ctx, ws)
	})

	count := 0
	for _, row := range rows {
		if row.EmailAddress.EmailDomainID != nil {
			count++
		}
	}

	return count
}
