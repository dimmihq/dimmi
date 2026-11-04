package webauthn_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/webauthn"
	"github.com/dimmihq/dimmi/framework/auth/webauthn/webauthntest"
	"github.com/dimmihq/dimmi/framework/clock"
)

const origin = "https://example.com"

/*
user is the account every test registers credentials for.
*/
func user() webauthn.User {
	return webauthn.User{ID: []byte("user-1"), Name: "ada@example.com", DisplayName: "Ada"}
}

/*
epoch is when the fake clocks start.
*/
func epoch() time.Time {
	return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
}

/*
party returns a relying party for example.com on the fake clock.
*/
func party(t *testing.T, clk clock.Clock) *webauthn.RelyingParty {
	t.Helper()

	rp, err := webauthn.New(webauthn.Config{ID: "example.com", Name: "Example", Origins: []string{origin}, Clock: clk})
	require.NoError(t, err)

	return rp
}

/*
register registers the authenticator with the relying party and returns
the credential.
*/
func register(t *testing.T, rp *webauthn.RelyingParty, a *webauthntest.Authenticator) webauthn.Credential {
	t.Helper()

	c, err := rp.BeginRegistration(user(), nil)
	require.NoError(t, err)

	answer, err := a.Create(c.Options)
	require.NoError(t, err)

	cred, err := rp.FinishRegistration(user(), c.State, answer)
	require.NoError(t, err)

	return cred
}

/*
login answers a new sign-in ceremony for the credential with the
authenticator.
*/
func login(
	t *testing.T, rp *webauthn.RelyingParty, a *webauthntest.Authenticator, cred webauthn.Credential,
) (webauthn.Credential, error) {
	t.Helper()

	c, err := rp.BeginLogin(user(), []webauthn.Credential{cred})
	require.NoError(t, err)

	answer, err := a.Get(c.Options)
	require.NoError(t, err)

	return rp.FinishLogin(user(), cred, c.State, answer)
}

