/*
|--------------------------------------------------------------------------
| Local Identity Provider
|--------------------------------------------------------------------------
|
| Drivers are tested against a real HTTP server playing the identity
| provider, so the drivers run unchanged: discovery, the token endpoint
| and the published keys answer over the network, and ID tokens are
| really signed. Like a real provider, the server binds each code to the
| PKCE challenge and redirect URL it was issued for and honors it once.
| An ID token comes back only when the openid scope was asked for.
|
| Approve stands in for the user clicking through the sign-in page, and
| Handle adds provider-specific API routes, such as a user endpoint.
|
*/

package drivertest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
keyID names the server's signing key.
*/
const keyID = "test-key"

/*
Server is a local identity provider. URL is its issuer and base URL;
ClientID and ClientSecret are the one client registered with it; User
is who signs in when Approve is called.
*/
type Server struct {
	URL          string
	ClientID     string
	ClientSecret string
	User         socialite.User

	t      *testing.T
	key    jwt.Key
	public jose.JSONWebKey
	mux    *http.ServeMux

	mu     sync.Mutex
	codes  map[string]grant
	tokens map[string]bool
	tamper func(claims *jwt.Claims)
}

/*
grant is what a code was issued for.
*/
type grant struct {
	challenge   string
	redirectURL string
	nonce       string
	openID      bool
}

/*
NewServer starts a provider that signs the user in, stopped when the
test ends.
*/
func NewServer(t *testing.T, user socialite.User) *Server {
	t.Helper()

	private, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	key, err := jwt.NewKey(keyID, jwt.RS256, private)
	require.NoError(t, err)

	s := &Server{
		ClientID:     "client-" + str.Random(8),
		ClientSecret: str.Random(32),
		User:         user,
		t:            t,
		key:          key,
		public:       jose.JSONWebKey{Key: &private.PublicKey, KeyID: keyID, Algorithm: string(jwt.RS256), Use: "sig"},
		mux:          http.NewServeMux(),
		codes:        map[string]grant{},
		tokens:       map[string]bool{},
	}

	s.mux.HandleFunc("GET /.well-known/openid-configuration", s.discovery)
	s.mux.HandleFunc("GET /jwks", s.jwks)
	s.mux.HandleFunc("POST /token", s.token)

	srv := httptest.NewServer(s.mux)
	t.Cleanup(srv.Close)
	s.URL = srv.URL

	return s
}

/*
Handle serves an extra route, such as a provider's user API.
*/
func (s *Server) Handle(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

/*
Authorized reports whether the request carries an access token the
server issued.
*/
func (s *Server) Authorized(r *http.Request) bool {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")

	s.mu.Lock()
	defer s.mu.Unlock()

	return ok && s.tokens[token]
}

/*
TamperIDToken changes the claims of every ID token issued from now on,
before they are signed.
*/
func (s *Server) TamperIDToken(tamper func(claims *jwt.Claims)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tamper = tamper
}

/*
Approve signs the user in for the authorization URL, which must point at
this server, and returns the callback URL with a fresh code.
*/
func (s *Server) Approve(t *testing.T, authURL string) string {
	t.Helper()

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	query := parsed.Query()

	require.Equal(t, s.URL+"/authorize", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	require.Equal(t, s.ClientID, query.Get("client_id"))
	require.Equal(t, "code", query.Get("response_type"))
	require.Equal(t, "S256", query.Get("code_challenge_method"))

	callback, err := url.Parse(query.Get("redirect_uri"))
	require.NoError(t, err)

	code := str.Random(32)

	s.mu.Lock()
	s.codes[code] = grant{
		challenge:   query.Get("code_challenge"),
		redirectURL: query.Get("redirect_uri"),
		nonce:       query.Get("nonce"),
		openID:      slices.Contains(strings.Fields(query.Get("scope")), "openid"),
	}
	s.mu.Unlock()

	values := callback.Query()
	values.Set("code", code)
	values.Set("state", query.Get("state"))
	callback.RawQuery = values.Encode()

	return callback.String()
}

func (s *Server) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                s.URL,
		"authorization_endpoint":                s.URL + "/authorize",
		"token_endpoint":                        s.URL + "/token",
		"jwks_uri":                              s.URL + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{string(jwt.RS256)},
	})
}

func (s *Server) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{s.public}})
}

/*
token redeems a code, accepting client credentials in the Authorization
header or the form, as providers do.
*/
func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})

		return
	}

	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}

	if id != s.ClientID || secret != s.ClientSecret {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})

		return
	}

	issued, ok := s.redeem(r.PostForm)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})

		return
	}

	access := str.Random(32)
	body := map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": str.Random(32),
	}

	if issued.openID {
		body["id_token"] = s.idToken(issued.nonce)
	}

	s.mu.Lock()
	s.tokens[access] = true
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, body)
}

/*
redeem uses up the code and returns its grant when the form matches it.
*/
func (s *Server) redeem(form url.Values) (grant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	issued, ok := s.codes[form.Get("code")]
	delete(s.codes, form.Get("code"))

	return issued, ok &&
		form.Get("grant_type") == "authorization_code" &&
		form.Get("redirect_uri") == issued.redirectURL &&
		oauth2.S256ChallengeFromVerifier(form.Get("code_verifier")) == issued.challenge
}

func (s *Server) idToken(nonce string) string {
	now := clock.System().Now()
	claims := jwt.Claims{
		Issuer:   s.URL,
		Subject:  s.User.ID,
		Audience: []string{s.ClientID},
		IssuedAt: now,
		Expiry:   now.Add(time.Hour),
		Extra: map[string]any{
			"nonce":          nonce,
			"email":          s.User.Email,
			"email_verified": s.User.EmailVerified,
			"name":           s.User.Name,
			"picture":        s.User.AvatarURL,
		},
	}

	s.mu.Lock()
	if s.tamper != nil {
		s.tamper(&claims)
	}
	s.mu.Unlock()

	token, err := jwt.Sign(claims, s.key)
	if err != nil {
		s.t.Errorf("sign id token: %v", err)
	}

	return token
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
