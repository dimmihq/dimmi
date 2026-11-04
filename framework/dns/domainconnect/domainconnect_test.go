package domainconnect_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/dns/domainconnect"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
)

func template() domainconnect.Template {
	return domainconnect.Template{ProviderID: "example.org", ServiceID: "mail"}
}

func discover(t *testing.T, zone *dns.Fake, client *httpclient.Client) (domainconnect.Host, bool) {
	t.Helper()

	return domainconnect.Finder{Resolver: zone, Client: client}.Discover(t.Context(), "acme.test", template())
}

const (
	settingsURL = "https://api.host.test/v2/acme.test/settings"
	templateURL = "https://api.host.test/v2/domainTemplates/providers/example.org/services/mail"
)

func newClient(t *testing.T) (*httpclient.Client, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	return client, httpclienttest.New(client).PreventStrayRequests()
}

func zone() *dns.Fake {
	return dns.NewFake().Set("_domainconnect.acme.test", dns.Records{TXT: []string{"api.host.test"}})
}

func answer(status int, body string) httpclient.Response {
	return httpclient.Response{Status: status, Body: []byte(body)}
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	settings := `{"providerName":"Host","urlSyncUX":"https://ux.host.test/","urlAPI":"https://api.host.test"}`

	t.Run("finds a host that knows the template", func(t *testing.T) {
		t.Parallel()
		client, fake := newClient(t)
		fake.Stub("GET "+settingsURL, answer(http.StatusOK, settings)).
			Stub("GET "+templateURL, answer(http.StatusOK, "{}"))

		host, ok := discover(t, zone(), client)

		require.True(t, ok)
		assert.Equal(t, domainconnect.Host{Name: "Host", SyncUX: "https://ux.host.test"}, host)
	})

	t.Run("offers nothing when any step fails", func(t *testing.T) {
		t.Parallel()

		cases := map[string]func(*httpclienttest.Fake){
			"template unknown": func(f *httpclienttest.Fake) {
				f.Stub("GET "+settingsURL, answer(http.StatusOK, settings)).
					Stub("GET "+templateURL, answer(http.StatusNotFound, ""))
			},
			"settings refused": func(f *httpclienttest.Fake) {
				f.Stub("GET "+settingsURL, answer(http.StatusNotFound, ""))
			},
			"settings garbled": func(f *httpclienttest.Fake) {
				f.Stub("GET "+settingsURL, answer(http.StatusOK, "nope"))
			},
			"host unreachable": func(*httpclienttest.Fake) {},
		}

		for name, stub := range cases {
			client, fake := newClient(t)
			stub(fake)

			_, ok := discover(t, zone(), client)

			assert.False(t, ok, name)
		}

		client, _ := newClient(t)
		_, ok := discover(t, dns.NewFake(), client)
		assert.False(t, ok, "no record")
	})
}

func TestApplyURL(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	host := domainconnect.Host{Name: "Host", SyncUX: "https://ux.host.test"}

	t.Run("signs the query so the host can check it", func(t *testing.T) {
		t.Parallel()

		for _, block := range []*pem.Block{
			{Type: "PRIVATE KEY", Bytes: pkcs8},
			{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)},
		} {
			link, err := domainconnect.ApplyURL(host, template(), domainconnect.Apply{
				Domain: "acme.test", Subdomain: "mail", Variables: map[string]string{"token": "t1"},
				RedirectURI: "https://app.test/back", PrivateKey: string(pem.EncodeToMemory(block)), KeyHost: "_dck",
			})
			require.NoError(t, err)

			prefix := "https://ux.host.test/v2/domainTemplates/providers/example.org/services/mail/apply?"
			require.True(t, strings.HasPrefix(link, prefix))
			signed, tail, found := strings.Cut(strings.TrimPrefix(link, prefix), "&key=")
			require.True(t, found)
			assert.Equal(t, "_dck", tail)
			query, sig, found := strings.Cut(signed, "&sig=")
			require.True(t, found)
			raw, err := url.QueryUnescape(sig)
			require.NoError(t, err)
			signature, err := base64.StdEncoding.DecodeString(raw)
			require.NoError(t, err)
			digest := sha256.Sum256([]byte(query))
			require.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature))
			assert.Contains(t, query, "domain=acme.test")
			assert.Contains(t, query, "host=mail")
			assert.Contains(t, query, "token=t1")
		}
	})

	t.Run("refuses a key it cannot read", func(t *testing.T) {
		t.Parallel()

		broken := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("x")}))
		for _, text := range []string{"nope", broken} {
			apply := domainconnect.Apply{Domain: "acme.test", PrivateKey: text}
			_, err := domainconnect.ApplyURL(host, template(), apply)

			require.ErrorIs(t, err, domainconnect.ErrInvalidKey)
		}
	})
}
