package saml_test

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/saml"
	"github.com/dimmihq/dimmi/framework/saml/samltest"
)

const (
	spEntity = "https://sp.example/auth/saml/ssoc_1/metadata"
	acsURL   = "https://sp.example/auth/saml/ssoc_1/acs"
)

/*
setup is a service provider trusting a fresh test identity provider, its
clock stopped at a fixed time.
*/
type setup struct {
	clock    *clock.Fake
	idp      *samltest.IDP
	provider *saml.Provider
}

func newSetup(t *testing.T, certificates ...*x509.Certificate) setup {
	t.Helper()

	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)

	idp, err := samltest.New("https://idp.example/entity", "https://idp.example/sso", now)
	require.NoError(t, err)

	pair, err := saml.NewKeyPair(spEntity, now)
	require.NoError(t, err)

	certificate, err := saml.ParseCertificate(pair.Certificate)
	require.NoError(t, err)

	block, _ := pem.Decode([]byte(pair.PrivateKey))
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)

	if len(certificates) == 0 {
		certificates = []*x509.Certificate{idp.Certificate()}
	}

	provider, err := saml.New(saml.Config{
		SP: saml.ServiceProvider{
			EntityID: spEntity, ACSURL: acsURL, Certificate: certificate, PrivateKey: crypto.Signer(key),
		},
		IDP:   saml.IdentityProvider{EntityID: idp.EntityID, SSOURL: idp.SSOURL, Certificates: certificates},
		Clock: clk,
	})
	require.NoError(t, err)

	return setup{clock: clk, idp: idp, provider: provider}
}

/*
answer returns a valid answer to the request, changed by the test.
*/
func (s setup) answer(t *testing.T, change func(*samltest.Response)) string {
	t.Helper()

	r := samltest.Response{
		ACSURL: acsURL, Audience: spEntity, InResponseTo: "_req1", NameID: "ada@acme.example",
		Attributes: map[string][]string{"email": {"ada@acme.example"}, "groups": {"eng", "ops"}},
		IssuedAt:   s.clock.Now(), SignAssertion: true, SignResponse: true,
	}
	if change != nil {
		change(&r)
	}

	encoded, err := s.idp.Encode(r)
	require.NoError(t, err)

	return encoded
}

func TestConsume(t *testing.T) {
	t.Parallel()

	t.Run("a valid signed response is accepted", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		a, err := s.provider.Consume(s.answer(t, nil), "_req1")
		require.NoError(t, err)
		assert.Equal(t, "ada@acme.example", a.NameID)
		assert.Equal(t, samltest.EmailFormat, a.NameIDFormat)
		assert.Equal(t, []string{"eng", "ops"}, a.Attributes["groups"])
		assert.NotEmpty(t, a.AssertionID)
		assert.Equal(t, s.clock.Now().Add(5*time.Minute), a.NotOnOrAfter)
	})

	t.Run("an assertion signed alone is accepted", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) { r.SignResponse = false }), "_req1")
		require.NoError(t, err)
	})

	t.Run("an unsigned response is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) {
			r.SignResponse, r.SignAssertion = false, false
		}), "_req1")
		require.ErrorIs(t, err, saml.ErrInvalidSignature)
	})

	t.Run("a response signed by another key is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)
		other := newSetup(t)

		_, err := s.provider.Consume(other.answer(t, nil), "_req1")
		require.Error(t, err)
	})

	t.Run("a response with an extra unsigned assertion is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) {
			r.SignResponse, r.ExtraUnsignedAssertion = false, true
		}), "_req1")
		require.Error(t, err)
	})

	t.Run("a wrapped signature is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		a, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) {
			r.SignResponse, r.WrapSignature = false, true
		}), "_req1")
		require.Error(t, err)
		assert.NotEqual(t, "intruder@attacker.example", a.NameID)
	})

	t.Run("a comment inside the name id does not truncate it", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		a, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) {
			r.NameID, r.NameIDComment = "ada@acme.example.attacker.example", len("ada@acme.example")
		}), "_req1")
		if err == nil {
			assert.Equal(t, "ada@acme.example.attacker.example", a.NameID)
		}
	})

	t.Run("an expired assertion is refused, and one within the skew is not", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)
		encoded := s.answer(t, nil)

		s.clock.Advance(7 * time.Minute)
		_, err := s.provider.Consume(encoded, "_req1")
		require.NoError(t, err)

		s.clock.Advance(2 * time.Minute)
		_, err = s.provider.Consume(encoded, "_req1")
		require.ErrorIs(t, err, saml.ErrExpired)
	})

	t.Run("a wrong audience is refused even though the library only warns", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) { r.Audience = "https://other.example" }),
			"_req1")
		require.ErrorIs(t, err, saml.ErrWrongAudience)
	})

	t.Run("a wrong destination is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) {
			r.Destination = "https://other.example/acs"
		}), "_req1")
		require.ErrorIs(t, err, saml.ErrWrongDestination)
	})

	t.Run("a wrong issuer is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) { r.Issuer = "https://evil.example" }),
			"_req1")
		require.ErrorIs(t, err, saml.ErrWrongIssuer)
	})

	t.Run("an identity provider that said no is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) {
			r.Status = "urn:oasis:names:tc:SAML:2.0:status:Requester"
		}), "_req1")
		require.ErrorIs(t, err, saml.ErrStatusNotSuccess)
	})

	t.Run("a response to another request is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, nil), "_req2")
		require.ErrorIs(t, err, saml.ErrInResponseTo)

		_, err = s.provider.Consume(s.answer(t, func(r *samltest.Response) { r.InResponseTo = "" }), "_req2")
		require.ErrorIs(t, err, saml.ErrInResponseTo)
	})

	t.Run("an unsolicited response is reported as unsolicited", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		a, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) { r.InResponseTo = "" }), "")
		require.ErrorIs(t, err, saml.ErrUnsolicited)
		assert.Equal(t, "ada@acme.example", a.NameID)

		_, err = s.provider.Consume(s.answer(t, nil), "")
		require.ErrorIs(t, err, saml.ErrInResponseTo)
	})

	t.Run("a response signed by a rotated certificate is accepted while listed", func(t *testing.T) {
		t.Parallel()
		old := newSetup(t)
		next := newSetup(t)
		both := newSetup(t, old.idp.Certificate(), next.idp.Certificate())

		for _, from := range []setup{old, next} {
			encoded := from.answer(t, nil)
			_, err := both.provider.Consume(encoded, "_req1")
			require.NoError(t, err)
		}
	})

	t.Run("an assertion not yet valid is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) {
			r.IssuedAt = s.clock.Now().Add(10 * time.Minute)
		}), "_req1")
		require.ErrorIs(t, err, saml.ErrExpired)
	})

	t.Run("an assertion for no audience at all is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) { r.NoAudience = true }), "_req1")
		require.ErrorIs(t, err, saml.ErrWrongAudience)
	})

	t.Run("a confirmation naming another request than its response is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) {
			r.ConfirmationInResponseTo = "_other"
		}), "_req1")
		require.Error(t, err)
	})

	t.Run("an assertion without a name is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(s.answer(t, func(r *samltest.Response) { r.NameID = "" }), "_req1")
		require.ErrorIs(t, err, saml.ErrMalformed)
	})

	t.Run("oversized and broken input is refused before parsing", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		_, err := s.provider.Consume(strings.Repeat("A", 300*1024), "_req1")
		require.ErrorIs(t, err, saml.ErrMalformed)
		_, err = s.provider.Consume("not base64!", "_req1")
		require.ErrorIs(t, err, saml.ErrMalformed)
		_, err = s.provider.Consume(base64.StdEncoding.EncodeToString([]byte("<x/>")), "_req1")
		require.Error(t, err)
	})
}

