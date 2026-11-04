package emaildns_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/integrations/email/dns"
)

/*
sesSpec is a domain set up for SES with its custom return path.
*/
func sesSpec() dns.Spec {
	return dns.Spec{
		Domain: "acme.test", OwnershipToken: "tok", Driver: dns.DriverSES,
		Tokens: []string{"t1", "t2", "t3"}, Zone: "dkim.amazonses.com",
		MailFrom: "bounce.acme.test", Region: "eu-west-1",
	}
}

/*
relaySpec is a domain set up for a relay that signs with our key.
*/
func relaySpec() dns.Spec {
	return dns.Spec{
		Domain: "acme.test", OwnershipToken: "tok", Driver: dns.DriverSMTP,
		Selector: "dimmi202611", PublicKey: strings.Repeat("K", 400), SPFInclude: "include:spf.relay.test",
	}
}

func TestExpectedRecords(t *testing.T) {
	t.Parallel()

	t.Run("asks ses domains for ownership, three signing aliases, a return path and dmarc", func(t *testing.T) {
		t.Parallel()

		got := dns.Expected(sesSpec())

		require.Len(t, got, 7)
		assert.Equal(t, dns.Record{
			Purpose: dns.PurposeOwnership, Type: dns.TypeTXT, Host: "_dimmi-verification.acme.test",
			Value: "dimmi-verification=tok", Required: true, Status: dns.StatusMissing,
		}, got[0])
		assert.Equal(t, "t2._domainkey.acme.test", got[2].Host)
		assert.Equal(t, "t2.dkim.amazonses.com", got[2].Value)
		assert.True(t, got[2].Required)
		assert.Equal(t, dns.Record{
			Purpose: dns.PurposeReturnPath, Type: dns.TypeMX, Host: "bounce.acme.test", Priority: 10,
			Value: "feedback-smtp.eu-west-1.amazonses.com", Status: dns.StatusMissing,
		}, got[4])
		assert.Equal(t, "v=spf1 include:amazonses.com ~all", got[5].Value)
		assert.Equal(t, dns.PurposeDMARC, got[6].Purpose)
		assert.False(t, got[6].Required)
	})

	t.Run("leaves the return path out when ses has none", func(t *testing.T) {
		t.Parallel()
		spec := sesSpec()
		spec.MailFrom = ""

		assert.Len(t, dns.Expected(spec), 5)
	})

	t.Run("asks relay domains for our signing key and the relay's spf include", func(t *testing.T) {
		t.Parallel()

		got := dns.Expected(relaySpec())

		require.Len(t, got, 4)
		assert.Equal(t, "dimmi202611._domainkey.acme.test", got[1].Host)
		assert.Equal(t, "v=DKIM1; k=rsa; p="+strings.Repeat("K", 400), got[1].Value)
		assert.True(t, got[1].Required)
		assert.Equal(t, "v=spf1 include:spf.relay.test ~all", got[2].Value)
		assert.False(t, got[2].Required)
	})

	t.Run("leaves the relay spf out when no include is known", func(t *testing.T) {
		t.Parallel()
		spec := relaySpec()
		spec.SPFInclude = ""

		assert.Len(t, dns.Expected(spec), 3)
	})

	t.Run("gives providers set up by hand guidance that never blocks", func(t *testing.T) {
		t.Parallel()

		for driver, selector := range map[string]string{
			dns.DriverPostmark: "<yyyymmddhhmmss>pm", dns.DriverResend: "resend",
			dns.DriverMailgun: "k1", dns.DriverSendGrid: "s1",
		} {
			got := dns.Expected(dns.Spec{Domain: "acme.test", OwnershipToken: "tok", Driver: driver})

			require.Len(t, got, 5, driver)
			assert.Equal(t, selector, got[1].Selector, driver)
			for _, record := range got[1:4] {
				assert.Equal(t, dns.StatusInfo, record.Status, driver)
				assert.False(t, record.Required, driver)
			}
		}
	})

	t.Run("asks an unknown driver for ownership and dmarc only", func(t *testing.T) {
		t.Parallel()

		assert.Len(t, dns.Expected(dns.Spec{Domain: "acme.test", Driver: "log"}), 2)
	})
}

func TestMergeSPF(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, existing, want string
	}{
		{"starts a record when there is none", "", "v=spf1 include:x.test ~all"},
		{
			"adds the include before a soft fail", "v=spf1 include:_spf.google.com ~all",
			"v=spf1 include:_spf.google.com include:x.test ~all",
		},
		{"adds the include before a hard fail", "v=spf1 mx -all", "v=spf1 mx include:x.test -all"},
		{"keeps a record that already includes it", "v=spf1 INCLUDE:x.test ~all", "v=spf1 INCLUDE:x.test ~all"},
		{"appends to a record without an all", "v=spf1 mx", "v=spf1 mx include:x.test"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.want, dns.MergeSPF(c.existing, "include:x.test"))
		})
	}
}

func TestZoneFile(t *testing.T) {
	t.Parallel()

	t.Run("splits long text into strings of 255 bytes", func(t *testing.T) {
		t.Parallel()

		chunks := dns.TXTChunks(strings.Repeat("a", 600))

		require.Len(t, chunks, 3)
		assert.Len(t, chunks[0], 255)
		assert.Len(t, chunks[2], 90)
		assert.Equal(t, []string{"short"}, dns.TXTChunks("short"))
	})

	t.Run("writes every record with a value as a zone file line", func(t *testing.T) {
		t.Parallel()
		records := dns.Expected(sesSpec())
		records = append(records, dns.Expected(relaySpec())[1], dns.Record{Type: dns.TypeTXT, Host: "x.acme.test"})

		got := dns.BIND(records)

		assert.Contains(t, got, "_dimmi-verification.acme.test. 3600 IN TXT \"dimmi-verification=tok\"\n")
		assert.Contains(t, got, "t1._domainkey.acme.test. 3600 IN CNAME t1.dkim.amazonses.com.\n")
		assert.Contains(t, got, "bounce.acme.test. 3600 IN MX 10 feedback-smtp.eu-west-1.amazonses.com.\n")
		assert.Contains(t, got, `IN TXT "v=DKIM1; k=rsa; p=`+strings.Repeat("K", 237)+`" "`)
		assert.NotContains(t, got, "x.acme.test")
	})
}
