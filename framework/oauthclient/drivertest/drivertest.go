/*
|--------------------------------------------------------------------------
| Provider Contract
|--------------------------------------------------------------------------
|
| Every provider must run the same flow, or adding one would weaken
| every connection. A driver's tests call Run with a constructor
| returning a driver and a way to approve an authorization URL, from a
| local server playing the provider or from the fake. The suite checks
| that the authorization URL carries the attempt and never the verifier,
| that an approval yields a token, that codes are bound to their
| redirect URL and work only once, and that refresh and revocation
| either work or say they are unsupported.
|
|   func TestDriver(t *testing.T) {
|       drivertest.Run(t, newHarness)
|   }
|
*/

package drivertest

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
RedirectURL is where attempts made by the suite send the browser back.
*/
const RedirectURL = "https://app.example.com/connections/callback"

/*
Harness is a driver under test. Approve plays the person approving the
authorization URL and returns the callback URL the browser is sent to.
Refreshes says whether the driver's provider issues refresh tokens, and
Revokes whether it takes tokens back.
*/
type Harness struct {
	Driver    oauthclient.Driver
	Approve   func(t *testing.T, authURL string) string
	Refreshes bool
	Revokes   bool
}

/*
NewHarness returns a driver no other test uses.
*/
type NewHarness func(t *testing.T) Harness

/*
NewAttempt returns a fresh attempt sending the browser back to
RedirectURL and asking for the scopes.
*/
func NewAttempt(scopes ...string) oauthclient.Attempt {
	return oauthclient.Attempt{
		State: str.Random(43), Verifier: oauth2.GenerateVerifier(), RedirectURL: RedirectURL, Scopes: scopes,
	}
}

/*
Run runs the contract against drivers from the constructor.
*/
func Run(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("authorization url", func(t *testing.T) { t.Parallel(); runAuthURL(t, newHarness) })
	t.Run("codes", func(t *testing.T) { t.Parallel(); runCodes(t, newHarness) })
	t.Run("refresh", func(t *testing.T) { t.Parallel(); runRefresh(t, newHarness) })
	t.Run("revoke", func(t *testing.T) { t.Parallel(); runRevoke(t, newHarness) })
}

/*
Approve approves a fresh attempt and returns it with the code the
callback carries.
*/
func Approve(t *testing.T, h Harness) (oauthclient.Attempt, string) {
	t.Helper()

	attempt := NewAttempt("read")
	authURL, err := h.Driver.AuthURL(t.Context(), attempt)
	require.NoError(t, err)

	callback, err := url.Parse(h.Approve(t, authURL))
	require.NoError(t, err)

	query := callback.Query()
	callback.RawQuery = ""

	require.Equal(t, attempt.RedirectURL, callback.String())
	require.Equal(t, attempt.State, query.Get("state"))
	require.NotEmpty(t, query.Get("code"))

	return attempt, query.Get("code")
}

/*
Connect approves a fresh attempt and exchanges its code.
*/
func Connect(t *testing.T, h Harness) oauthclient.Grant {
	t.Helper()

	attempt, code := Approve(t, h)
	grant, err := h.Driver.Exchange(t.Context(), code, attempt)
	require.NoError(t, err)

	return grant
}

func runAuthURL(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("carries the attempt but never the verifier", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		attempt := NewAttempt("read")

		authURL, err := h.Driver.AuthURL(t.Context(), attempt)
		require.NoError(t, err)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		query := parsed.Query()

		assert.NotEmpty(t, query.Get("client_id"))
		assert.Equal(t, attempt.State, query.Get("state"))
		assert.Equal(t, attempt.RedirectURL, query.Get("redirect_uri"))
		assert.Contains(t, query.Get("scope"), "read")
		assert.NotContains(t, authURL, attempt.Verifier)

		if challenge := query.Get("code_challenge"); challenge != "" {
			assert.Equal(t, "S256", query.Get("code_challenge_method"))
			assert.Equal(t, oauth2.S256ChallengeFromVerifier(attempt.Verifier), challenge)
		}
	})
}

func runCodes(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("trades an approved code for a token", func(t *testing.T) {
		t.Parallel()
		grant := Connect(t, newHarness(t))

		assert.True(t, grant.Access != nil || grant.UserAccess != nil)
	})

	t.Run("refuses a code exchanged for another redirect url", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		attempt, code := Approve(t, h)
		attempt.RedirectURL = "https://evil.example.com/callback"

		_, err := h.Driver.Exchange(t.Context(), code, attempt)
		require.ErrorIs(t, err, oauthclient.ErrExchange)
	})

	t.Run("refuses a code used twice", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		attempt, code := Approve(t, h)

		_, err := h.Driver.Exchange(t.Context(), code, attempt)
		require.NoError(t, err)

		_, err = h.Driver.Exchange(t.Context(), code, attempt)
		require.ErrorIs(t, err, oauthclient.ErrExchange)
	})

	t.Run("refuses a code it never issued", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.Driver.Exchange(t.Context(), "made-up", NewAttempt())
		require.ErrorIs(t, err, oauthclient.ErrExchange)
	})
}

func runRefresh(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("renews a refresh token it issued", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		if !h.Refreshes {
			t.Skip("the provider issues no refresh tokens")
		}
		grant := Connect(t, h)
		require.NotNil(t, grant.Refresh)

		renewed, err := h.Driver.Refresh(t.Context(), grant.Refresh.Value)
		require.NoError(t, err)
		require.NotNil(t, renewed.Access)
		assert.NotEqual(t, grant.Access.Value, renewed.Access.Value)
	})

	t.Run("refuses a refresh token it never issued", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.Driver.Refresh(t.Context(), "made-up")
		require.ErrorIs(t, err, oauthclient.ErrInvalidGrant)
	})
}

func runRevoke(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("gives a token back or says it cannot", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		grant := Connect(t, h)
		access := grant.Access
		if access == nil {
			access = grant.UserAccess
		}

		err := h.Driver.Revoke(t.Context(), access.Value)
		if h.Revokes {
			require.NoError(t, err)

			return
		}

		require.ErrorIs(t, err, oauthclient.ErrUnsupported)
	})
}
