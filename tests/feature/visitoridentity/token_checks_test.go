package visitoridentity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIdentityTokenChecks(t *testing.T) {
	t.Parallel()

	t.Run("refuses a token when no secret exists", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := secret{id: "isk_none", value: "dmi_idsec_0123456789012345678901234567890123456789012"}
		d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "u"}), nil).AssertUnauthorized().
			AssertJSONPath("code", "identity_not_configured")
	})

	t.Run("refuses bad tokens with a plain hint and changes nothing", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		lead := d.anonymous()
		now := d.tc.Clock.Now()
		wrong := secret{id: s.id, value: "dmi_idsec_wrongwrongwrongwrongwrongwrongwrongwrong123"}
		cases := map[string]string{
			"not a jwt": "token_malformed",
			wrong.sign(t, now, map[string]any{"sub": "u"}):                                  "token_signature_invalid",
			s.sign(t, now, map[string]any{}):                                                "token_sub_missing",
			s.sign(t, now, map[string]any{"sub": "u", "exp": now.Add(-10 * time.Minute)}):   "token_expired",
			s.sign(t, now, map[string]any{"sub": "u", "exp": now.Add(30 * 24 * time.Hour)}): "token_lifetime_too_long",
			s.signAs(t, "isk_01jbd9w4c2xxxxxxxxxxxxxxxx", now, map[string]any{"sub": "u"}):  "token_kid_unknown",
			s.sign(t, now, map[string]any{"sub": "u", "aud": "someone-else"}):               "token_audience_mismatch",
		}
		for token, code := range cases {
			res := d.signIn(token, lead).AssertUnauthorized().AssertJSONPath("code", code)
			assert.NotEmpty(t, res.String("hint"), code)
		}
		lead.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.kind", "anonymous")
		assert.Nil(t, d.session(lead.id).RevokedAt)
	})

	t.Run("tries each shared secret when the token names no key", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.newSecret()
		s := d.newSecret()
		d.signIn(s.signAs(t, "", d.tc.Clock.Now(), map[string]any{"sub": "user_1"}), nil).AssertCreated()
	})

	t.Run("accepts a nonce only once", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		token := s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "user_1", "nonce": "n-1"})
		d.signIn(token, nil).AssertCreated()
		d.signIn(token, nil).AssertUnauthorized().AssertJSONPath("code", "token_replayed")
	})

	t.Run("accepts the messenger's own audience", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "u", "aud": d.app}), nil).AssertCreated()
		d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "u", "aud": "dimmi"}), nil).AssertCreated()
	})

	t.Run("drops a malformed claim but still signs the visitor in", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		s := d.newSecret()
		res := d.signIn(s.sign(t, d.tc.Clock.Now(), map[string]any{
			"sub": "user_1", "email": "not-an-email", "avatar_url": "http://insecure.test/a.png", "name": "Ok",
		}), nil).AssertCreated()
		c := d.contact(res.String("data.contact.id"))
		assert.Nil(t, c.Email)
		assert.Nil(t, c.AvatarUrl)
		assert.Equal(t, "Ok", *c.Name)
	})

	t.Run("refuses a token signed for another workspace", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		other := newDesk(t)
		d.newSecret()
		foreign := other.newSecret()
		d.signIn(foreign.sign(t, d.tc.Clock.Now(), map[string]any{"sub": "u"}), nil).AssertUnauthorized().
			AssertJSONPath("code", "token_kid_unknown")
		d.signIn(foreign.signAs(t, "", d.tc.Clock.Now(), map[string]any{"sub": "u"}), nil).AssertUnauthorized().
			AssertJSONPath("code", "token_signature_invalid")
	})
}
