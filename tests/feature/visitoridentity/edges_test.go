package visitoridentity_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/hashing"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
handmade signs a token from raw header and claims, for shapes a JWT
library refuses to produce.
*/
func handmade(s secret, header, claims map[string]any) string {
	part := func(v map[string]any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	unsigned := part(header) + "." + part(claims)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(hashing.HMAC([]byte(s.value), []byte(unsigned)))
}

func TestIdentityEdges(t *testing.T) {
	t.Parallel()

	t.Run("explains every malformed or mismatched token", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		now := d.tc.Clock.Now()
		claims := map[string]any{"sub": "u", "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix()}
		named := map[string]any{"alg": "HS256", "kid": s.id}
		late := map[string]any{"sub": "u", "nbf": now.Add(time.Hour).Unix(), "exp": now.Add(2 * time.Hour).Unix()}
		cases := map[string]string{
			handmade(s, map[string]any{"alg": "none", "kid": s.id}, claims):              "token_alg_not_allowed",
			handmade(s, map[string]any{"alg": "RS256", "kid": s.id}, claims):             "token_alg_mismatch",
			handmade(s, map[string]any{"alg": "RS256"}, claims):                          "token_kid_required",
			handmade(s, named, map[string]any{"sub": 42, "exp": claims["exp"]}):          "token_sub_invalid",
			handmade(s, named, map[string]any{"sub": "u"}):                               "token_exp_missing",
			handmade(s, named, late):                                                     "token_not_yet_valid",
			s.sign(t, now, map[string]any{"sub": strings.Repeat("x", 300)}):              "token_sub_invalid",
			s.sign(t, now, map[string]any{"sub": "u", "pad": strings.Repeat("x", 9000)}): "token_too_large",
			"bad.payload.!!": "token_malformed",
		}
		for token, code := range cases {
			res := d.signIn(token, nil).AssertUnauthorized().AssertJSONPath("code", code)
			assert.NotEmpty(t, res.String("message"), code)
		}
		d.signIn(s.sign(t, now, map[string]any{"sub": "u", "exp": now.Add(-3 * time.Hour)}), nil).
			AssertUnauthorized().AssertJSONPath("code", "token_expired").
			AssertJSONPath("hint", "It expired 3 hours ago. Mint a token per request from your identity endpoint; "+
				"never cache it or put it in your HTML.")
		d.signIn(s.sign(t, now, map[string]any{"sub": "u", "exp": now.Add(-50 * time.Hour)}), nil).
			AssertJSONPath("code", "token_expired")
		d.signIn(s.sign(t, now, map[string]any{"sub": "u", "exp": now.Add(-90 * time.Second)}), nil).
			AssertJSONPath("code", "token_expired")
		d.signIn(s.sign(t, now, map[string]any{"sub": "u", "nonce": strings.Repeat("n", 200)}), nil).
			AssertUnauthorized().AssertJSONPath("code", "token_malformed")
	})

	t.Run("keeps the claims that are right and drops the rest", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		res := d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub": "user_1", "phone": "+14155550100", "locale": "fr-fr", "timezone": "Europe/Paris",
			"avatar_url": "https://cdn.acme.test/a.png", "name": strings.Repeat("n", 200),
			"company": map[string]any{"name": "No id"},
			"custom_attributes": map[string]any{
				"plan": "pro", "Bad Key": 1, "nested": map[string]any{"a": 1}, "long": strings.Repeat("l", 2000),
				"ratio": 0.5, "beta": true,
			},
		}), nil).AssertCreated()
		c := d.contact(res.String("data.contact.id"))
		assert.Equal(t, "+14155550100", *c.Phone)
		assert.Equal(t, "fr-FR", *c.Locale)
		assert.Equal(t, "Europe/Paris", *c.Timezone)
		assert.Equal(t, "https://cdn.acme.test/a.png", *c.AvatarUrl)
		assert.Nil(t, c.Name)
		assert.Nil(t, c.CompanyID)
		assert.ElementsMatch(t, []string{"beta", "plan", "ratio"}, c.TrustedAttributeKeys)

		res = d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub": "user_2", "phone": "555", "locale": "not a locale!", "timezone": "Mars/Base",
			"company": "Acme", "custom_attributes": []any{1}, "email": 7,
		}), nil).AssertCreated()
		c = d.contact(res.String("data.contact.id"))
		assert.Nil(t, c.Phone)
		assert.Nil(t, c.Locale)
		assert.Nil(t, c.Timezone)
	})

	t.Run("writes a later token's changes and removes attributes sent as null", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub": "user_1", "email": "old@acme.test", "custom_attributes": map[string]any{"plan": "pro", "seats": 2},
			"company": map[string]any{"id": "org_1", "name": "Acme"},
		}), nil)
		v.client().PatchJSON("/widget/v1/session", testkit.JSON{
			"user_jwt": s.sign(t, d.tc.Clock.Now(), map[string]any{
				"sub": "user_1", "email": "old@acme.test", "email_verified": true,
				"custom_attributes": map[string]any{"plan": nil}, "company": map[string]any{"id": "org_2"},
			}),
		}).AssertOK()
		c := d.contact(v.contact)
		assert.True(t, c.EmailVerified)
		assert.Equal(t, []string{"seats"}, c.TrustedAttributeKeys)
		assert.NotContains(t, string(c.CustomAttributes), "plan")

		v.client().PatchJSON("/widget/v1/session", testkit.JSON{
			"user_jwt": s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1", "email": "new@acme.test"}),
		}).AssertOK()
		c = d.contact(v.contact)
		assert.Equal(t, "new@acme.test", *c.Email)
		assert.False(t, c.EmailVerified)
		v.client().PatchJSON("/widget/v1/session", testkit.JSON{"user_jwt": "nope"}).AssertUnauthorized()
	})

	t.Run("refuses an anonymous session's refresh", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		lead := d.anonymous()
		lead.client().PatchJSON("/widget/v1/session", testkit.JSON{
			"user_jwt": s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}),
		}).AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "identity_mismatch")
	})

	t.Run("ignores a previous token that is unknown or from another messenger", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		ghost := &visitor{d: d, token: "dmi_ws_" + strings.Repeat("A", 43)}
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), ghost)
		assert.NotEmpty(t, v.token)

		other := newDesk(t)
		foreign := other.anonymous()
		d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), foreign).AssertCreated()
		foreign.client().GetJSON("/widget/v1/session").AssertOK()
	})

	t.Run("checks names, statuses and dates of secret changes", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		path := "/api/v1/identity-secrets/" + s.id
		d.owner.PatchJSON(path, testkit.JSON{"name": "   "}).AssertStatus(fhttp.StatusUnprocessableEntity).
			AssertJSONPath("code", "name_invalid")
		d.owner.PatchJSON(path, testkit.JSON{"retires_at": d.tc.Clock.Now().Add(time.Hour)}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "retires_at_without_status")
		d.owner.PatchJSON(path, testkit.JSON{"status": "active"}).AssertStatus(fhttp.StatusUnprocessableEntity).
			AssertJSONPath("code", "invalid_transition")
		d.owner.PatchJSON(path, testkit.JSON{"name": "Renamed"}).AssertOK().AssertJSONPath("data.name", "Renamed")
		d.owner.PatchJSON(path, testkit.JSON{"status": "retiring"}).AssertOK().AssertJSONPath("data.status", "retiring")
		d.owner.GetJSON("/api/v1/identity-secrets/isk_01jbd9w4c2xxxxxxxxxxxxxxxx").AssertNotFound()
		d.owner.GetJSON("/api/v1/identity-secrets/nonsense").AssertNotFound()
		long := testkit.JSON{"name": strings.Repeat("n", 61), "algorithm": "HS256"}
		d.owner.PostJSON("/api/v1/identity-secrets", long).AssertStatus(fhttp.StatusUnprocessableEntity)
		d.owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{"algorithm": "RS256", "public_key": "junk"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "public_key_invalid")
		pair := testkit.Ed25519(t)
		d.owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{"algorithm": "RS256", "public_key": pair.PublicPEM}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "public_key_invalid")
		created := d.owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{
			"algorithm": "EdDSA", "public_key": pair.PublicPEM,
		}).AssertCreated()
		d.owner.PostJSON("/api/v1/identity-secrets/"+created.String("data.id")+"/rotations", testkit.JSON{
			"public_key": testkit.Ed25519(t).PublicPEM,
		}).AssertCreated().AssertJSONPath("data.algorithm", "EdDSA")
		require.NotEmpty(t, created.String("data.fingerprint"))
	})
}
