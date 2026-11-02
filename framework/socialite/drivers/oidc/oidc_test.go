package oidc_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivers/oidc"
	"github.com/dimmihq/dimmi/framework/socialite/drivertest"
)

func newClient(t *testing.T) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	return client
}

func newUser() socialite.User {
	return socialite.User{
		Provider:      "sso",
		ID:            "00u1abcd",
		Email:         "ada@example.com",
		EmailVerified: true,
		Name:          "Ada Lovelace",
		AvatarURL:     "https://example.com/ada.png",
	}
}

/*
newProvider returns a provider registered with the server, reading time
from the clock.
*/
func newProvider(t *testing.T, srv *drivertest.Server, clk clock.Clock, scopes ...string) *oidc.Provider {
	t.Helper()

	provider, err := oidc.New("sso", newClient(t), clk, oidc.Options{
		Issuer:       srv.URL,
		ClientID:     srv.ClientID,
		ClientSecret: srv.ClientSecret,
		Scopes:       scopes,
	})
	require.NoError(t, err)

	return provider
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	srv := drivertest.NewServer(t, newUser())

	return drivertest.Harness{Provider: newProvider(t, srv, clock.System()), User: srv.User, Approve: srv.Approve}
}

func TestProvider(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

/*
exchange signs in at the server and trades the code, changing the
attempt first.
*/
func exchange(t *testing.T, srv *drivertest.Server, p *oidc.Provider, change func(*socialite.Attempt)) error {
	t.Helper()

	attempt, code := drivertest.SignIn(t, drivertest.Harness{Provider: p, Approve: srv.Approve})
	change(&attempt)

	_, err := p.Exchange(t.Context(), code, attempt)

	return err
}

func keep(*socialite.Attempt) {}

func TestIDToken(t *testing.T) {
	t.Parallel()

	tampered := map[string]func(*jwt.Claims){
		"another client":   func(c *jwt.Claims) { c.Audience = []string{"someone-else"} },
		"another issuer":   func(c *jwt.Claims) { c.Issuer = "https://evil.example.com" },
		"another party":    func(c *jwt.Claims) { c.Extra["azp"] = "someone-else" },
		"several audience": func(c *jwt.Claims) { c.Audience = append(c.Audience, "someone-else") },
	}

	for name, tamper := range tampered {
		t.Run("refuses a token for "+name, func(t *testing.T) {
			t.Parallel()
			srv := drivertest.NewServer(t, newUser())
			srv.TamperIDToken(tamper)

			err := exchange(t, srv, newProvider(t, srv, clock.System()), keep)

			require.ErrorIs(t, err, socialite.ErrInvalidIDToken)
		})
	}

	t.Run("refuses a token for another attempt", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, newUser())
		p := newProvider(t, srv, clock.System())

		err := exchange(t, srv, p, func(a *socialite.Attempt) { a.Nonce = "another" })
		require.ErrorIs(t, err, socialite.ErrInvalidIDToken)

		err = exchange(t, srv, p, func(a *socialite.Attempt) { a.Nonce = "" })
		require.ErrorIs(t, err, socialite.ErrInvalidIDToken)
	})

	t.Run("refuses an expired token", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, newUser())
		later := clock.NewFake(clock.System().Now().Add(2 * time.Hour))

		err := exchange(t, srv, newProvider(t, srv, later), keep)

		require.ErrorIs(t, err, socialite.ErrInvalidIDToken)
	})

	t.Run("refuses a sign-in without an id token", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, newUser())

		err := exchange(t, srv, newProvider(t, srv, clock.System(), "email"), keep)

		require.ErrorIs(t, err, socialite.ErrInvalidIDToken)
	})

	t.Run("refuses a token without a subject", func(t *testing.T) {
		t.Parallel()
		p := newProvider(t, drivertest.NewServer(t, newUser()), clock.System())

		_, err := p.User(t.Context(), socialite.Token{Claims: map[string]any{"email": "ada@example.com"}})

		require.ErrorIs(t, err, socialite.ErrInvalidIDToken)
	})
}

func TestUser(t *testing.T) {
	t.Parallel()

	t.Run("trusts the address only as far as the provider vouches", func(t *testing.T) {
		t.Parallel()
		unverified := newUser()
		unverified.EmailVerified = false
		srv := drivertest.NewServer(t, unverified)
		p := newProvider(t, srv, clock.System())
		attempt, code := drivertest.SignIn(t, drivertest.Harness{Provider: p, Approve: srv.Approve})

		token, err := p.Exchange(t.Context(), code, attempt)
		require.NoError(t, err)
		user, err := p.User(t.Context(), token)
		require.NoError(t, err)

		assert.Equal(t, "ada@example.com", user.Email)
		assert.False(t, user.EmailVerified)
		assert.Equal(t, srv.URL, user.Raw["iss"])
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("builds from configuration", func(t *testing.T) {
		t.Parallel()
		srv := drivertest.NewServer(t, newUser())
		factory := oidc.Factory(newClient(t), clock.System())

		p, err := factory("sso", config.Section{
			"issuer": srv.URL, "client_id": srv.ClientID, "prompt": "login", "scopes": []any{"openid", "email"},
		})
		require.NoError(t, err)

		authURL, err := p.AuthURL(t.Context(), drivertest.NewAttempt())
		require.NoError(t, err)
		assert.Contains(t, authURL, "prompt=login")
		assert.Contains(t, authURL, "scope=openid+email&")

		_, err = factory("sso", config.Section{"issuer": map[string]any{"not": "a url"}})
		require.Error(t, err)
	})

	t.Run("needs an issuer and a client", func(t *testing.T) {
		t.Parallel()

		_, err := oidc.New("sso", newClient(t), clock.System(), oidc.Options{Issuer: "https://idp.example.com"})
		require.ErrorIs(t, err, socialite.ErrMisconfigured)

		_, err = oidc.New("sso", newClient(t), clock.System(), oidc.Options{ClientID: "c"})
		require.ErrorIs(t, err, socialite.ErrMisconfigured)
	})

	t.Run("reports an issuer it cannot discover", func(t *testing.T) {
		t.Parallel()
		p, err := oidc.New("sso", newClient(t), clock.System(), oidc.Options{Issuer: "http://127.0.0.1:1", ClientID: "c"})
		require.NoError(t, err)

		_, err = p.AuthURL(t.Context(), drivertest.NewAttempt())
		require.ErrorIs(t, err, socialite.ErrProvider)

		_, err = p.Exchange(t.Context(), "code", drivertest.NewAttempt())
		require.ErrorIs(t, err, socialite.ErrProvider)
	})
}
