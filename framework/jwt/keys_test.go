package jwt_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/jwt"
)

func TestNewKey(t *testing.T) {
	t.Parallel()

	t.Run("accepts private and public halves", func(t *testing.T) {
		t.Parallel()
		ed := edKey(t)
		ec := ecKey(t, elliptic.P256())

		_, err := jwt.NewKey("k", jwt.EdDSA, ed.Public())
		require.NoError(t, err)
		_, err = jwt.NewKey("k", jwt.ES256, &ec.PublicKey)
		require.NoError(t, err)
	})

	t.Run("refuses weak keys", func(t *testing.T) {
		t.Parallel()

		_, err := jwt.NewKey("k", jwt.HS256, []byte("short"))
		require.ErrorIs(t, err, jwt.ErrWeakKey)
		_, err = jwt.NewKey("k", jwt.RS256, rsaKey(t, 1024))
		require.ErrorIs(t, err, jwt.ErrWeakKey)
	})

	t.Run("refuses material that does not fit the algorithm", func(t *testing.T) {
		t.Parallel()
		ed := edKey(t)

		for alg, material := range map[jwt.Algorithm]any{
			jwt.HS256:              ed.Public(),
			jwt.RS256:              secret(),
			jwt.ES256:              ecKey(t, elliptic.P384()),
			jwt.EdDSA:              secret(),
			"none":                 secret(),
			jwt.Algorithm("PS256"): rsaKey(t, 2048),
		} {
			_, err := jwt.NewKey("k", alg, material)
			require.ErrorIs(t, err, jwt.ErrUnsupportedKey, alg)
		}
	})

	t.Run("cannot sign with a public key", func(t *testing.T) {
		t.Parallel()
		key, err := jwt.NewKey("k", jwt.EdDSA, edKey(t).Public())
		require.NoError(t, err)

		_, err = jwt.Sign(valid(), key)

		require.ErrorIs(t, err, jwt.ErrUnsupportedKey)
	})
}

func TestParsePublicKeyPEM(t *testing.T) {
	t.Parallel()

	t.Run("reads a public key", func(t *testing.T) {
		t.Parallel()
		ed := edKey(t)
		der, err := x509.MarshalPKIXPublicKey(ed.Public())
		require.NoError(t, err)

		key, err := jwt.ParsePublicKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

		require.NoError(t, err)
		assert.Equal(t, ed.Public(), key)
	})

	t.Run("refuses a private key", func(t *testing.T) {
		t.Parallel()
		der, err := x509.MarshalPKCS8PrivateKey(edKey(t))
		require.NoError(t, err)

		_, err = jwt.ParsePublicKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

		require.ErrorIs(t, err, jwt.ErrPrivateKey)
	})

	t.Run("refuses what is not a public key", func(t *testing.T) {
		t.Parallel()

		_, err := jwt.ParsePublicKeyPEM([]byte("not pem"))
		require.ErrorIs(t, err, jwt.ErrUnsupportedKey)
		_, err = jwt.ParsePublicKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte{1}}))
		require.ErrorIs(t, err, jwt.ErrUnsupportedKey)
	})
}

/*
keySet is a provider's signing keys and its published JWKS.
*/
type keySet struct {
	rsa  *rsa.PrivateKey
	ec   *ecdsa.PrivateKey
	ed   ed25519.PrivateKey
	jwks []byte
}

func newKeySet(t *testing.T) keySet {
	t.Helper()
	set := keySet{rsa: rsaKey(t, 2048), ec: ecKey(t, elliptic.P256()), ed: edKey(t)}

	data, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &set.rsa.PublicKey, KeyID: "rsa", Use: "sig"},
		{Key: &set.ec.PublicKey, KeyID: "ec", Algorithm: "ES256"},
		{Key: set.ed.Public(), KeyID: "ed"},
		{Key: &set.rsa.PublicKey, KeyID: "enc", Use: "enc"},
		{Key: secret(), KeyID: "oct", Algorithm: "HS256"},
		{Key: &ecKey(t, elliptic.P384()).PublicKey, KeyID: "p384"},
	}})
	require.NoError(t, err)
	set.jwks = data

	return set
}