func TestRelyingParty(t *testing.T) {
	t.Parallel()

	user := user()

	t.Run("refuses a configuration that cannot work", func(t *testing.T) {
		t.Parallel()

		for _, cfg := range []webauthn.Config{
			{},
			{ID: "127.0.0.1", Name: "Example", Origins: []string{origin}},
			{ID: "example.com", Name: "Example"},
		} {
			_, err := webauthn.New(cfg)
			require.ErrorIs(t, err, webauthn.ErrInvalidConfig)
		}
	})

	t.Run("registers a credential and signs in with it", func(t *testing.T) {
		t.Parallel()

		rp := party(t, clock.NewFake(epoch()))
		a := webauthntest.New(origin)
		cred := register(t, rp, a)

		assert.Equal(t, a.ID(), cred.ID)
		assert.Equal(t, []string{"internal"}, cred.Transports)
		assert.True(t, cred.UserVerified)
		assert.Equal(t, "platform", cred.Attachment)

		after, err := login(t, rp, a, cred)
		require.NoError(t, err)
		assert.Equal(t, uint32(1), after.SignCount)

		after, err = login(t, rp, a, after)
		require.NoError(t, err)
		assert.Equal(t, uint32(2), after.SignCount)
	})

	t.Run("hands out options a browser understands", func(t *testing.T) {
		t.Parallel()

		rp := party(t, nil)
		existing := webauthn.Credential{ID: []byte("old"), Transports: []string{"usb"}}

		c, err := rp.BeginRegistration(user, []webauthn.Credential{existing})
		require.NoError(t, err)

		var opts map[string]any
		require.NoError(t, json.Unmarshal(c.Options, &opts))
		assert.NotEmpty(t, opts["challenge"])
		assert.Equal(t, map[string]any{"id": "example.com", "name": "Example"}, opts["rp"])
		assert.Equal(t, []any{map[string]any{"id": "b2xk", "type": "public-key", "transports": []any{"usb"}}},
			opts["excludeCredentials"])
		assert.Equal(t, map[string]any{
			"userVerification": "preferred", "residentKey": "preferred", "requireResidentKey": false,
		}, opts["authenticatorSelection"])
	})

	t.Run("refuses an answer to another challenge", func(t *testing.T) {
		t.Parallel()

		rp := party(t, nil)
		a := webauthntest.New(origin)
		cred := register(t, rp, a)

		first, err := rp.BeginLogin(user, []webauthn.Credential{cred})
		require.NoError(t, err)

		second, err := rp.BeginLogin(user, []webauthn.Credential{cred})
		require.NoError(t, err)

		answer, err := a.Get(first.Options)
		require.NoError(t, err)

		_, err = rp.FinishLogin(user, cred, second.State, answer)
		require.ErrorIs(t, err, webauthn.ErrInvalidResponse)
	})

	t.Run("refuses a registration from another origin", func(t *testing.T) {
		t.Parallel()

		rp := party(t, nil)
		c, err := rp.BeginRegistration(user, nil)
		require.NoError(t, err)

		answer, err := webauthntest.New("https://evil.example").Create(c.Options)
		require.NoError(t, err)

		_, err = rp.FinishRegistration(user, c.State, answer)
		require.ErrorIs(t, err, webauthn.ErrInvalidResponse)
	})

	t.Run("refuses a cloned key whose counter falls behind", func(t *testing.T) {
		t.Parallel()

		rp := party(t, nil)
		a := webauthntest.New(origin)
		cred := register(t, rp, a)
		clone := a.Clone()

		after, err := login(t, rp, a, cred)
		require.NoError(t, err)

		_, err = login(t, rp, clone, after)
		require.ErrorIs(t, err, webauthn.ErrCloned)
	})

	t.Run("refuses an answer from another credential", func(t *testing.T) {
		t.Parallel()

		rp := party(t, nil)
		a, b := webauthntest.New(origin), webauthntest.New(origin)
		first, second := register(t, rp, a), register(t, rp, b)

		c, err := rp.BeginLogin(user, []webauthn.Credential{first, second})
		require.NoError(t, err)

		answer, err := b.Get(c.Options)
		require.NoError(t, err)

		_, err = rp.FinishLogin(user, first, c.State, answer)
		require.ErrorIs(t, err, webauthn.ErrUnknownCredential)

		after, err := rp.FinishLogin(user, second, c.State, answer)
		require.NoError(t, err)
		assert.Equal(t, b.ID(), after.ID)
	})

	t.Run("refuses a credential the ceremony did not allow", func(t *testing.T) {
		t.Parallel()

		rp := party(t, nil)
		a, b := webauthntest.New(origin), webauthntest.New(origin)
		first, second := register(t, rp, a), register(t, rp, b)

		c, err := rp.BeginLogin(user, []webauthn.Credential{first})
		require.NoError(t, err)

		answer, err := a.Get(c.Options)
		require.NoError(t, err)

		_, err = rp.FinishLogin(user, second, c.State, answer)
		require.ErrorIs(t, err, webauthn.ErrUnknownCredential)

		_, err = b.Get(c.Options)
		require.ErrorIs(t, err, webauthntest.ErrNoCredential)
	})

	t.Run("refuses state that ran out or cannot be read", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewFake(epoch())
		rp := party(t, clk)
		a := webauthntest.New(origin)

		c, err := rp.BeginRegistration(user, nil)
		require.NoError(t, err)

		answer, err := a.Create(c.Options)
		require.NoError(t, err)

		clk.Travel(webauthn.DefaultTTL)

		_, err = rp.FinishRegistration(user, c.State, answer)
		require.ErrorIs(t, err, webauthn.ErrExpired)

		_, err = rp.FinishLogin(user, webauthn.Credential{}, json.RawMessage(`"nope"`), answer)
		require.ErrorIs(t, err, webauthn.ErrExpired)
	})

	t.Run("refuses answers that are not credentials", func(t *testing.T) {
		t.Parallel()

		rp := party(t, nil)

		c, err := rp.BeginRegistration(user, nil)
		require.NoError(t, err)

		_, err = rp.FinishRegistration(user, c.State, json.RawMessage(`{}`))
		require.ErrorIs(t, err, webauthn.ErrInvalidResponse)

		_, err = rp.FinishLogin(user, webauthn.Credential{}, c.State, json.RawMessage(`{}`))
		require.ErrorIs(t, err, webauthn.ErrInvalidResponse)

		_, err = rp.FinishRegistration(webauthn.User{ID: []byte("other")}, c.State, json.RawMessage(`{}`))
		require.ErrorIs(t, err, webauthn.ErrInvalidResponse)
	})

	t.Run("cannot begin a sign-in without credentials", func(t *testing.T) {
		t.Parallel()

		_, err := party(t, nil).BeginLogin(user, nil)
		require.Error(t, err)
	})
}

func TestAuthenticator(t *testing.T) {
	t.Parallel()

	t.Run("refuses options it cannot read", func(t *testing.T) {
		t.Parallel()

		a := webauthntest.New(origin)

		_, err := a.Create(json.RawMessage(`[`))
		require.Error(t, err)

		_, err = a.Create(json.RawMessage(`{"user":{"id":"!"}}`))
		require.Error(t, err)

		_, err = a.Get(json.RawMessage(`[`))
		require.Error(t, err)
	})
}
