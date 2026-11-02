package authcheck_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io/fs"
	"net/netip"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/mail/dkim"
	"github.com/dimmihq/dimmi/framework/mailbox/authcheck"
)

/*
domainKey is a signing key with the DNS record that publishes it.
*/
type domainKey struct {
	private string
	record  string
}

func newKey(t *testing.T) domainKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	return domainKey{
		private: string(pem.EncodeToMemory(&pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
		})),
		record: "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(public),
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)

	return raw
}

/*
sign signs the message for the domain, the way its outgoing server
would.
*/
func sign(t *testing.T, key domainKey, domain string, raw []byte) []byte {
	t.Helper()

	signer, err := dkim.New(dkim.Options{Domain: domain, Selector: "sel", PrivateKey: key.private})
	require.NoError(t, err)

	signed, err := signer.Sign(raw)
	require.NoError(t, err)

	return signed
}

/*
zone publishes the sender domain's key, SPF and DMARC records with the
DMARC record given.
*/
func zone(key domainKey, record string) *dns.Fake {
	fake := dns.NewFake().
		Set("sel._domainkey.sender.example", dns.Records{TXT: []string{key.record}}).
		Set("sel._domainkey.relay.example", dns.Records{TXT: []string{key.record}}).
		Set("sel._domainkey.mail.sender.example", dns.Records{TXT: []string{key.record}}).
		Set("sender.example", dns.Records{
			TXT: []string{"v=spf1 ip4:192.0.2.0/24 mx -all"},
			MX:  []dns.MX{{Host: "mx.sender.example", Pref: 10}},
		}).
		Set("bounces.sender.example", dns.Records{
			TXT:   []string{"v=spf1 a -all"},
			Addrs: []netip.Addr{netip.MustParseAddr("198.51.100.7")},
		}).
		Set("mx.sender.example", dns.Records{Addrs: []netip.Addr{netip.MustParseAddr("203.0.113.9")}})

	if record != "" {
		fake.Set("_dmarc.sender.example", dns.Records{TXT: []string{record}})
	}

	return fake
}

func TestVerify(t *testing.T) {
	t.Parallel()

	key := newKey(t)
	message := fixture(t, "message.eml")

	t.Run("passes an aligned signature under a reject policy", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, "v=DMARC1; p=reject"))

		got := checker.Verify(t.Context(), authcheck.Input{Raw: sign(t, key, "sender.example", message)})

		assert.Equal(t, "sender.example", got.FromDomain)
		assert.Equal(t, []authcheck.DKIMResult{{Domain: "sender.example", Result: authcheck.Pass}}, got.DKIM)
		assert.Equal(t, authcheck.None, got.SPF)
		assert.Equal(t, authcheck.Pass, got.DMARC)
		assert.Equal(t, "reject", got.DMARCPolicy)
		assert.Equal(t, "sender.example", got.AlignedDomain)
	})

	t.Run("passes aligned spf from an allowed server", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, "v=DMARC1; p=quarantine"))

		got := checker.Verify(t.Context(), authcheck.Input{
			Raw: message, RemoteIP: netip.MustParseAddr("192.0.2.10"), HELO: "out.sender.example",
			MailFrom: "bounce@sender.example",
		})

		assert.Equal(t, authcheck.Pass, got.SPF)
		assert.Equal(t, "sender.example", got.SPFDomain)
		assert.Equal(t, authcheck.Pass, got.DMARC)
		assert.Equal(t, "sender.example", got.AlignedDomain)
	})

	t.Run("follows mx and a mechanisms", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, ""))

		viaMX := checker.Verify(t.Context(), authcheck.Input{
			Raw: message, RemoteIP: netip.MustParseAddr("203.0.113.9"), MailFrom: "a@sender.example",
		})
		viaA := checker.Verify(t.Context(), authcheck.Input{
			Raw: message, RemoteIP: netip.MustParseAddr("198.51.100.7"), MailFrom: "a@bounces.sender.example",
		})

		assert.Equal(t, authcheck.Pass, viaMX.SPF)
		assert.Equal(t, authcheck.Pass, viaA.SPF)
		assert.Equal(t, "bounces.sender.example", viaA.AlignedDomain)
	})

	t.Run("fails spf from another server and dmarc without an aligned pass", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, "v=DMARC1; p=reject"))

		got := checker.Verify(t.Context(), authcheck.Input{
			Raw: message, RemoteIP: netip.MustParseAddr("2001:db8::1"), MailFrom: "a@sender.example",
		})

		assert.Equal(t, authcheck.Fail, got.SPF)
		assert.Equal(t, authcheck.Fail, got.DMARC)
		assert.Empty(t, got.AlignedDomain)
	})

	t.Run("does not align a signature from another domain", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, "v=DMARC1; p=reject"))

		got := checker.Verify(t.Context(), authcheck.Input{Raw: sign(t, key, "relay.example", message)})

		assert.Equal(t, authcheck.Pass, got.DKIM[0].Result)
		assert.Equal(t, authcheck.Fail, got.DMARC)
	})

	t.Run("requires an exact domain under strict alignment", func(t *testing.T) {
		t.Parallel()
		signed := sign(t, key, "mail.sender.example", message)

		in := authcheck.Input{Raw: signed}

		relaxed := authcheck.New(zone(key, "v=DMARC1; p=reject")).Verify(t.Context(), in)
		strict := authcheck.New(zone(key, "v=DMARC1; p=reject; adkim=s")).Verify(t.Context(), in)

		assert.Equal(t, authcheck.Pass, relaxed.DMARC)
		assert.Equal(t, authcheck.Fail, strict.DMARC)
	})

	t.Run("reports an aligned pass even without a dmarc record", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, ""))

		got := checker.Verify(t.Context(), authcheck.Input{Raw: sign(t, key, "sender.example", message)})

		assert.Equal(t, authcheck.None, got.DMARC)
		assert.Empty(t, got.DMARCPolicy)
		assert.Equal(t, "sender.example", got.AlignedDomain)
	})

	t.Run("applies the organizational subdomain policy", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, "v=DMARC1; p=reject; sp=quarantine"))

		signed := sign(t, key, "sender.example", fixture(t, "subdomain.eml"))

		got := checker.Verify(t.Context(), authcheck.Input{Raw: signed})

		assert.Equal(t, "mail.sender.example", got.FromDomain)
		assert.Equal(t, "quarantine", got.DMARCPolicy)
		assert.Equal(t, authcheck.Pass, got.DMARC)
	})

	t.Run("takes the receiving provider's spf verdict without an address", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, "v=DMARC1; p=reject"))

		got := checker.Verify(t.Context(), authcheck.Input{
			Raw: message, MailFrom: "bounce@sender.example", SPF: authcheck.Pass,
		})

		assert.Equal(t, authcheck.Pass, got.SPF)
		assert.Equal(t, authcheck.Pass, got.DMARC)
	})

	t.Run("checks the helo name for the null sender", func(t *testing.T) {
		t.Parallel()
		checker := authcheck.New(zone(key, ""))

		got := checker.Verify(t.Context(), authcheck.Input{
			Raw: message, RemoteIP: netip.MustParseAddr("198.51.100.7"), HELO: "Bounces.Sender.Example",
		})

		assert.Equal(t, "bounces.sender.example", got.SPFDomain)
		assert.Equal(t, authcheck.Pass, got.SPF)
	})
}

