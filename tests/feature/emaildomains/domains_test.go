package emaildomains_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestAddingDomains(t *testing.T) {
	t.Parallel()

	t.Run("answers with every record to publish and the guide of the dns host", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, sesEnv())
		zone.Set("acme.test", dns.Records{NS: []string{"ada.ns.cloudflare.com"}})
		fake := stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})

		res := tc.ActingAsOwner().PostJSON("/api/v1/email-domains", map[string]any{"domain": "Acme.TEST."}).
			AssertCreated()

		res.AssertJSONPath("data.object", "email_domain")
		res.AssertJSONPath("data.domain", "acme.test")
		res.AssertJSONPath("data.status", "pending")
		res.AssertJSONPath("data.driver", "ses")
		res.AssertJSONPath("data.provider_hint", "cloudflare")
		res.AssertJSONPath("data.host_guide.detected", true)
		res.AssertJSONPath("data.host_guide.host_form", "relative")
		res.AssertJSONPath("data.mail_from_domain", "bounce.acme.test")
		res.AssertJSONPath("data.records.#", 7.0)
		res.AssertJSONPath("data.records.0.purpose", "ownership")
		res.AssertJSONPath("data.records.0.host", "_dimmi-verification")
		res.AssertJSONPath("data.records.0.name", "_dimmi-verification.acme.test")
		res.AssertJSONPath("data.records.1.value", "tok1.dkim.amazonses.com")
		res.AssertJSONPath("data.records.4.value", "feedback-smtp.eu-west-1.amazonses.com")
		assert.True(t, strings.HasSuffix(res.String("data.links.zone_file"), "?format=bind"))
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(string(req.Body), "bounce.acme.test")
		})
	})

	t.Run("shows an international domain as written and stores it for dns", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))

		res := tc.ActingAsOwner().PostJSON("/api/v1/email-domains", map[string]any{"domain": "café.example"}).
			AssertCreated()

		res.AssertJSONPath("data.domain", "café.example")
		res.AssertJSONPath("data.ascii_domain", "xn--caf-dma.example")
		res.AssertJSONPath("data.records.0.name", "_dimmi-verification.xn--caf-dma.example")
	})

	t.Run("refuses names nobody can own as a sending domain", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()

		for _, name := range []string{"com", "192.0.2.1", "mail." + inboundDomain, "user@acme.test", "co.uk"} {
			owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": name}).
				AssertStatus(422).AssertJSONPath("code", "invalid_domain")
		}

		owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "gmail.com"}).
			AssertStatus(422).AssertJSONPath("code", "invalid_domain")
		owner.PostJSON("/api/v1/email-domains", map[string]any{}).AssertValidationError("domain")
		owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).AssertCreated()
		owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).
			AssertStatus(409).AssertJSONPath("code", "domain_exists")
	})

	t.Run("refuses domains while customer mail goes nowhere", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t)

		tc.ActingAsOwner().PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).
			AssertStatus(422).AssertJSONPath("code", "sending_not_configured")
	})

	t.Run("answers when ses cannot be reached", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, sesEnv())
		httpclienttest.New(tc.App.HTTP).PreventStrayRequests()

		tc.ActingAsOwner().PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).
			AssertStatus(503).AssertJSONPath("code", "email_provider_unavailable")
	})

	t.Run("lists the domains and shows one as a zone file", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))

		owner.GetJSON("/api/v1/email-domains").AssertOK().
			AssertJSONPath("data.#", 1.0).AssertJSONPath("data.0.id", created.Data.ID)

		res := owner.GetJSON("/api/v1/email-domains/" + created.Data.ID + "?format=bind").AssertOK()
		assert.Contains(t, res.Header.Get("Content-Type"), "text/plain")
		assert.Contains(t, string(res.Body), "_dimmi-verification.acme.test. 3600 IN TXT")
		assert.Contains(t, string(res.Body), `"v=DKIM1; k=rsa; p=`)

		owner.GetJSON("/api/v1/email-domains/edm_00000000000000000000000000").AssertNotFound()
		owner.GetJSON("/api/v1/email-domains/nonsense").AssertNotFound()
	})
}

