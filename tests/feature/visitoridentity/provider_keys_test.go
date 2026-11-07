package visitoridentity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/testkit"
)

const (
	providerJWKS   = "https://acme.clerk.test/.well-known/jwks.json"
	providerIssuer = "https://acme.clerk.test"
)

/*
publish makes the provider's key set answer with the keys.
*/
func (d *desk) publish(keys map[string]testkit.KeyPair) {
	d.t.Helper()
	httpclienttest.New(d.tc.App.HTTP).Stub(providerJWKS, httpclient.Response{Body: testkit.JWKS(d.t, keys)})
}

/*
providerToken mints a token the way an auth provider would: ES256 under
its own key id, with its issuer, issued now.
*/
func (d *desk) providerToken(pair testkit.KeyPair, kid, issuer string) string {
	t, now := d.t, d.tc.Clock.Now()
	t.Helper()
	key, err := jwt.NewKey(kid, jwt.ES256, pair.Private)
	require.NoError(t, err)
	token, err := jwt.Sign(jwt.Claims{
		Issuer: issuer, Subject: "user_2abc", Audience: []string{"https://api.acme.test"}, IssuedAt: now,
		Expiry: now.Add(time.Minute), Extra: map[string]any{"email": "ada@acme.test"},
	}, key)
	require.NoError(t, err)
	return token
}

func TestProviderKeySets(t *testing.T) {
	t.Parallel()

	t.Run("signs customers in with an auth provider's own tokens", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		pair := testkit.P256(t)
		d.publish(map[string]testkit.KeyPair{"k1": pair})
		res := d.owner.PostJSON("/api/v1/identity-secrets", map[string]any{
			"name": "Clerk", "algorithm": "ES256", "jwks_url": providerJWKS,
		}).AssertCreated()
		res.AssertJSONPath("data.source", "jwks").AssertJSONPath("data.kid", "")
		assert.Empty(t, res.String("data.secret"))

		v := d.signedIn(d.providerToken(pair, "k1", providerIssuer), nil)
		assert.Equal(t, "user_2abc", *d.contact(v.contact).ExternalID)
	})

	t.Run("refuses a token from another issuer or an unknown key", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		pair := testkit.P256(t)
		d.publish(map[string]testkit.KeyPair{"k1": pair})
		d.owner.PostJSON("/api/v1/identity-secrets", map[string]any{
			"algorithm": "ES256", "jwks_url": providerJWKS,
		}).AssertCreated()
		d.signIn(d.providerToken(pair, "k1", "https://evil.test"), nil).AssertUnauthorized().
			AssertJSONPath("code", "token_issuer_mismatch")
		d.signIn(d.providerToken(testkit.P256(t), "k9", providerIssuer), nil).AssertUnauthorized().
			AssertJSONPath("code", "token_kid_unknown")
	})

	t.Run("refuses an address that does not answer with keys", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		httpclienttest.New(d.tc.App.HTTP).Stub(providerJWKS, httpclient.Response{Status: 500})
		d.owner.PostJSON("/api/v1/identity-secrets", map[string]any{"algorithm": "ES256", "jwks_url": providerJWKS}).
			AssertStatus(422).AssertJSONPath("code", "jwks_unreachable")
		d.publish(map[string]testkit.KeyPair{"k1": testkit.Ed25519(t)})
		d.owner.PostJSON("/api/v1/identity-secrets", map[string]any{"algorithm": "ES256", "jwks_url": providerJWKS}).
			AssertStatus(422).AssertJSONPath("code", "jwks_invalid")
		d.owner.PostJSON("/api/v1/identity-secrets", map[string]any{
			"algorithm": "ES256", "jwks_url": "https://keys.acme.test/keys.json",
		}).AssertStatus(422).AssertJSONPath("code", "issuer_required")
	})

	t.Run("picks up a key the provider rotated in", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		old, rotated := testkit.P256(t), testkit.P256(t)
		d.publish(map[string]testkit.KeyPair{"k1": old})
		d.owner.PostJSON("/api/v1/identity-secrets", map[string]any{
			"algorithm": "ES256", "jwks_url": providerJWKS,
		}).AssertCreated()
		d.publish(map[string]testkit.KeyPair{"k1": old, "k2": rotated})
		d.tc.Clock.Travel(6 * time.Minute)
		d.signIn(d.providerToken(rotated, "k2", providerIssuer), nil).AssertCreated()
	})

	t.Run("stops verifying when the provider has been unreachable for a day", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		pair := testkit.P256(t)
		d.publish(map[string]testkit.KeyPair{"k1": pair})
		created := d.owner.PostJSON("/api/v1/identity-secrets", map[string]any{
			"algorithm": "ES256", "jwks_url": providerJWKS,
		}).AssertCreated()
		httpclienttest.New(d.tc.App.HTTP).Stub(providerJWKS, httpclient.Response{Status: 503})
		d.tc.Clock.Travel(2 * time.Hour)
		d.signIn(d.providerToken(pair, "k1", providerIssuer), nil).AssertCreated()
		d.tc.Clock.Travel(25 * time.Hour)
		d.signIn(d.providerToken(pair, "k1", providerIssuer), nil).AssertUnauthorized().
			AssertJSONPath("code", "token_jwks_unavailable")
		unreachable := "The key set address could not be reached or did not answer with a key set."
		d.owner.GetJSON("/api/v1/identity-secrets/"+created.String("data.id")).AssertOK().
			AssertJSONPath("data.jwks_error", unreachable)
	})

	t.Run("refreshes copies older than six hours on the hourly round", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		pair := testkit.P256(t)
		d.publish(map[string]testkit.KeyPair{"k1": pair})
		created := d.owner.PostJSON("/api/v1/identity-secrets", map[string]any{
			"algorithm": "ES256", "jwks_url": providerJWKS,
		}).AssertCreated()
		before := created.String("data.jwks_fetched_at")
		d.tc.Clock.Travel(7 * time.Hour)
		_, err := d.tc.App.Actions.IdentityKeySets.RefreshDue(t.Context(), 1000)
		require.NoError(t, err)
		after := d.owner.GetJSON("/api/v1/identity-secrets/" + created.String("data.id")).AssertOK().
			String("data.jwks_fetched_at")
		assert.NotEqual(t, before, after)
	})
}
