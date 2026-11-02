/*
|--------------------------------------------------------------------------
| Provider Contract
|--------------------------------------------------------------------------
|
| Every provider must run the same flow, or adding one would weaken
| sign-in for all. A driver's tests call Run with a constructor
| returning a provider, the user its identity provider signs in, and a
| way to approve an authorization URL as that user, from a local server
| playing the identity provider or from the fake. The suite checks that
| the authorization URL carries the attempt, that a sign-in returns the
| user, and that codes are bound to their verifier and redirect URL and
| work only once.
|
|   func TestProvider(t *testing.T) {
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

	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
RedirectURL is where attempts made by the suite send users back.
*/
const RedirectURL = "https://app.example.com/auth/callback"

/*
Harness is a provider under test. User is the user its identity
provider signs in, as the provider must report it; Raw and Token are
not compared. Approve plays that user approving the authorization URL
and returns the callback URL the browser is sent to.
*/
type Harness struct {
	Provider socialite.Provider
	User     socialite.User
	Approve  func(t *testing.T, authURL string) string
}

/*
NewHarness returns a provider no other test uses.
*/
type NewHarness func(t *testing.T) Harness

/*
NewAttempt returns a fresh attempt sending users back to RedirectURL.
*/
func NewAttempt() socialite.Attempt {
	return socialite.Attempt{
		State:       str.Random(43),
		Verifier:    oauth2.GenerateVerifier(),
		Nonce:       str.Random(43),
		RedirectURL: RedirectURL,
	}
}

/*
Run runs the contract against providers from the constructor.
*/
func Run(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("authorization url", func(t *testing.T) { t.Parallel(); runAuthURL(t, newHarness) })
	t.Run("sign in", func(t *testing.T) { t.Parallel(); runSignIn(t, newHarness) })
	t.Run("codes", func(t *testing.T) { t.Parallel(); runCodes(t, newHarness) })
}

/*
SignIn approves a fresh attempt and returns it with the code the
callback carries.
*/
func SignIn(t *testing.T, h Harness) (socialite.Attempt, string) {
	t.Helper()

	attempt := NewAttempt()
	authURL, err := h.Provider.AuthURL(t.Context(), attempt)
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

func runAuthURL(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("carries the attempt but never the verifier", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		attempt := NewAttempt()

		authURL, err := h.Provider.AuthURL(t.Context(), attempt)
		require.NoError(t, err)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		query := parsed.Query()

		assert.NotEmpty(t, query.Get("client_id"))
		assert.Equal(t, "code", query.Get("response_type"))
		assert.Equal(t, attempt.State, query.Get("state"))
		assert.Equal(t, attempt.RedirectURL, query.Get("redirect_uri"))
		assert.Equal(t, "S256", query.Get("code_challenge_method"))
		assert.Equal(t, oauth2.S256ChallengeFromVerifier(attempt.Verifier), query.Get("code_challenge"))
		assert.NotContains(t, authURL, attempt.Verifier)
	})
}

func runSignIn(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("returns the user the provider signed in", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		attempt, code := SignIn(t, h)

		token, err := h.Provider.Exchange(t.Context(), code, attempt)
		require.NoError(t, err)
		assert.NotEmpty(t, token.AccessToken)

		user, err := h.Provider.User(t.Context(), token)
		require.NoError(t, err)

		user.Raw, user.Token = nil, socialite.Token{}
		want := h.User
		want.Raw, want.Token = nil, socialite.Token{}
		assert.Equal(t, want, user)
	})
}

func runCodes(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("refuses a code exchanged with another verifier", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		attempt, code := SignIn(t, h)
		attempt.Verifier = oauth2.GenerateVerifier()

		_, err := h.Provider.Exchange(t.Context(), code, attempt)
		require.ErrorIs(t, err, socialite.ErrExchange)
	})

	t.Run("refuses a code exchanged for another redirect url", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		attempt, code := SignIn(t, h)
		attempt.RedirectURL = "https://evil.example.com/callback"

		_, err := h.Provider.Exchange(t.Context(), code, attempt)
		require.ErrorIs(t, err, socialite.ErrExchange)
	})

	t.Run("refuses a code used twice", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		attempt, code := SignIn(t, h)

		_, err := h.Provider.Exchange(t.Context(), code, attempt)
		require.NoError(t, err)

		_, err = h.Provider.Exchange(t.Context(), code, attempt)
		require.ErrorIs(t, err, socialite.ErrExchange)
	})

	t.Run("refuses a code it never issued", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.Provider.Exchange(t.Context(), "made-up", NewAttempt())
		require.ErrorIs(t, err, socialite.ErrExchange)
	})
}