func TestParseJWKS(t *testing.T) {
	t.Parallel()

	t.Run("keeps supported signing keys pinned to their algorithm", func(t *testing.T) {
		t.Parallel()

		keys, err := jwt.ParseJWKS(newKeySet(t).jwks)

		require.NoError(t, err)
		pinned := map[string]jwt.Algorithm{}
		for _, key := range keys {
			pinned[key.ID] = key.Algorithm
		}
		assert.Equal(t, map[string]jwt.Algorithm{"rsa": jwt.RS256, "ec": jwt.ES256, "ed": jwt.EdDSA}, pinned)
	})

	t.Run("reports a document that is not a jwks", func(t *testing.T) {
		t.Parallel()

		_, err := jwt.ParseJWKS([]byte(`[]`))

		require.Error(t, err)
	})
}

/*
publisher serves a JWKS document and counts fetches.
*/
type publisher struct {
	body  []byte
	err   error
	calls int
}

func (p *publisher) fetch(_ context.Context, url string) ([]byte, error) {
	p.calls++
	if url != "https://idp.example.com/jwks" {
		return nil, errors.New("unexpected url")
	}

	return p.body, p.err
}

func TestJWKS(t *testing.T) {
	t.Parallel()

	sign := func(t *testing.T, id string, alg jwt.Algorithm, material any) string {
		t.Helper()
		key, err := jwt.NewKey(id, alg, material)
		require.NoError(t, err)
		token, err := jwt.Sign(valid(), key)
		require.NoError(t, err)

		return token
	}
	opts := jwt.Options{Algorithms: []jwt.Algorithm{jwt.RS256, jwt.ES256, jwt.EdDSA}, Clock: clock.NewFake(testNow())}

	t.Run("verifies tokens against published keys and caches them", func(t *testing.T) {
		t.Parallel()
		set := newKeySet(t)
		source := &publisher{body: set.jwks}
		c := clock.NewFake(testNow())
		jwks := jwt.NewJWKS("https://idp.example.com/jwks", source.fetch, jwt.JWKSOptions{Clock: c})

		for _, token := range []string{
			sign(t, "rsa", jwt.RS256, set.rsa), sign(t, "ec", jwt.ES256, set.ec), sign(t, "ed", jwt.EdDSA, set.ed),
		} {
			_, err := jwks.Verify(t.Context(), token, opts)
			require.NoError(t, err)
		}
		assert.Equal(t, 1, source.calls)

		c.Travel(11 * time.Minute)
		_, err := jwks.Keys(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 2, source.calls)
	})

	t.Run("refetches once for an unknown key", func(t *testing.T) {
		t.Parallel()
		set := newKeySet(t)
		source := &publisher{body: []byte(`{"keys":[]}`)}
		c := clock.NewFake(testNow())
		jwks := jwt.NewJWKS("https://idp.example.com/jwks", source.fetch, jwt.JWKSOptions{Clock: c})
		token := sign(t, "ed", jwt.EdDSA, set.ed)

		_, err := jwks.Verify(t.Context(), token, opts)
		require.ErrorIs(t, err, jwt.ErrUnknownKey)
		assert.Equal(t, 1, source.calls)

		c.Travel(2 * time.Minute)
		source.body = set.jwks
		_, err = jwks.Verify(t.Context(), token, opts)
		require.NoError(t, err)
		assert.Equal(t, 2, source.calls)
	})

	t.Run("reports fetch and parse failures", func(t *testing.T) {
		t.Parallel()
		source := &publisher{err: errors.New("connection refused")}
		jwks := jwt.NewJWKS("https://idp.example.com/jwks", source.fetch, jwt.JWKSOptions{})

		_, err := jwks.Verify(t.Context(), "token", opts)
		require.ErrorContains(t, err, "fetch jwks https://idp.example.com/jwks: connection refused")

		source.err, source.body = nil, []byte("nope")
		_, err = jwks.Keys(t.Context())
		require.ErrorContains(t, err, "parse jwks")
	})

	t.Run("reports a failed refetch", func(t *testing.T) {
		t.Parallel()
		source := &publisher{body: []byte(`{"keys":[]}`)}
		c := clock.NewFake(testNow())
		jwks := jwt.NewJWKS("https://idp.example.com/jwks", source.fetch, jwt.JWKSOptions{Clock: c})
		_, err := jwks.Keys(t.Context())
		require.NoError(t, err)

		c.Travel(2 * time.Minute)
		source.err = errors.New("down")
		_, err = jwks.Verify(t.Context(), sign(t, "ed", jwt.EdDSA, edKey(t)), opts)

		require.ErrorContains(t, err, "down")
	})
}