func TestFailures(t *testing.T) {
	t.Parallel()

	key := newKey(t)
	message := fixture(t, "message.eml")

	t.Run("fails a signature over a changed body", func(t *testing.T) {
		t.Parallel()
		signed := sign(t, key, "sender.example", message)
		tampered := bytes.Replace(signed, []byte("a week"), []byte("a year"), 1)

		got := authcheck.New(zone(key, "")).Verify(t.Context(), authcheck.Input{Raw: tampered})

		assert.Equal(t, authcheck.Fail, got.DKIM[0].Result)
	})

	t.Run("reports a signature whose key is not published", func(t *testing.T) {
		t.Parallel()

		signed := sign(t, key, "sender.example", message)

		got := authcheck.New(dns.NewFake()).Verify(t.Context(), authcheck.Input{Raw: signed})

		assert.Equal(t, authcheck.PermError, got.DKIM[0].Result)
		assert.Equal(t, authcheck.None, got.DMARC)
	})

	t.Run("reports lookups that fail as temporary", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		got := authcheck.New(zone(key, "v=DMARC1; p=reject")).Verify(ctx, authcheck.Input{
			Raw: sign(t, key, "sender.example", message), RemoteIP: netip.MustParseAddr("192.0.2.10"),
			MailFrom: "a@sender.example",
		})

		assert.Equal(t, authcheck.TempError, got.SPF)
		assert.Equal(t, authcheck.TempError, got.DKIM[0].Result)
		assert.Equal(t, authcheck.TempError, got.DMARC)
	})

	t.Run("reports a malformed dmarc record", func(t *testing.T) {
		t.Parallel()

		got := authcheck.New(zone(key, "v=DMARC1; p=sometimes")).Verify(t.Context(), authcheck.Input{Raw: message})

		assert.Equal(t, authcheck.PermError, got.DMARC)
	})

	t.Run("reports a message without a from address", func(t *testing.T) {
		t.Parallel()

		got := authcheck.New(zone(key, "")).Verify(t.Context(), authcheck.Input{Raw: []byte("Subject: x\r\n\r\nbody")})

		assert.Empty(t, got.FromDomain)
		assert.Equal(t, authcheck.PermError, got.DMARC)
		assert.Empty(t, got.DKIM)
	})

	t.Run("reports text that is no message", func(t *testing.T) {
		t.Parallel()

		got := authcheck.New(zone(key, "")).Verify(t.Context(), authcheck.Input{Raw: []byte("not a message")})

		assert.Equal(t, authcheck.PermError, got.DMARC)
	})

	t.Run("verifies only the first signatures of a message with many", func(t *testing.T) {
		t.Parallel()
		signed := message
		for range 6 {
			signed = sign(t, key, "sender.example", signed)
		}

		got := authcheck.New(zone(key, "")).Verify(t.Context(), authcheck.Input{Raw: signed})

		assert.Len(t, got.DKIM, 5)
	})
}
