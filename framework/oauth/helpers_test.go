package oauth_test

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauth"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
The values every test shares. The verifier and challenge are the PKCE
example pair from the standard.
*/
const (
	issuer    = "https://auth.example.com"
	prefix    = "app_"
	callback  = "https://client.example.com/callback"
	verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	mobileID  = "mobile"
	mobileURI = "com.example.app:/callback"
)

/*
openDB returns a small pool on a fresh database migrated with the OAuth
tables.
*/
func openDB(t *testing.T) *database.DB {
	t.Helper()

	dbURL := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: dbURL, Framework: oauth.Migrations()}.Up(t.Context())
	require.NoError(t, err)

	db, err := database.Open(t.Context(), database.Config{URL: dbURL, MaxConns: 3, PoolMode: database.PoolModeSession})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

/*
harness is a server with a frozen clock, behind a router that serves
its endpoints and two routes guarded by its access tokens.
*/
type harness struct {
	server  *oauth.Server
	clock   *clock.Fake
	handler http.Handler
}

/*
baseOptions returns the options every harness starts from: two scopes,
registration on and a configured first-party client.
*/
func baseOptions() oauth.Options {
	return oauth.Options{
		Issuer:       issuer + "/",
		Prefix:       prefix,
		Scopes:       []string{"records:read", "records:write"},
		Registration: oauth.Registration{Enabled: true, MaxAttempts: 100},
		Clients: []oauth.Client{{
			ID: mobileID, Name: "Mobile", RedirectURIs: []string{mobileURI}, FirstParty: true,
		}},
	}
}

func newHarness(t *testing.T, db *database.DB, change ...func(*oauth.Options)) *harness {
	t.Helper()

	opts := baseOptions()
	for _, fn := range change {
		fn(&opts)
	}

	limiter, err := ratelimit.New(testkit.NewValkey(t), ratelimit.Options{Keyspace: "rl"})
	require.NoError(t, err)

	clk := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))
	server, err := oauth.New(db, clk, limiter, opts)
	require.NoError(t, err)

	return &harness{server: server, clock: clk, handler: routes(t, server)}
}

/*
routes returns the server's endpoints and a read and a write route
behind the bearer guard that resolves its access tokens.
*/
func routes(t *testing.T, server *oauth.Server) http.Handler {
	t.Helper()

	guards := auth.NewGuards(http.Errors{})
	guards.Register("bearer", auth.BearerGuard(server.Resolver()))

	kernel := http.NewKernel()
	kernel.Alias("auth", guards.Middleware)

	r := routing.New(routing.Options{Middleware: kernel})
	server.Routes(r)
	r.Handle(http.MethodGet, "/records", "records.index", whoami()).Middleware("auth").Ability("records:read")
	r.Handle(http.MethodPost, "/records", "records.store", whoami()).Middleware("auth").Ability("records:write")

	h, err := r.Handler()
	require.NoError(t, err)

	return h
}

/*
caller is what a guarded route saw of its caller.
*/
type caller struct {
	ID         string   `json:"id"`
	Guard      string   `json:"guard"`
	Tenant     string   `json:"tenant"`
	Abilities  []string `json:"abilities"`
	Credential string   `json:"credential"`
	Confirmed  int64    `json:"confirmed"`
}

func whoami() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.From(r.Context())
		identity, _ := p.(*auth.Identity)
		http.WriteJSON(w, http.StatusOK, caller{
			ID: p.ID().String(), Guard: p.Guard(), Tenant: p.Tenant(), Abilities: p.Abilities(),
			Credential: identity.Credential.String(), Confirmed: p.ConfirmedAt().Unix(),
		})
	})
}

/*
publicClient registers a public client with one redirect URI.
*/
func (h *harness) publicClient(t *testing.T, change ...func(*oauth.Client)) oauth.Client {
	t.Helper()

	c := oauth.Client{Name: "Tool", RedirectURIs: []string{callback}}
	for _, fn := range change {
		fn(&c)
	}

	created, _, err := h.server.CreateClient(t.Context(), c)
	require.NoError(t, err)

	return created
}

/*
query returns a valid authorization query for the client, with the
overrides applied; an empty override removes the parameter.
*/
func query(clientID string, overrides map[string]string) url.Values {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {callback},
		"state":                 {"xyz"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	for name, value := range overrides {
		if value == "" {
			q.Del(name)

			continue
		}

		q.Set(name, value)
	}

	return q
}

/*
code approves a valid request for the client and returns the code from
the redirect.
*/
func (h *harness) code(t *testing.T, q url.Values, a oauth.Approval) string {
	t.Helper()

	req, err := h.server.ValidateAuthorization(t.Context(), q)
	require.NoError(t, err)

	if a.User == ids.Nil() {
		a.User = ids.New()
	}

	to, err := h.server.Approve(t.Context(), req, a)
	require.NoError(t, err)

	u, err := url.Parse(to)
	require.NoError(t, err)

	return u.Query().Get("code")
}

/*
tokens is a token endpoint answer, success or error.
*/
type tokens struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

/*
post sends the form to the path and returns the recorded response.
*/
func (h *harness) post(t *testing.T, path string, form url.Values, basic ...string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if len(basic) == 2 {
		r.SetBasicAuth(basic[0], basic[1])
	}

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, r)

	return rec
}

/*
token posts to the token endpoint and decodes the answer.
*/
func (h *harness) token(t *testing.T, form url.Values, basic ...string) (int, tokens) {
	t.Helper()

	rec := h.post(t, "/oauth/token", form, basic...)

	var out tokens
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())

	return rec.Code, out
}

/*
exchange redeems the code for the public client and requires success.
*/
func (h *harness) exchange(t *testing.T, clientID, code string) tokens {
	t.Helper()

	status, out := h.token(t, exchangeForm(clientID, code))
	require.Equal(t, http.StatusOK, status, out)

	return out
}

func exchangeForm(clientID, code string) url.Values {
	return url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code},
		"redirect_uri": {callback}, "code_verifier": {verifier},
	}
}

func refreshForm(clientID, refresh string) url.Values {
	return url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {refresh}}
}

/*
call sends a request with the bearer token to the guarded route.
*/
func (h *harness) call(t *testing.T, method, bearer string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), method, "/records", nil)
	r.Header.Set("Authorization", "Bearer "+bearer)

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, r)

	return rec
}

/*
whoIs calls the read route with the token and returns the caller it saw.
*/
func (h *harness) whoIs(t *testing.T, bearer string) caller {
	t.Helper()

	rec := h.call(t, http.MethodGet, bearer)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out caller
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	return out
}

/*
errorCode returns the code field of a JSON error body.
*/
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var out struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	return out.Code
}
