package emaildomains_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/dns"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	maildomains "github.com/dimmihq/dimmi/framework/mail/domains"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
verifiedRelayDomain adds acme.test on a relay, publishes its records and
checks it until it is verified.
*/
func verifiedRelayDomain(tc *tests.TestCase, zone *dns.Fake) (*testkit.Client, domainBody) {
	tc.T.Helper()

	owner := tc.ActingAsOwner()
	created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).AssertCreated())
	publish(zone, created)
	owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
	tc.Drain()
	owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).AssertJSONPath("data.status", "verified")

	return owner, created
}

/*
status returns the domain's status as the API shows it.
*/
func status(owner *testkit.Client, id string) string {
	return owner.GetJSON("/api/v1/email-domains/" + id).AssertOK().String("data.status")
}

func TestDomainStates(t *testing.T) {
	t.Parallel()

	t.Run("degrades a domain only after two checks an hour apart miss its records", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner, created := verifiedRelayDomain(tc, zone)
		ws := tc.Workspace.Workspace.ID
		zone.Set("_dimmi-verification.acme.test", dns.Records{})

		check(tc, ws, created.Data.ID)
		assert.Equal(t, "verified", status(owner, created.Data.ID))

		tc.Clock.Travel(time.Hour)
		check(tc, ws, created.Data.ID)
		degraded := owner.GetJSON("/api/v1/email-domains/" + created.Data.ID)
		degraded.AssertJSONPath("data.status", "degraded")
		degraded.AssertJSONPath("data.failure_reason", "records_removed")
		assert.Contains(t, degraded.String("data.failure_message"), "dimmi address")

		publish(zone, created)
		check(tc, ws, created.Data.ID)
		owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).
			AssertJSONPath("data.status", "verified").AssertJSONPath("data.failure_reason", nil)
	})

	t.Run("fails a domain degraded for a week", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner, created := verifiedRelayDomain(tc, zone)
		ws := tc.Workspace.Workspace.ID
		zone.Set("_dimmi-verification.acme.test", dns.Records{})
		check(tc, ws, created.Data.ID)
		tc.Clock.Travel(time.Hour)
		check(tc, ws, created.Data.ID)

		tc.Clock.Travel(8 * 24 * time.Hour)
		check(tc, ws, created.Data.ID)

		assert.Equal(t, "failed", status(owner, created.Data.ID))
		owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).AssertJSONPath("data.next_check_at", nil)
	})

	t.Run("fails a domain not verified within three days and lets it start again", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()

		tc.Clock.Travel(73 * time.Hour)
		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)
		failed := owner.GetJSON("/api/v1/email-domains/" + created.Data.ID)
		failed.AssertJSONPath("data.status", "failed").AssertJSONPath("data.failure_reason", "not_verified")

		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)
		assert.Equal(t, "failed", status(owner, created.Data.ID))

		require.NoError(t, tc.App.Limiter.Clear(t.Context(), "email_domain_check:"+domainID(tc, created.Data.ID)))
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).
			AssertStatus(202).AssertJSONPath("data.status", "verifying").AssertJSONPath("data.failure_reason", nil)
	})

	t.Run("checks a verifying domain less often as time passes", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, relayEnv(t))
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		ws := tc.Workspace.Workspace.ID

		waits := []time.Duration{}
		for _, travel := range []time.Duration{0, 20 * time.Minute, 2 * time.Hour, 30 * time.Hour} {
			tc.Clock.Travel(travel)
			check(tc, ws, created.Data.ID)
			next, err := time.Parse(time.RFC3339Nano, owner.GetJSON("/api/v1/email-domains/"+created.Data.ID).
				String("data.next_check_at"))
			require.NoError(t, err)
			waits = append(waits, next.Sub(tc.Clock.Now()))
		}

		assert.Equal(t, []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}, waits)
	})
}

func TestDomainOwnership(t *testing.T) {
	t.Parallel()

	t.Run("keeps the domain with the first workspace while its proof is published", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		_, first := verifiedRelayDomain(tc, zone)
		second, theirs := otherWorkspaceDomain(tc)
		zone.Set("_dimmi-verification.acme.test", dns.Records{
			TXT: []string{first.Data.Records[0].Value, theirs.Data.Records[0].Value},
		})

		second.PostJSON("/api/v1/email-domains/"+theirs.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()

		shown := decode(second.GetJSON("/api/v1/email-domains/" + theirs.Data.ID))
		assert.Equal(t, "verifying", shown.Data.Status)
		assert.Contains(t, warningCodes(shown), "domain_claimed")
	})

	t.Run("moves the domain to a workspace that proves it once the holder's proof is gone", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner, first := verifiedRelayDomain(tc, zone)
		second, theirs := otherWorkspaceDomain(tc)
		publish(zone, theirs)

		second.PostJSON("/api/v1/email-domains/"+theirs.Data.ID+"/checks", nil).AssertStatus(202)
		tc.Drain()

		assert.Equal(t, "verified", status(second, theirs.Data.ID))
		lost := owner.GetJSON("/api/v1/email-domains/" + first.Data.ID)
		lost.AssertJSONPath("data.status", "failed").AssertJSONPath("data.failure_reason", "ownership_transferred")
	})
}

