package sso_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestDomains(t *testing.T) {
	t.Parallel()

	t.Run("a domain is verified by its txt record and shows the record to publish", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		owner := in(tc, tc.Owner)

		added := owner.PostJSON(domainsPath, testkit.JSON{"domain": " Acme.Example. "}).AssertCreated().
			AssertJSONPath("data.domain", "acme.example").
			AssertJSONPath("data.status", "pending").
			AssertJSONPath("data.record_name", "_dimmi-verify.acme.example")
		assert.Contains(t, added.String("data.record_value"), "dimmi-domain-verification=")
		id := added.String("data.id")

		owner.PostJSON(domainsPath+"/"+id+"/checks", testkit.JSON{}).AssertCreated().
			AssertJSONPath("data.status", "pending").AssertJSONPath("data.found_records.#", 0.0)

		zone.Set("_dimmi-verify.acme.example", dns.Records{TXT: []string{"other", added.String("data.record_value")}})
		owner.PostJSON(domainsPath+"/"+id+"/checks", testkit.JSON{}).AssertCreated().
			AssertJSONPath("data.status", "verified").AssertJSONPath("data.domain.verified_via", "dns")

		owner.PostJSON(domainsPath, testkit.JSON{"domain": "acme.example"}).AssertCreated().
			AssertJSONPath("data.id", id)
		owner.GetJSON(domainsPath).AssertOK().AssertJSONPath("data.#", 1.0)
	})

	t.Run("public suffixes and free mail domains cannot be claimed", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)
		owner := in(tc, tc.Owner)

		for _, domain := range []string{"gmail.com", "co.uk", "com", "not a domain", "10.0.0.1"} {
			owner.PostJSON(domainsPath, testkit.JSON{"domain": domain}).AssertStatus(422).
				AssertJSONPath("code", "domain_not_claimable")
		}
	})

	t.Run("the first organization to prove a domain holds it", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		verifiedDomain(tc, zone)

		rival := tc.CreateUser()
		created, err := factories.Workspace(t.Context(), tc.App, rival)
		require.NoError(t, err)
		browser := tc.ActingAs(rival).WithHeader("Dimmi-Workspace",
			models.PublicID(models.PrefixWorkspace, created.Workspace.ID))
		added := browser.PostJSON(domainsPath, testkit.JSON{"domain": "acme.example"}).AssertCreated()
		zone.Set(added.String("data.record_name"), dns.Records{TXT: []string{added.String("data.record_value")}})

		browser.PostJSON(domainsPath+"/"+added.String("data.id")+"/checks", testkit.JSON{}).
			AssertStatus(409).AssertJSONPath("code", "domain_taken")
	})

	t.Run("an unverified domain routes nowhere and a removed one is gone", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)
		owner := in(tc, tc.Owner)
		connection := samlConnection(tc)
		id := owner.PostJSON(domainsPath, testkit.JSON{"domain": "acme.example"}).AssertCreated().String("data.id")

		owner.PatchJSON(domainsPath+"/"+id, testkit.JSON{"sso_connection_id": connection}).
			AssertStatus(422).AssertJSONPath("code", "domain_not_verified")
		owner.Delete(domainsPath + "/" + id).AssertNoContent()
		owner.GetJSON(domainsPath).AssertOK().AssertJSONPath("data.#", 0.0)
	})

	t.Run("the operator command verifies a domain public dns cannot see", func(t *testing.T) {
		t.Parallel()
		tc, _ := secure(t)
		owner := in(tc, tc.Owner)
		owner.PostJSON(domainsPath, testkit.JSON{"domain": "intranet.acme.example"}).AssertCreated()

		code, out := tc.Command("sso:verify-domain", "intranet.acme.example")
		require.Equal(t, 0, code, out)

		owner.GetJSON(domainsPath).AssertOK().
			AssertJSONPath("data.0.status", "verified").AssertJSONPath("data.0.verified_via", "cli")
		owner.GetJSON("/api/v1/audit-logs?filter[event]=organization_domain.verified").AssertOK().
			AssertJSONPath("data.0.actor.type", "system")

		code, _ = tc.Command("sso:verify-domain", "unknown.example")
		assert.NotEqual(t, 0, code)
	})

	t.Run("a proof missing for a week fails the domain, unenforces and tells the owners", func(t *testing.T) {
		t.Parallel()
		tc, zone := secure(t)
		fake := tc.Mail()
		owner := in(tc, tc.Owner)
		id := samlConnection(tc)
		markTested(t, tc, id)
		domain := verifiedDomain(tc, zone)
		owner.PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).AssertOK()
		signedInThrough(t, tc, tc.Owner, id)
		owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"enforced": true}).AssertOK()
		zone.Set("_dimmi-verify.acme.example", dns.Records{TXT: []string{"gone"}})
		recheck := func() {
			t.Helper()
			require.NoError(t, (&jobs.RecheckOrganizationDomains{
				DB: tc.App.DB, Clock: tc.Clock, DNS: tc.App.DNS, Mail: tc.App.Mail, Events: tc.App.Events,
				Links: tc.App.Links,
			}).Handle(t.Context(), &queue.Job[jobs.RecheckOrganizationDomainsArgs]{}))
		}

		recheck()
		owner.GetJSON(domainsPath).AssertOK().AssertJSONPath("data.0.status", "verified")

		tc.Clock.Advance(8 * 24 * time.Hour)
		recheck()
		tc.Drain()

		owner.GetJSON(domainsPath).AssertOK().AssertJSONPath("data.0.status", "failed")
		owner.GetJSON(connectionsPath+"/"+id).AssertOK().AssertJSONPath("data.enforced", false)
		sent, _ := tests.Sent[appmail.DomainVerificationLostMail](tc, fake)
		require.Len(t, sent, 1)
		assert.Equal(t, "acme.example", sent[0].Domain)
	})
}
