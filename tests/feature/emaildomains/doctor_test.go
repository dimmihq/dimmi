package emaildomains_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/tests"
)

/*
diagnose runs the doctor on acme.test and returns its report, one line
per finding.
*/
func diagnose(tc *tests.TestCase, production bool) []string {
	tc.T.Helper()

	doctor := &email.DiagnoseEmailDomain{Domains: tc.App.EmailDomains, Lang: tc.App.Lang, Production: production}
	findings, err := doctor.Handle(tc.T.Context(), "acme.test")
	require.NoError(tc.T, err)

	lines := make([]string, 0, len(findings))
	for _, f := range findings {
		lines = append(lines, "["+f.Level+"] "+f.Message)
	}

	return lines
}

func TestEmailDoctor(t *testing.T) {
	t.Parallel()

	t.Run("explains a doubled spf record behind a relay", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t))
		zone.Set(inboundDomain, dns.Records{MX: []dns.MX{{Host: "inbound-smtp.eu-west-1.amazonaws.com", Pref: 10}}}).
			Set("acme.test", dns.Records{TXT: []string{"v=spf1 mx ~all", "v=spf1 a ~all"}}).
			Set("_dmarc.acme.test", dns.Records{TXT: []string{"v=DMARC1; p=none"}})
		tc.ActingAsOwner().PostJSON("/api/v1/email-domains", map[string]any{"domain": "acme.test"}).AssertCreated()

		got := diagnose(tc, false)

		assert.Equal(t, []string{
			"[ok] Customer mail leaves through the SMTP relay localhost.",
			"[info] dimmi never runs a mail server of its own: mail arrives through your provider's inbound service.",
			"[ok] inbound.test receives mail at inbound-smtp.eu-west-1.amazonaws.com.",
			"[info] 1 workspace(s) added acme.test: pending.",
			"[warn] acme.test has two SPF records and mail servers reject both. Replace them with this one record: " +
				"v=spf1 mx include:spf.relay.test ~all",
		}, got)
	})

	t.Run("fails an install that sends nothing and receives nothing", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t)

		got := diagnose(tc, true)

		assert.Contains(t, got[0], "[fail] Customer mail is only written to the log.")
		assert.Contains(t, got, "[fail] inbound.test has no MX record, so forwarded mail never arrives.")
		assert.Contains(t, got, "[info] No workspace has added acme.test yet.")
		assert.Contains(t, got, "[warn] _dmarc.acme.test has no DMARC record. Gmail, Yahoo and Outlook expect one. "+
			"Add the DMARC record below.")
	})

	t.Run("names what a provider's mailer lacks", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, postmarkEnv("acme.test"), tests.WithEnv("DIMMI_MAIL__MAILERS__CUSTOMER__TOKEN="))
		zone.Set("_dmarc.acme.test", dns.Records{TXT: []string{"v=DMARC1; p=none"}})

		got := diagnose(tc, false)

		assert.Contains(t, got, "[fail] The customer mailer has no token.")
		assert.Contains(t, got,
			"[warn] No event webhook is set for postmark, so bounces and complaints are not recorded.")
		assert.Contains(t, got, "[info] No signing record shows at <yyyymmddhhmmss>pm._domainkey.acme.test; "+
			"check the selector in your provider's dashboard.")
	})

	t.Run("warns of a relay that looks like sending straight to recipients", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t, tests.WithEnv(
			"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=smtp", "DIMMI_MAIL__MAILERS__CUSTOMER__HOST=mx.example.test",
			"DIMMI_MAIL__MAILERS__CUSTOMER__PORT=25",
		))

		got := diagnose(tc, false)

		assert.Contains(t, got[1], "[warn] The relay is reached on port 25 without a login")
		assert.Contains(t, got[2], "[info] No SPF include is set for the relay")
	})

	t.Run("refuses a name that is no domain", func(t *testing.T) {
		t.Parallel()
		tc, _ := domainCase(t)

		code, out := tc.Command("email:doctor", "not a domain")

		assert.NotZero(t, code)
		assert.Contains(t, out, "Enter a domain you own")
	})
}