func TestDomainPermissions(t *testing.T) {
	t.Parallel()

	t.Run("lets members read domains and only admins change them", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		created := decode(tc.ActingAsOwner().PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		member := addMember(tc, enums.RoleMember)
		admin := addMember(tc, enums.RoleAdmin)

		asMember := tc.ActingAsToken(member, "settings:read", "settings:write")
		asMember.GetJSON("/api/v1/email-domains").AssertOK()
		asMember.PostJSON("/api/v1/email-domains", map[string]any{"domain": "other.test"}).AssertForbidden()
		asMember.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertForbidden()

		asAdmin := tc.ActingAsToken(admin, "settings:read", "settings:write")
		asAdmin.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		asAdmin.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/dkim-rotations", nil).AssertForbidden()

		reader := tc.ActingAsToken(admin, "settings:read")
		reader.PostJSON("/api/v1/email-domains", map[string]any{"domain": "other.test"}).AssertForbidden()
	})

	t.Run("hides the domains of other workspaces", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		created := decode(tc.ActingAsOwner().PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		stranger := tc.CreateUser()
		_, err := factories.Workspace(t.Context(), tc.App, stranger)
		require.NoError(t, err)

		other := tc.ActingAs(stranger)
		other.GetJSON("/api/v1/email-domains/" + created.Data.ID).AssertNotFound()
		other.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertNotFound()
		other.Delete("/api/v1/email-domains/" + created.Data.ID).AssertNotFound()
		other.GetJSON("/api/v1/email-domains").AssertOK().AssertJSONPath("data.#", 0.0)
	})
}

func TestCheckingDomains(t *testing.T) {
	t.Parallel()

	t.Run("verifies a domain once its records show and ses signs for it", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, sesEnv())
		stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})
		owner := tc.ActingAsOwner()
		owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "support@acme.test"}).AssertCreated()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))

		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).
			AssertStatus(202).AssertJSONPath("data.status", "verifying")
		tc.Drain()
		shown := owner.GetJSON("/api/v1/email-domains/" + created.Data.ID).AssertOK()
		shown.AssertJSONPath("data.status", "verifying")
		shown.AssertJSONPath("data.records.0.status", "missing")

		publish(zone, created)
		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)
		owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).AssertJSONPath("data.status", "verifying")

		stubSES(tc, identity{dkim: "SUCCESS", mailFrom: "SUCCESS"})
		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)

		verified := owner.GetJSON("/api/v1/email-domains/" + created.Data.ID).AssertOK()
		verified.AssertJSONPath("data.status", "verified")
		verified.AssertJSONPath("data.records.0.status", "verified")
		assert.NotEmpty(t, verified.String("data.verified_at"))
		assert.Equal(t, 1, linkedAddresses(tc, created.Data.ID))
	})

	t.Run("lets a check be asked for once every ten seconds", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))

		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		res := owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(429)

		res.AssertJSONPath("code", "domain_check_throttled")
		assert.NotEmpty(t, res.Header.Get("Retry-After"))
	})

	t.Run("explains records typed with the domain twice", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		zone.Set("_dimmi-verification.acme.test.acme.test", dns.Records{TXT: []string{created.Data.Records[0].Value}})

		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()

		shown := decode(owner.GetJSON("/api/v1/email-domains/" + created.Data.ID))
		require.NotEmpty(t, shown.Data.Warnings)
		assert.Equal(t, "doubled_domain", shown.Data.Warnings[0].Code)
		assert.Contains(t, shown.Data.Warnings[0].Message, "_dimmi-verification.acme.test.acme.test")
	})

	t.Run("queues the checks that are due", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()
		publish(zone, created)

		tc.Clock.Travel(2 * time.Minute)
		require.NoError(t, tc.App.Queue.Push(t.Context(), jobs.ScheduleEmailDomainChecksArgs{}))
		tc.Drain()

		owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).AssertJSONPath("data.status", "verified")
	})
}

/*
addMember adds a new user to the test workspace with the role.
*/
func addMember(tc *tests.TestCase, role enums.Role) models.WorkspaceMember {
	tc.T.Helper()

	member, err := factories.Member(tc.T.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: tc.CreateUser(), Role: role,
	})
	require.NoError(tc.T, err)

	return member
}

/*
linkedAddresses counts the live addresses sending from the domain.
*/
func linkedAddresses(tc *tests.TestCase, public string) int {
	tc.T.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixEmailDomain), public)
	require.NoError(tc.T, err)

	ws := tc.Workspace.Workspace.ID
	rows := query(tc, ws, func(ctx context.Context, q *models.Queries) ([]string, error) {
		return q.ListEmailDomainAddresses(ctx, models.ListEmailDomainAddressesParams{
			WorkspaceID: ws, EmailDomainID: &id,
		})
	})

	return len(rows)
}
