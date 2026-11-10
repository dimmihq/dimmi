/*
|--------------------------------------------------------------------------
| Local Provider
|--------------------------------------------------------------------------
|
| Drivers are tested against a real HTTP server playing a provider that
| follows the OAuth 2 standard, so the drivers run unchanged over the
| network. Like a real provider, the server binds each code to the
| redirect URL and PKCE challenge it was issued for and honors it once,
| rotates refresh tokens and takes tokens back at its revocation
| endpoint.
|
| Approve stands in for the person clicking through the consent page,
| and Handle adds provider-specific API routes.
|
*/

package drivertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
tokenLifetime is how many seconds the server's access tokens last.
*/
const tokenLifetime = 3600

/*
Server is a local provider. URL is its base URL; ClientID and
ClientSecret are the one client registered with it.
*/
type Server struct {
	URL          string
	ClientID     string
	ClientSecret string

	mux *http.ServeMux

	mu       sync.Mutex
	codes    map[string]issued
	access   map[string]bool
	refreshs map[string]bool
	revoked  []string
}

/*
issued is what a code was issued for.
*/
type issued struct {
	challenge   string
	redirectURL string
	scope       string
}

/*
NewServer starts a provider, stopped when the test ends. Its
authorization URL is URL/authorize, its token URL URL/token and its
revocation URL URL/revoke.
*/
func NewServer(t *testing.T) *Server {
	t.Helper()

	s := &Server{
		ClientID: "client-" + str.Random(8), ClientSecret: str.Random(32), mux: http.NewServeMux(),
		codes: map[string]issued{}, access: map[string]bool{}, refreshs: map[string]bool{},
	}

	s.mux.HandleFunc("POST /token", s.token)
	s.mux.HandleFunc("POST /revoke", s.revoke)

	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)
	s.URL = srv.URL

	return s
}

/*
Handle serves an extra route, such as a provider's API.
*/
func (s *Server) Handle(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

/*
Revoked returns every token given back, oldest first.
*/
func (s *Server) Revoked() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.revoked)
}

/*
Approve plays the person approving the authorization URL and returns
the callback URL carrying a fresh code and the state.
*/
func (s *Server) Approve(t *testing.T, authURL string) string {
	t.Helper()

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	query := parsed.Query()
	require.Equal(t, s.ClientID, query.Get("client_id"))

	callback, err := url.Parse(query.Get("redirect_uri"))
	require.NoError(t, err)

	code := str.Random(32)

	s.mu.Lock()
	s.codes[code] = issued{
		challenge: query.Get("code_challenge"), redirectURL: callback.String(), scope: query.Get("scope"),
	}
	s.mu.Unlock()

	values := callback.Query()
	values.Set("code", code)
	values.Set("state", query.Get("state"))
	callback.RawQuery = values.Encode()

	return callback.String()
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if !s.client(r) {
		answer(w, http.StatusUnauthorized, map[string]any{"error": "invalid_client"})

		return
	}

	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.redeem(w, r)
	case "refresh_token":
		s.renew(w, r)
	default:
		answer(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
	}
}

/*
redeem trades a code for tokens once, when the redirect URL and the
verifier match.
*/
func (s *Server) redeem(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	code, ok := s.codes[r.PostForm.Get("code")]
	delete(s.codes, r.PostForm.Get("code"))
	s.mu.Unlock()

	verified := code.challenge == "" ||
		code.challenge == oauth2.S256ChallengeFromVerifier(r.PostForm.Get("code_verifier"))

	if !ok || code.redirectURL != r.PostForm.Get("redirect_uri") || !verified {
		answer(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})

		return
	}

	answer(w, http.StatusOK, s.issue(code.scope))
}

/*
renew trades a refresh token for fresh tokens, spending it.
*/
func (s *Server) renew(w http.ResponseWriter, r *http.Request) {
	refresh := r.PostForm.Get("refresh_token")

	s.mu.Lock()
	ok := s.refreshs[refresh]
	delete(s.refreshs, refresh)
	s.mu.Unlock()

	if !ok {
		answer(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})

		return
	}

	answer(w, http.StatusOK, s.issue(""))
}

func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	if !s.client(r) {
		answer(w, http.StatusUnauthorized, map[string]any{"error": "invalid_client"})

		return
	}

	token := r.PostForm.Get("token")

	s.mu.Lock()
	delete(s.access, token)
	delete(s.refreshs, token)
	s.revoked = append(s.revoked, token)
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

/*
client reads the form and reports whether it authenticates the
registered client, in the form or as basic auth.
*/
func (s *Server) client(r *http.Request) bool {
	if r.ParseForm() != nil {
		return false
	}

	id, secret, basic := r.BasicAuth()
	if !basic {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}

	return id == s.ClientID && secret == s.ClientSecret
}

/*
issue mints an access and a refresh token.
*/
func (s *Server) issue(scope string) map[string]any {
	access, refresh := "at-"+str.Random(32), "rt-"+str.Random(32)

	s.mu.Lock()
	s.access[access] = true
	s.refreshs[refresh] = true
	s.mu.Unlock()

	return map[string]any{
		"access_token": access, "token_type": "Bearer", "expires_in": tokenLifetime,
		"refresh_token": refresh, "scope": scope,
	}
}

func answer(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
