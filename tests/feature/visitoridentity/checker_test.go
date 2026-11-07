package visitoridentity_test

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
check pastes a token in the dashboard's token checker.
*/
func (d *desk) check(token string) *testkit.Response {
	d.t.Helper()
	return d.owner.PostJSON("/api/v1/identity-verification/validations", testkit.JSON{"token": token}).AssertOK()
}

/*
signRaw signs claims as given, with the key bytes given, so tests can
make the mistakes developers make.
*/
func signRaw(t *testing.T, key []byte, kid string, claims jwt.Claims) string {
	t.Helper()
	k, err := jwt.NewKey(kid, jwt.HS256, key)
	require.NoError(t, err)
	token, err := jwt.Sign(claims, k)
	require.NoError(t, err)
	return token
}

func TestTokenChecker(t *testing.T) {
	t.Parallel()

	t.Run("explains a valid token without using it up", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		token := s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1", "nonce": "n-1", "email": "a@b.test"})
		res := d.check(token)
		res.AssertJSONPath("data.valid", true).AssertJSONPath("data.contact_preview.action", "create").
			AssertJSONPath("data.claims.sub", "user_1").AssertJSONPath("data.header.kid", s.id).
			AssertJSONPath("data.warnings.0.code", "email_not_flagged")
		assert.NotContains(t, string(res.Body), token[len(token)-20:])
		v := d.signedIn(token, nil)
		d.check(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"})).
			AssertJSONPath("data.contact_preview.action", "reuse").
			AssertJSONPath("data.contact_preview.contact_id", v.contact)
	})

	t.Run("names the slip behind a failed token", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		now := d.tc.Clock.Now()
		claims := jwt.Claims{Subject: "user_1", IssuedAt: now, Expiry: now.Add(time.Minute)}
		decoded, err := base64.RawURLEncoding.DecodeString(s.value[len("dmi_idsec_"):])
		require.NoError(t, err)
		cases := map[string]string{
			signRaw(t, []byte(s.value+"\n"), s.id, claims):                "secret_trailing_newline",
			signRaw(t, []byte(`"`+s.value+`"`), s.id, claims):             "secret_quoted",
			signRaw(t, []byte(s.value[len("dmi_idsec_"):]), s.id, claims): "secret_prefix_dropped",
			signRaw(t, decoded, s.id, claims):                             "secret_base64_decoded",
			"Bearer " + s.sign(t, now, map[string]any{"sub": "u"}):        "bearer_prefix",
			s.sign(t, now, map[string]any{"user_id": "42"}):               "user_id_alias",
		}
		for token, mistake := range cases {
			d.check(token).AssertJSONPath("data.valid", false).AssertJSONPath("data.mistake.code", mistake)
		}
	})

	t.Run("says a key belongs to another workspace without naming it", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		other, err := factories.Workspace(t.Context(), d.tc.App, d.tc.Owner)
		require.NoError(t, err)
		elsewhere := d.tc.ActingAs(d.tc.Owner).
			WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, other.Workspace.ID))
		res := elsewhere.PostJSON("/api/v1/identity-secrets", testkit.JSON{"algorithm": "HS256"}).AssertCreated()
		theirs := secret{id: res.String("data.id"), value: res.String("data.secret")}
		d.newSecret()
		d.check(theirs.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "u"})).
			AssertJSONPath("data.code", "token_kid_unknown").AssertJSONPath("data.mistake.code", "other_workspace_key")
	})
}
