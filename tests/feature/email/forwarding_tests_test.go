package email_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestForwardingTests(t *testing.T) {
	t.Parallel()

	t.Run("stores a test that settles when its mail comes back", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		owner := tc.ActingAsOwner()
		id, forwarding, public := testAddress(tc)

		res := owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated()
		res.AssertJSONPath("data.object", "email_address_test")
		res.AssertJSONPath("data.status", "waiting")
		res.AssertJSONPath("data.email_address", id)
		res.AssertJSONPath("data.sent_to", public)
		res.AssertJSONMissing("data.token")
		test := res.String("data.id")
		assert.True(t, strings.HasPrefix(test, "eat_"))

		mails := received(t, public)
		require.Len(t, mails, 1)
		mail := parsed(t, mails[0])
		assert.True(t, strings.HasPrefix(mail.Subject, "dimmi forwarding test "))
		assert.Contains(t, mail.Text, "dimmi test "+mail.Header.Get(inbound.TestHeader))

		tc.Clock.Advance(12 * time.Second)
		deliver(tc, mails[0], to(forwarding))

		shown := owner.GetJSON("/api/v1/email-addresses/" + id + "/tests/" + test).AssertOK()
		shown.AssertJSONPath("data.status", "received")
		shown.AssertJSONPath("data.latency_ms", float64(12000))
		shown.AssertJSONPath("data.findings.#", float64(0))
		assert.NotEmpty(t, shown.String("data.completed_at"))
		assert.NotEmpty(t, shown.String("data.message_id"))
		assert.NotEmpty(t, owner.GetJSON("/api/v1/email-addresses/"+id).AssertOK().String("data.inbound_verified_at"))
		assert.Empty(t, conversations(tc))
		owner.GetJSON("/api/v1/email-addresses/"+id+"/inbound-emails").AssertOK().
			AssertJSONPath("data.0.ignore_reason", "forwarding_test")
	})

	t.Run("finds the token on the last line when the forward drops the header", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		owner := tc.ActingAsOwner()
		id, forwarding, public := testAddress(tc)

		test := owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated().String("data.id")
		raw := string(received(t, public)[0])
		stripped := strings.Replace(raw, inbound.TestHeader+":", "X-Dropped:", 1)

		tc.Clock.Advance(6 * time.Minute)
		deliver(tc, []byte(stripped), to(forwarding))

		shown := owner.GetJSON("/api/v1/email-addresses/" + id + "/tests/" + test).AssertOK()
		shown.AssertJSONPath("data.status", "received")
		shown.AssertJSONPath("data.findings.0.code", "slow_forward")
		shown.AssertJSONPath("data.findings.0.severity", "info")
		assert.NotEmpty(t, shown.String("data.findings.0.message"))
	})

	t.Run("drops a copy of a finished test as stale", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		owner := tc.ActingAsOwner()
		id, forwarding, public := testAddress(tc)

		owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated()
		raw := received(t, public)[0]
		deliver(tc, raw, to(forwarding))
		deliver(tc, raw, to(forwarding, "id", "replayed"))

		owner.GetJSON("/api/v1/email-addresses/"+id+"/inbound-emails").AssertOK().
			AssertJSONPath("data.0.ignore_reason", "test_stale")
		assert.Empty(t, conversations(tc))
	})

	t.Run("settles only its own test when two run on one address", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		owner := tc.ActingAsOwner()
		id, forwarding, public := testAddress(tc)
		addressID, err := tc.App.IDs.Decode("ema", id)
		require.NoError(t, err)

		first := owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated().String("data.id")
		owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertStatus(429).
			AssertJSONPath("code", "email_test_throttled")
		require.NoError(t, tc.App.Limiter.Clear(t.Context(), "email_test:"+addressID.String()))
		created := owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated()
		second := created.String("data.id")

		mails := received(t, public)
		require.Len(t, mails, 2)
		idx := slices.IndexFunc(mails, func(raw []byte) bool {
			return parsed(t, raw).MessageID == created.String("data.message_id")
		})
		require.GreaterOrEqual(t, idx, 0)
		deliver(tc, mails[idx], to(forwarding))

		owner.GetJSON("/api/v1/email-addresses/"+id+"/tests/"+second).AssertOK().
			AssertJSONPath("data.status", "received")
		owner.GetJSON("/api/v1/email-addresses/"+id+"/tests/"+first).AssertOK().
			AssertJSONPath("data.status", "waiting")

		tc.Clock.Advance(10 * time.Minute)
		runTimeouts(tc)

		owner.GetJSON("/api/v1/email-addresses/"+id+"/tests/"+first).AssertOK().
			AssertJSONPath("data.status", "timed_out")
		owner.GetJSON("/api/v1/email-addresses/"+id+"/tests/"+second).AssertOK().
			AssertJSONPath("data.status", "received")
	})

	t.Run("times out with hints for the mail service of the address", func(t *testing.T) {
		t.Parallel()
		tc, zone := emailCase(t, mailpitEnv(t))
		owner := tc.ActingAsOwner()
		public := unique("support")
		zone.Set("customer.test", dns.Records{MX: []dns.MX{{Host: "aspmx.l.google.com.", Pref: 1}}})
		id := owner.PostJSON("/api/v1/email-addresses", testkit.JSON{"address": public}).AssertCreated().
			String("data.id")

		test := owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated().String("data.id")
		tc.Clock.Advance(10 * time.Minute)
		runTimeouts(tc)

		shown := owner.GetJSON("/api/v1/email-addresses/" + id + "/tests/" + test).AssertOK()
		shown.AssertJSONPath("data.status", "timed_out")
		shown.AssertJSONPath("data.findings.0.code", "google_rule_delay")
		shown.AssertJSONPath("data.findings.1.code", "check_spam")
	})

	t.Run("fails on a bounce without suppressing the address", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		owner := tc.ActingAsOwner()
		id, forwarding, public := testAddress(tc)

		created := owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated()
		bounce := strings.ReplaceAll(string(fixture(t, "bounce_dsn.eml", map[string]string{
			"TO": forwarding, "RECIPIENT": public, "FORWARDING": forwarding,
			"ORIGINAL": created.String("data.message_id"),
		})), "5.1.1", "5.7.520")
		deliver(tc, []byte(bounce), to(forwarding))

		shown := owner.GetJSON("/api/v1/email-addresses/" + id + "/tests/" + created.String("data.id")).AssertOK()
		shown.AssertJSONPath("data.status", "failed")
		shown.AssertJSONPath("data.findings.0.code", "m365_external_forward_blocked")
		shown.AssertJSONPath("data.findings.0.severity", "error")
		tc.Table("email_suppressions").AssertCount(0, testkit.Row{})
		tc.Table("outbound_emails").AssertHas(map[string]any{"kind": "test", "status": "bounced"})
	})

	t.Run("stores the test for sending by hand when mail only goes to the log", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		owner := tc.ActingAsOwner()
		address := defaultAddress(tc)

		res := owner.PostJSON("/api/v1/email-addresses/"+publicID(address)+"/tests", nil).AssertStatus(422)
		res.AssertJSONPath("code", "no_sender_available")
		res.AssertJSONPath("send_to", address.Address)
		assert.True(t, strings.HasPrefix(res.String("subject"), "dimmi forwarding test "))
		shown := "/api/v1/email-addresses/" + publicID(address) + "/tests/" + res.String("test")
		owner.GetJSON(shown).AssertOK().AssertJSONPath("data.status", "sending")
		tc.Table("outbound_emails").AssertCount(0, testkit.Row{"kind": "test"})

		byHand := email{
			from: unique("owner"), to: address.Address, subject: res.String("subject"), id: unique("hand"),
			body: "Testing the forward.",
		}
		deliver(tc, byHand.raw(), to(address.ForwardingAddress))

		owner.GetJSON(shown).AssertOK().AssertJSONPath("data.status", "received").
			AssertJSONPath("data.findings.#", float64(0))
		assert.Empty(t, conversations(tc))
	})

	t.Run("lets only managers read tests and hides other workspaces", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		id, _, _ := testAddress(tc)
		test := tc.ActingAsOwner().PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated().
			String("data.id")

		user := tc.CreateUser()
		_, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)

		tc.ActingAs(user).GetJSON("/api/v1/email-addresses/" + id + "/tests/" + test).AssertForbidden()
		tc.ActingAsOwner().GetJSON("/api/v1/email-addresses/" + id + "/tests/eat_bogus").AssertNotFound()
		tc.ActingAsOwner().GetJSON("/api/v1/email-addresses/" + id + "/tests/not-an-id").AssertNotFound()
	})

	t.Run("prunes tests older than thirty days", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		id, _, _ := testAddress(tc)
		tc.ActingAsOwner().PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated()

		tc.Clock.Advance(31 * 24 * time.Hour)
		_, err := tc.Operator().Exec(t.Context(),
			"INSERT INTO river_job (kind, args, queue, state, max_attempts, scheduled_at) "+
				"VALUES ('prune_email_address_tests', '{}', 'default', 'available', 1, now())")
		require.NoError(t, err)
		tc.Drain()

		tc.Table("email_address_tests").AssertCount(0, testkit.Row{})
	})
}

/*
testAddress connects a fresh public address and returns its public id,
its forwarding address and the public address.
*/
func testAddress(tc *tests.TestCase) (string, string, string) {
	tc.T.Helper()

	public := unique("support")
	created := tc.ActingAsOwner().PostJSON("/api/v1/email-addresses", testkit.JSON{"address": public}).
		AssertCreated()

	return created.String("data.id"), created.String("data.forwarding_address"), public
}

/*
runTimeouts makes every waiting test timeout due now and runs it.
*/
func runTimeouts(tc *tests.TestCase) {
	tc.T.Helper()

	_, err := tc.Operator().Exec(context.WithoutCancel(tc.T.Context()),
		"UPDATE river_job SET state = 'available', scheduled_at = now(), finalized_at = NULL "+
			"WHERE kind = 'check_email_address_test'")
	require.NoError(tc.T, err)
	tc.Drain()
}
