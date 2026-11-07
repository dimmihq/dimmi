package visitoridentity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestIdentitySecrets(t *testing.T) {
	t.Parallel()

	t.Run("shows a new shared secret once and never lists it", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		res := d.owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{"name": "Production", "algorithm": "HS256"}).
			AssertCreated().AssertJSONPath("data.status", "active").AssertJSONPath("meta.secret_shown_once", true).
			AssertJSONPath("meta.env_var", "DIMMI_IDENTITY_SECRET")
		value, id := res.String("data.secret"), res.String("data.id")
		assert.True(t, strings.HasPrefix(value, "dmi_idsec_"))
		assert.Len(t, value, len("dmi_idsec_")+43)
		assert.Equal(t, id, res.String("data.kid"))
		assert.Equal(t, value[len(value)-4:], res.String("data.fingerprint"))

		list := d.owner.GetJSON("/api/v1/identity-secrets").AssertOK()
		list.AssertJSONPath("data.0.id", id).AssertJSONMissing("data.0.secret")
		d.owner.GetJSON("/api/v1/identity-secrets/" + id).AssertOK().AssertJSONMissing("data.secret")
	})

	t.Run("lets members read but only admins change secrets", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		user := d.tc.CreateUser()
		_, err := factories.Member(t.Context(), d.tc.App, factories.Membership{
			Workspace: d.tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)
		member := d.tc.In(user)
		member.GetJSON("/api/v1/identity-secrets").AssertOK()
		member.PostJSON("/api/v1/identity-secrets", testkit.JSON{"algorithm": "HS256"}).AssertForbidden()
	})

	t.Run("keeps at most five secrets at once", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		for range 5 {
			d.newSecret()
		}
		d.owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{"algorithm": "HS256"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "identity_secret_limit")
	})

	t.Run("rotates to a new secret while the old one keeps working for a week", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		old := d.newSecret()
		res := d.owner.PostJSON("/api/v1/identity-secrets/"+old.id+"/rotations", testkit.JSON{}).AssertCreated()
		fresh := secret{id: res.String("data.id"), value: res.String("data.secret")}
		assert.NotEqual(t, old.value, fresh.value)
		d.owner.GetJSON("/api/v1/identity-secrets/"+old.id).AssertOK().AssertJSONPath("data.status", "retiring")

		now := d.tc.Clock.Now()
		d.signIn(old.sign(t, now, map[string]any{"sub": "user_1"}), nil).AssertCreated()
		d.signIn(fresh.sign(t, now, map[string]any{"sub": "user_1"}), nil).AssertCreated()

		d.tc.Clock.Advance(8 * 24 * time.Hour)
		later := d.tc.Clock.Now()
		d.signIn(old.sign(t, later, map[string]any{"sub": "user_1"}), nil).AssertUnauthorized().
			AssertJSONPath("code", "token_secret_revoked")
		d.signIn(fresh.sign(t, later, map[string]any{"sub": "user_1"}), nil).AssertCreated()
		d.owner.PostJSON("/api/v1/identity-secrets/"+old.id+"/rotations", testkit.JSON{}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "secret_not_active")
	})

	t.Run("signs out every visitor a revoked secret signed in", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), nil)
		v.client().GetJSON("/widget/v1/conversations").AssertOK()

		d.owner.PatchJSON("/api/v1/identity-secrets/"+s.id, testkit.JSON{"status": "revoked"}).AssertOK().
			AssertJSONPath("data.status", "revoked")
		v.client().GetJSON("/widget/v1/conversations").AssertUnauthorized().AssertJSONPath("code", "session_revoked")
		assert.Equal(t, "secret_revoked", *d.session(v.id).RevokeReason)
		d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), nil).AssertUnauthorized().
			AssertJSONPath("code", "identity_not_configured")
		d.newSecret()
		d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), nil).AssertUnauthorized().
			AssertJSONPath("code", "token_secret_revoked")
		d.owner.PatchJSON("/api/v1/identity-secrets/"+s.id, testkit.JSON{"status": "retiring"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "invalid_transition")
	})

	t.Run("keeps signed-in visitors when asked while revoking", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		v := d.signedIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), nil)
		d.owner.PatchJSON("/api/v1/identity-secrets/"+s.id, testkit.JSON{
			"status": "revoked", "reason": "compromised", "revoke_sessions": false,
		}).AssertOK()
		v.client().GetJSON("/widget/v1/conversations").AssertOK()
	})

	t.Run("renames and retires a secret on a chosen date", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		at := d.tc.Clock.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
		d.owner.PatchJSON("/api/v1/identity-secrets/"+s.id, testkit.JSON{
			"name": "Old servers", "status": "retiring", "retires_at": at,
		}).AssertOK().AssertJSONPath("data.name", "Old servers").AssertJSONPath("data.status", "retiring")
		d.owner.PatchJSON("/api/v1/identity-secrets/"+s.id, testkit.JSON{
			"status": "retiring", "retires_at": d.tc.Clock.Now().Add(60 * 24 * time.Hour),
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "retires_at_invalid")
	})

	t.Run("verifies EdDSA tokens with an uploaded public key and refuses private keys", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		pair := testkit.Ed25519(t)
		res := d.owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{
			"name": "Signer", "algorithm": "EdDSA", "public_key": pair.PublicPEM,
		}).AssertCreated().AssertJSONMissing("data.secret")
		kid := res.String("data.kid")

		key, err := jwt.NewKey(kid, jwt.EdDSA, pair.Private)
		require.NoError(t, err)
		now := d.tc.Clock.Now()
		token, err := jwt.Sign(jwt.Claims{Subject: "user_9", IssuedAt: now, Expiry: now.Add(5 * time.Minute)}, key)
		require.NoError(t, err)
		d.signIn(token, nil).AssertCreated().AssertJSONPath("data.kind", "identified")

		d.owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{
			"algorithm": "EdDSA", "public_key": pair.PrivatePEM,
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "private_key_uploaded")
	})
}
