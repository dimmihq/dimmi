package email_test

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/cloudflare"
	"github.com/dimmihq/dimmi/framework/webhooks"

	emailactions "github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/integrations/email/cloudflareworker"
	"github.com/dimmihq/dimmi/app/models"
)

/*
checksOf returns the check with the id, or an empty one.
*/
func checksOf(t *testing.T, checks []emailactions.Check, id string) emailactions.Check {
	t.Helper()

	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}

	return emailactions.Check{}
}

func TestInboundDoctor(t *testing.T) {
	t.Parallel()

	t.Run("passes an mx that matches the source and names one that does not", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "postmark", map[string]string{"USERNAME": "hook", "PASSWORD": "secret"})
		doctor := tc.App.Actions.InboundDoctor

		zone := tc.App.DNS.Fake()
		zone.Set(inboundDomain, dns.Records{MX: []dns.MX{{Host: "inbound.postmarkapp.com.", Pref: 10}}})
		assert.Equal(t, emailactions.CheckPass, checksOf(t, doctor.Handle(t.Context()), "email.inbound_mx").Status)

		zone.Set(inboundDomain, dns.Records{MX: []dns.MX{{Host: "mail.other.example.", Pref: 10}}})
		check := checksOf(t, doctor.Handle(t.Context()), "email.inbound_mx")
		assert.Equal(t, emailactions.CheckFail, check.Status)
		assert.Contains(t, check.Message, "mail.other.example")

		zone.Set(inboundDomain, dns.Records{})
		assert.Equal(t, emailactions.CheckFail, checksOf(t, doctor.Handle(t.Context()), "email.inbound_mx").Status)
	})

	t.Run("knows the mx of every source it can match", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		for source, host := range map[string]string{
			"ses": "inbound-smtp.eu-west-1.amazonaws.com.", "mailgun": "mxa.mailgun.org.",
			"sendgrid": "mx.sendgrid.net.", "cloudflare": "route1.mx.cloudflare.net.",
		} {
			doctor := *tc.App.Actions.InboundDoctor
			doctor.Config.Inbound.Drivers = []string{source}
			tc.App.DNS.Fake().Set(inboundDomain, dns.Records{MX: []dns.MX{{Host: host, Pref: 10}}})

			check := checksOf(t, doctor.Handle(t.Context()), "email.inbound_mx")
			assert.Equal(t, emailactions.CheckPass, check.Status, source)
		}
	})

	t.Run("cannot match resend, so it only sees an mx exists", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "resend", map[string]string{
			"WEBHOOK_SECRET": webhooks.NewSecret().Encode(), "API_KEY": "k",
		})
		tc.App.DNS.Fake().Set(inboundDomain, dns.Records{MX: []dns.MX{{Host: "inbound.resend.example.", Pref: 10}}})

		check := checksOf(t, tc.App.Actions.InboundDoctor.Handle(t.Context()), "email.inbound_mx")

		assert.Equal(t, emailactions.CheckWarn, check.Status)
	})

	t.Run("says when the install has no inbound domain", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		doctor := *tc.App.Actions.InboundDoctor
		doctor.Config.InboundDomain = ""

		assert.Equal(t, emailactions.CheckPass, checksOf(t, doctor.Handle(t.Context()), "email.inbound_mx").Status)
	})

	t.Run("warns about an address providers cannot reach", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		doctor := *tc.App.Actions.InboundDoctor

		for address, want := range map[string]emailactions.CheckStatus{
			"http://example.com":  emailactions.CheckWarn,
			"https://localhost":   emailactions.CheckWarn,
			"https://10.0.0.1":    emailactions.CheckWarn,
			"https://app.example": emailactions.CheckPass,
		} {
			doctor.AppURL = address
			assert.Equal(t, want, checksOf(t, doctor.Handle(t.Context()), "email.inbound_url").Status, address)
		}
	})

	t.Run("names a setting a delivery said was off", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "cloudflare", map[string]string{"SECRET": cloudflareSecret})
		raw, address := inboundRaw(tc)
		timestamp := strconv.FormatInt(tc.Clock.Now().Unix(), 10)

		tc.Client().WithHeader("Content-Type", "message/rfc822").
			WithHeader(cloudflare.HeaderTimestamp, timestamp).
			WithHeader(cloudflare.HeaderSignature, cloudflare.Sign(cloudflareSecret, timestamp, raw)).
			WithHeader(cloudflare.HeaderEnvelopeTo, address).
			WithHeader(cloudflare.HeaderFallback, "off").
			Send("POST", "/webhooks/cloudflare/inbound", raw).AssertOK()

		check := checksOf(t, tc.App.Actions.InboundDoctor.Handle(t.Context()), "email.misconfigured.cloudflare")
		assert.Equal(t, emailactions.CheckFail, check.Status)
		assert.Contains(t, check.Message, cloudflare.FallbackAdvice)
	})

	t.Run("warns when addresses exist but no mail arrived, and passes once it has", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		doctor := tc.App.Actions.InboundDoctor

		assert.Equal(t, emailactions.CheckWarn, checksOf(t, doctor.Handle(t.Context()), "email.inbound_recent").Status)

		address := defaultAddress(tc)
		raw := "From: bob@customer.test\r\nTo: " + address.ForwardingAddress + "\r\nSubject: Hello there\r\n\r\nHi\r\n"
		deliver(tc, []byte(raw), "")

		assert.Equal(t, emailactions.CheckPass, checksOf(t, doctor.Handle(t.Context()), "email.inbound_recent").Status)
	})

	t.Run("warns when this server's clock drifted from the database's", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		doctor := tc.App.Actions.InboundDoctor

		now, err := models.New(tc.App.DB).DatabaseNow(t.Context())
		require.NoError(t, err)

		tc.Clock.Freeze(now.Add(10 * time.Minute))
		assert.Equal(t, emailactions.CheckWarn, checksOf(t, doctor.Handle(t.Context()), "email.clock").Status)

		tc.Clock.Freeze(now)
		assert.Equal(t, emailactions.CheckPass, checksOf(t, doctor.Handle(t.Context()), "email.clock").Status)
	})

	t.Run("fails the command when a check fails", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "postmark", map[string]string{"USERNAME": "hook", "PASSWORD": "secret"})

		code, output := tc.Command("email:inbound-doctor")

		assert.NotZero(t, code)
		assert.Contains(t, output, "email.inbound_mx")
		assert.Contains(t, output, "email.inbound_source.postmark")
	})

	t.Run("reports a source that cannot be built", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		doctor := *tc.App.Actions.InboundDoctor
		doctor.Config.Inbound.Drivers = []string{"mailgun"}

		check := checksOf(t, doctor.Handle(t.Context()), "email.inbound_source.mailgun")
		assert.Equal(t, emailactions.CheckFail, check.Status)
	})
}