/*
domainID returns the internal id of a domain's public id.
*/
func domainID(tc *tests.TestCase, public string) string {
	tc.T.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixEmailDomain), public)
	require.NoError(tc.T, err)

	return id.String()
}

/*
otherWorkspaceDomain adds acme.test in a workspace of another owner.
*/
func otherWorkspaceDomain(tc *tests.TestCase) (*testkit.Client, domainBody) {
	tc.T.Helper()

	stranger := tc.CreateUser()
	_, err := factories.Workspace(tc.T.Context(), tc.App, stranger)
	require.NoError(tc.T, err)

	client := tc.ActingAs(stranger)
	created := decode(client.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).AssertCreated())

	return client, created
}

/*
warningCodes returns the codes of a domain's warnings.
*/
func warningCodes(body domainBody) []string {
	codes := []string{}
	for _, w := range body.Data.Warnings {
		codes = append(codes, w.Code)
	}

	return codes
}

func TestRelayKeys(t *testing.T) {
	t.Parallel()

	t.Run("signs with the new key once its record shows", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner, created := verifiedRelayDomain(tc, zone)
		oldKey := created.Data.Records[1]

		rotated := decode(owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/dkim-rotations", nil).
			AssertCreated())
		next := rotated.Data.Records[len(rotated.Data.Records)-1]
		require.Equal(t, "dkim_next", next.Purpose)
		assert.NotEqual(t, oldKey.Name, next.Name)

		again := decode(owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/dkim-rotations", nil))
		assert.Equal(t, next.Name, again.Data.Records[len(again.Data.Records)-1].Name)

		zone.Set(next.Name, dns.Records{TXT: []string{next.Value}})
		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)

		done := decode(owner.GetJSON("/api/v1/email-domains/" + created.Data.ID))
		assert.Equal(t, "verified", done.Data.Status)
		var keys []record
		for _, r := range done.Data.Records {
			if strings.HasPrefix(r.Purpose, "dkim") {
				keys = append(keys, r)
			}
		}
		require.Len(t, keys, 1)
		assert.Equal(t, next.Name, keys[0].Name)
		assert.Equal(t, "dkim", keys[0].Purpose)
	})

	t.Run("leaves keys to ses", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, sesEnv())
		stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))

		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/dkim-rotations", nil).
			AssertStatus(422).AssertJSONPath("code", "dkim_rotation_unsupported")
	})
}

func TestSESDomains(t *testing.T) {
	t.Parallel()

	t.Run("asks ses to look again when it gave up but the records are there", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, sesEnv())
		stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))
		publish(zone, created)
		owner.PostJSON("/api/v1/email-domains/"+created.Data.ID+"/checks", nil).AssertStatus(202)
		fake := stubSES(tc, identity{dkim: "FAILED", mailFrom: "SUCCESS"})

		tc.Drain()

		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.Method == fhttp.MethodPut && strings.HasSuffix(req.URL.Path, "/dkim/signing")
		})
		assert.Equal(t, "verifying", status(owner, created.Data.ID))
	})

	t.Run("moves the return path to another subdomain", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, sesEnv())
		fake := stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})
		owner := tc.ActingAsOwner()
		created := decode(owner.PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}))

		res := owner.PatchJSON("/api/v1/email-domains/"+created.Data.ID, map[string]any{
			"mail_from_subdomain": "dimmi-bounce", "dns_host": "godaddy",
		}).AssertOK()

		res.AssertJSONPath("data.mail_from_domain", "dimmi-bounce.acme.test")
		res.AssertJSONPath("data.provider_hint", "godaddy")
		res.AssertJSONPath("data.host_guide.detected", false)
		res.AssertJSONPath("data.records.#(purpose==\"return_path\").name", "dimmi-bounce.acme.test")
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return strings.Contains(string(req.Body), "dimmi-bounce.acme.test")
		})
		owner.PatchJSON("/api/v1/email-domains/"+created.Data.ID, map[string]any{"mail_from_subdomain": "a.b"}).
			AssertStatus(422).AssertJSONPath("code", "invalid_domain")
	})

	t.Run("sets a domain up again when the install switches provider", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		owner, created := verifiedRelayDomain(tc, zone)
		stubSES(tc, identity{dkim: "PENDING", mailFrom: "PENDING"})
		tc.App.EmailDomains.Driver, tc.App.EmailDomains.Region = "ses", "eu-west-1"
		tc.App.EmailDomains.Provider = &email.SESDomains{
			DB: tc.App.DB, Clock: tc.App.Clock, Region: "eu-west-1",
			Connect: func(ctx context.Context) (email.SESIdentities, error) {
				return maildomains.Identities(ctx, tc.App.HTTP, config.Section{
					"region": "eu-west-1", "access_key_id": "AKID", "secret_access_key": "secret",
				})
			},
		}

		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)

		switched := owner.GetJSON("/api/v1/email-domains/" + created.Data.ID)
		switched.AssertJSONPath("data.status", "verifying").AssertJSONPath("data.driver", "ses")
		switched.AssertJSONPath("data.records.1.type", "CNAME")
	})
}
