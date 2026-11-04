package emaildomains_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/tests"
)

/*
cloudSESEnv sends through SES with a tenant per workspace and its own
resolvers.
*/
func cloudSESEnv() tests.Option {
	return tests.WithEnv(
		"DIMMI_CHANNELS__EMAIL__SENDING__SES__TENANT_PER_WORKSPACE=true",
		"DIMMI_CHANNELS__EMAIL__SENDING__SES__ACCOUNT_ID=000000000000",
		"DIMMI_CHANNELS__EMAIL__SENDING__RESOLVERS=127.0.0.1:1053,127.0.0.1:2053",
	)
}

func TestSESTenants(t *testing.T) {
	t.Parallel()

	t.Run("ties every domain to the workspace's own tenant", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, sesEnv(), cloudSESEnv())
		fake := stubSES(tc, identity{dkim: "SUCCESS", mailFrom: "SUCCESS"}).
			Stub("POST "+sesBase+"/tenants", httpclient.Response{Body: []byte("{}")}).
			Stub("POST "+sesBase+"/tenants/resources", httpclient.Response{Body: []byte("{}")})
		owner := tc.ActingAsOwner()

		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).
			AssertCreated())

		tenant := "ws-" + tc.Workspace.Workspace.ID.String()
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.HasSuffix(req.URL.Path, "/tenants/resources") &&
				strings.Contains(string(req.Body), tenant) &&
				strings.Contains(string(req.Body), "arn:aws:ses:eu-west-1:000000000000:identity/acme.test")
		})

		publish(zone, created)
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()
		assert.Equal(t, "verified", status(owner, created.Data.ID))
	})

	t.Run("refuses the domain when its tenant cannot be named", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, sesEnv(), cloudSESEnv())
		stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})
		tc.BreakTable("email_sending_profiles")

		tc.ActingAsOwner().PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).
			AssertStatus(503)
	})
}

func TestProviderFailures(t *testing.T) {
	t.Parallel()

	t.Run("keeps checking when ses lost the identity", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, sesEnv())
		stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		publish(zone, created)
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		httpclienttest.New(tc.App.HTTP).PreventStrayRequests().Stub("GET "+sesBase+"/identities/acme.test",
			httpclient.Response{
				Status: fhttp.StatusNotFound, Header: fhttp.Header{"X-Amzn-Errortype": {"NotFoundException"}},
				Body: []byte(`{"message":"gone"}`),
			})

		tc.Drain()

		assert.Equal(t, "verifying", status(owner, created.Data.ID))
		owner.Delete("/api/v1/email-domains/" + created.Data.ID).AssertNoContent()
	})

	t.Run("tries a check again when ses does not answer", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, sesEnv())
		stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		checker := &email.CheckEmailDomain{Domains: tc.App.EmailDomains}
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)

		err := checker.Handle(t.Context(), tc.Workspace.Workspace.ID, mustID(tc, created.Data.ID))

		require.Error(t, err)
		owner.PatchJSON("/api/v1/email-domains/"+created.Data.ID, map[string]any{"mail_from_subdomain": "mail"}).
			AssertStatus(503)
	})

	t.Run("signs nothing for a domain the provider lost", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, resendEnv())
		api := drivertest.ResendDomains()
		fake := httpclienttest.New(tc.App.HTTP).PreventStrayRequests().Stub(api.Create.Pattern, api.Create.Answer)
		for _, route := range api.Tracking {
			fake.Stub(route.Pattern, route.Answer)
		}
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "example.com"}))
		zone.Set(created.Data.Records[0].Name, dns.Records{TXT: []string{created.Data.Records[0].Value}})
		lost := httpclienttest.New(tc.App.HTTP).PreventStrayRequests()
		for _, route := range api.Verify {
			lost.Stub(route.Pattern, api.NotFound)
		}
		lost.Stub(api.Delete.Pattern, api.NotFound)

		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()

		assert.Equal(t, "verifying", status(owner, created.Data.ID))
		owner.Delete("/api/v1/email-domains/" + created.Data.ID).AssertNoContent()
	})
}

func TestDomainStorageFailures(t *testing.T) {
	t.Parallel()

	t.Run("answers with a server error when domains cannot be read", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		tc.BreakTable("email_domains")

		owner.GetJSON("/api/v1/email-domains").AssertStatus(500)
		owner.GetJSON("/api/v1/email-domains/" + created.Data.ID).AssertStatus(500)
		owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "other.test"}).AssertStatus(500)
		owner.Delete("/api/v1/email-domains/" + created.Data.ID).AssertStatus(500)
	})

	t.Run("answers with a server error when a domain cannot be changed", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner, created := verifiedRelayDomain(tc, zone)
		tc.FailWrites("email_domains", "UPDATE")

		owner.PatchJSON("/api/v1/email-domains/"+created.Data.ID, map[string]any{"dns_host": "vercel"}).
			AssertStatus(500)
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/dkim-rotations", nil).AssertStatus(500)
		owner.Delete("/api/v1/email-domains/" + created.Data.ID + "?force=true").AssertStatus(500)
		require.NoError(t, tc.App.Limiter.Clear(t.Context(), "email_domain_check:"+domainID(tc, created.Data.ID)))
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(500)

		checker := &email.CheckEmailDomain{Domains: tc.App.EmailDomains}
		require.Error(t, checker.Handle(t.Context(), tc.Workspace.Workspace.ID, mustID(tc, created.Data.ID)))
	})

	t.Run("answers with a server error when addresses cannot be let go", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner, created := verifiedRelayDomain(tc, zone)
		tc.BreakTable("email_addresses")

		owner.Delete("/api/v1/email-domains/" + created.Data.ID).AssertStatus(500)
		owner.Delete("/api/v1/email-domains/" + created.Data.ID + "?force=true").AssertStatus(500)
	})

	t.Run("leaves a removed domain alone", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		owner.Delete("/api/v1/email-domains/" + created.Data.ID).AssertNoContent()

		checker := &email.CheckEmailDomain{Domains: tc.App.EmailDomains}
		require.NoError(t, checker.Handle(t.Context(), tc.Workspace.Workspace.ID, mustID(tc, created.Data.ID)))
	})
}