func TestRequestsAndMetadata(t *testing.T) {
	t.Parallel()

	t.Run("a sign-in is a signed redirect carrying the relay state and the request id", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		address, requestID, err := s.provider.AuthURL("relay-1")
		require.NoError(t, err)
		assert.NotEmpty(t, requestID)

		parsed, err := url.Parse(address)
		require.NoError(t, err)
		assert.Equal(t, "idp.example", parsed.Host)
		assert.Equal(t, "relay-1", parsed.Query().Get("RelayState"))
		assert.NotEmpty(t, parsed.Query().Get("SAMLRequest"))
		assert.Contains(t, parsed.Query().Get("SigAlg"), "rsa-sha256")
		assert.NotEmpty(t, parsed.Query().Get("Signature"))
	})

	t.Run("the service provider metadata names its entity, its acs and its certificate", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		doc, err := s.provider.Metadata()
		require.NoError(t, err)
		assert.Contains(t, string(doc), spEntity)
		assert.Contains(t, string(doc), acsURL)
		assert.Contains(t, string(doc), `use="signing"`)
		assert.Contains(t, string(doc), `use="encryption"`)
	})

	t.Run("identity provider metadata is read with every signing certificate", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)

		idp, err := saml.ParseMetadata(s.idp.Metadata())
		require.NoError(t, err)
		assert.Equal(t, "https://idp.example/entity", idp.EntityID)
		assert.Equal(t, "https://idp.example/sso", idp.SSOURL)
		require.Len(t, idp.Certificates, 1)

		listed := "<EntitiesDescriptor xmlns=\"urn:oasis:names:tc:SAML:2.0:metadata\">" +
			strings.TrimPrefix(string(s.idp.Metadata()), "") + "</EntitiesDescriptor>"
		_, err = saml.ParseMetadata([]byte(listed))
		require.NoError(t, err)
	})

	t.Run("metadata without a redirect binding is refused", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t)
		posted := strings.ReplaceAll(string(s.idp.Metadata()), "HTTP-Redirect", "HTTP-POST")

		_, err := saml.ParseMetadata([]byte(posted))
		require.ErrorIs(t, err, saml.ErrNoRedirectBinding)
		_, err = saml.ParseMetadata([]byte("<nonsense"))
		require.ErrorIs(t, err, saml.ErrInvalidMetadata)
		_, err = saml.ParseMetadata(nil)
		require.ErrorIs(t, err, saml.ErrInvalidMetadata)

		unsigned := strings.ReplaceAll(string(s.idp.Metadata()), `use="signing"`, `use="encryption"`)
		_, err = saml.ParseMetadata([]byte(unsigned))
		require.ErrorIs(t, err, saml.ErrInvalidMetadata)

		broken := strings.Replace(string(s.idp.Metadata()), "<ds:X509Certificate>", "<ds:X509Certificate>AAAA", 1)
		_, err = saml.ParseMetadata([]byte(broken))
		require.ErrorIs(t, err, saml.ErrInvalidMetadata)

		_, err = saml.ParseMetadata([]byte(`<EntitiesDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata"/>`))
		require.ErrorIs(t, err, saml.ErrInvalidMetadata)
	})

	t.Run("a provider without its keys is refused", func(t *testing.T) {
		t.Parallel()

		_, err := saml.New(saml.Config{})
		require.ErrorIs(t, err, saml.ErrInvalidKey)
	})
}