func TestCloudflareWorkerCommand(t *testing.T) {
	t.Parallel()

	t.Run("prints the worker with the install's address and never a secret", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "cloudflare", map[string]string{"SECRET": cloudflareSecret})

		code, output := tc.Command("email:cloudflare-worker")

		require.Zero(t, code, output)
		assert.Contains(t, output, "/webhooks/cloudflare/inbound")
		assert.Contains(t, output, cloudflare.HeaderSignature)
		assert.NotContains(t, output, cloudflareSecret)
	})

	t.Run("fills the address without doubling a slash", func(t *testing.T) {
		t.Parallel()

		files := cloudflareworker.Render("https://app.example/")

		assert.Contains(t, files.Config, `"https://app.example/webhooks/cloudflare/inbound"`)
		for _, header := range []string{
			cloudflare.HeaderTimestamp, cloudflare.HeaderSignature, cloudflare.HeaderEnvelopeFrom,
			cloudflare.HeaderEnvelopeTo, cloudflare.HeaderFallback,
		} {
			assert.Contains(t, files.Script, header)
		}
	})
}

func TestResendBackfillCommand(t *testing.T) {
	t.Parallel()

	secret := webhooks.NewSecret()
	options := map[string]string{"WEBHOOK_SECRET": secret.Encode(), "API_KEY": "re_test", "BASE_URL": resendAPI}

	t.Run("refuses a date it cannot read", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "resend", options)

		code, output := tc.Command("email:resend-backfill", "--since", "yesterday")

		assert.NotZero(t, code)
		assert.Contains(t, output, "--since")
	})

	t.Run("feeds through what resend holds", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "resend", options)
		raw, address := inboundRaw(tc)

		list, err := json.Marshal(map[string]any{"data": []map[string]any{{
			"id": "em-9", "created_at": "2030-01-01T00:00:00Z", "from": "bob@customer.test", "to": []string{address},
		}}})
		require.NoError(t, err)
		link, err := json.Marshal(map[string]any{"raw": map[string]string{"download_url": resendAPI + "/raw/em-9"}})
		require.NoError(t, err)

		httpclienttest.New(tc.App.HTTP).PreventStrayRequests().
			Stub("GET "+resendAPI+"/emails/receiving?limit=100", ok(list)).
			Stub("GET "+resendAPI+"/emails/receiving/em-9", ok(link)).
			Stub("GET "+resendAPI+"/raw/em-9", ok(raw))

		count, err := tc.App.Actions.BackfillResend.Handle(t.Context(), tc.Clock.Now().AddDate(-1, 0, 0))
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		assertBecameConversation(tc, "resend")
	})

	t.Run("keeps a mail it fed through once", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "resend", options)
		raw, address := inboundRaw(tc)

		list, err := json.Marshal(map[string]any{"data": []map[string]any{{
			"id": "em-9", "created_at": "2030-01-01T00:00:00Z", "from": "bob@customer.test", "to": []string{address},
		}}})
		require.NoError(t, err)
		link, err := json.Marshal(map[string]any{"raw": map[string]string{"download_url": resendAPI + "/raw/em-9"}})
		require.NoError(t, err)

		httpclienttest.New(tc.App.HTTP).PreventStrayRequests().
			Stub("GET "+resendAPI+"/emails/receiving?limit=100", ok(list)).
			Stub("GET "+resendAPI+"/emails/receiving/em-9", ok(link)).
			Stub("GET "+resendAPI+"/raw/em-9", ok(raw))

		for range 2 {
			_, err := tc.App.Actions.BackfillResend.Handle(t.Context(), tc.Clock.Now().AddDate(-1, 0, 0))
			require.NoError(t, err)
		}

		assertBecameConversation(tc, "resend")
	})

	t.Run("says how far it got when a mail cannot be kept", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "resend", options)
		raw, address := inboundRaw(tc)

		list, err := json.Marshal(map[string]any{"data": []map[string]any{{
			"id": "em-9", "created_at": "2030-01-01T00:00:00Z", "from": "bob@customer.test", "to": []string{address},
		}}})
		require.NoError(t, err)
		link, err := json.Marshal(map[string]any{"raw": map[string]string{"download_url": resendAPI + "/raw/em-9"}})
		require.NoError(t, err)

		httpclienttest.New(tc.App.HTTP).PreventStrayRequests().
			Stub("GET "+resendAPI+"/emails/receiving?limit=100", ok(list)).
			Stub("GET "+resendAPI+"/emails/receiving/em-9", ok(link)).
			Stub("GET "+resendAPI+"/raw/em-9", ok(raw))
		tc.BreakTable("inbound_emails")

		count, err := tc.App.Actions.BackfillResend.Handle(t.Context(), tc.Clock.Now().AddDate(-1, 0, 0))

		require.ErrorContains(t, err, "after 0 emails")
		assert.Zero(t, count)
	})

	t.Run("fails the command when resend cannot be reached", func(t *testing.T) {
		t.Parallel()
		tc := sourceCase(t, "resend", options)

		code, output := tc.Command("email:resend-backfill", "--since", "2026-11-01")

		assert.NotZero(t, code)
		assert.Contains(t, output, "after 0 emails")
	})

	t.Run("reports an install without the source", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		_, err := tc.App.Actions.BackfillResend.Handle(t.Context(), tc.Clock.Now())

		require.Error(t, err)
	})
}

func ok(body []byte) httpclient.Response {
	return httpclient.Response{Status: fhttp.StatusOK, Body: body}
}

func TestInboundHealth(t *testing.T) {
	t.Parallel()

	t.Run("loses only the signal when the cache is away", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		health := tc.App.Actions.InboundHealth

		tc.BreakValkey()
		health.RecordMisconfigured(t.Context(), "postmark", "raw email")
		health.ClearMisconfigured(t.Context(), "postmark")

		_, err := health.Misconfigured(t.Context(), "postmark")
		require.Error(t, err)
	})
}
