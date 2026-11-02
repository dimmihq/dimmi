package oauth_test

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/oauth"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
registered is a registration answer, success or error.
*/
type registered struct {
	ClientID              string   `json:"client_id"`
	ClientSecret          string   `json:"client_secret"`
	ClientIDIssuedAt      int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt *int64   `json:"client_secret_expires_at"`
	ClientName            string   `json:"client_name"`
	RedirectURIs          []string `json:"redirect_uris"`
	GrantTypes            []string `json:"grant_types"`
	AuthMethod            string   `json:"token_endpoint_auth_method"`
	ApplicationType       string   `json:"application_type"`
	Scope                 string   `json:"scope"`
	Error                 string   `json:"error"`
}

/*
register posts the JSON body to the registration endpoint.
*/
func (h *harness) register(t *testing.T, body string) (*httptest.ResponseRecorder, registered) {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/register", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, r)

	var out registered
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())

	return rec, out
}

func TestRegistration(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	h := newHarness(t, db)

	t.Run("registers a public client that can start a flow at once", func(t *testing.T) {
		t.Parallel()
		rec, out := h.register(t, `{"client_name":"Agent","redirect_uris":["`+callback+`"]}`)

		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		assert.Regexp(t, `^app_oci_[0-9A-Za-z]{26}$`, out.ClientID)
		assert.Empty(t, out.ClientSecret)
		assert.Nil(t, out.ClientSecretExpiresAt)
		assert.Equal(t, h.clock.Now().Unix(), out.ClientIDIssuedAt)
		assert.Equal(t, "none", out.AuthMethod)
		assert.Equal(t, "native", out.ApplicationType)
		assert.Equal(t, []string{"authorization_code", "refresh_token"}, out.GrantTypes)
		assert.Equal(t, "records:read records:write", out.Scope)

		req, err := h.server.ValidateAuthorization(t.Context(), query(out.ClientID, nil))
		require.NoError(t, err)
		assert.True(t, req.Client.Dynamic)
		assert.Equal(t, "Agent", req.Client.Name)
	})

	t.Run("registers a confidential client and shows its secret once", func(t *testing.T) {
		t.Parallel()
		rec, out := h.register(t, `{"client_name":"Server","redirect_uris":["`+callback+`"],`+
			`"token_endpoint_auth_method":"client_secret_basic","application_type":"web","scope":"records:read",`+
			`"response_types":["code"],"logo_uri":"https://client.example.com/logo.png"}`)

		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.Regexp(t, `^app_ocs_[0-9A-Za-z]{43}$`, out.ClientSecret)
		require.NotNil(t, out.ClientSecretExpiresAt)
		assert.Zero(t, *out.ClientSecretExpiresAt)
		assert.Equal(t, "records:read", out.Scope)

		code := h.code(t, query(out.ClientID, nil), oauth.Approval{})
		status, tok := h.token(t, exchangeForm("", code), out.ClientID, out.ClientSecret)
		assert.Equal(t, http.StatusOK, status, tok)
	})

	uris := func(list ...string) string {
		return `{"client_name":"A","redirect_uris":["` + strings.Join(list, `","`) + `"]}`
	}
	invalid := map[string]struct {
		body string
		code string
	}{
		"a body that is not json":      {`[`, "invalid_client_metadata"},
		"no name":                      {`{"redirect_uris":["` + callback + `"]}`, "invalid_client_metadata"},
		"an unknown auth method":       {withField("token_endpoint_auth_method", `"jwt"`), "invalid_client_metadata"},
		"the implicit response type":   {withField("response_types", `["token"]`), "invalid_client_metadata"},
		"an unknown application type":  {withField("application_type", `"desktop"`), "invalid_client_metadata"},
		"a scope the server lacks":     {withField("scope", `"admin"`), "invalid_client_metadata"},
		"a grant it does not support":  {withField("grant_types", `["client_credentials"]`), "invalid_client_metadata"},
		"no redirect uri":              {`{"client_name":"Agent","redirect_uris":[]}`, "invalid_redirect_uri"},
		"a script redirect uri":        {uris("javascript:alert(1)"), "invalid_redirect_uri"},
		"a plain http public host":     {uris("http://example.com/cb"), "invalid_redirect_uri"},
		"a redirect uri with fragment": {uris("https://a.example/cb#x"), "invalid_redirect_uri"},
		"a wildcard redirect uri":      {uris("https://*.example/cb"), "invalid_redirect_uri"},
		"an https uri without host":    {uris("https:/cb"), "invalid_redirect_uri"},
		"a relative redirect uri":      {uris("/cb"), "invalid_redirect_uri"},
		"a web client on loopback": {
			`{"client_name":"A","application_type":"web","redirect_uris":["http://127.0.0.1/cb"]}`,
			"invalid_redirect_uri",
		},
		"more than ten redirect uris": {uris(slices.Repeat([]string{callback}, 11)...), "invalid_redirect_uri"},
	}

	for name, tc := range invalid {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()
			rec, out := h.register(t, tc.body)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, tc.code, out.Error)
		})
	}

	t.Run("accepts loopback and app scheme redirects for native apps", func(t *testing.T) {
		t.Parallel()
		rec, _ := h.register(t, `{"client_name":"CLI","redirect_uris":["http://[::1]/cb","http://localhost/cb",`+
			`"com.example.tool:/cb"]}`)

		assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	})
}

func withField(name, value string) string {
	return `{"client_name":"Agent","redirect_uris":["` + callback + `"],"` + name + `":` + value + `}`
}

func TestRegistrationLimits(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	body := `{"client_name":"Agent","redirect_uris":["` + callback + `"]}`

	t.Run("counts registrations per address", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db, func(o *oauth.Options) { o.Registration.MaxAttempts = 2 })

		for range 2 {
			rec, _ := h.register(t, body)
			require.Equal(t, http.StatusCreated, rec.Code)
		}

		rec, out := h.register(t, body)

		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Equal(t, "temporarily_unavailable", out.Error)
		assert.NotEmpty(t, rec.Header().Get("Retry-After"))
	})

	t.Run("refuses while the count cannot be checked", func(t *testing.T) {
		t.Parallel()
		client, err := redis.Open(t.Context(), testkit.ValkeyConfig(t))
		require.NoError(t, err)
		limiter, err := ratelimit.New(client, ratelimit.Options{})
		require.NoError(t, err)
		client.Close()
		server, err := oauth.New(db, clock.System(), limiter, baseOptions())
		require.NoError(t, err)
		h := &harness{server: server, handler: routes(t, server)}

		rec, out := h.register(t, body)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "5", rec.Header().Get("Retry-After"))
		assert.Equal(t, "temporarily_unavailable", out.Error)
	})

	t.Run("refuses an id that is taken", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		c := oauth.Client{ID: "fixed-" + testkit.UniqueName("c"), Name: "A", RedirectURIs: []string{callback}}
		_, _, err := h.server.CreateClient(t.Context(), c)
		require.NoError(t, err)

		_, _, err = h.server.CreateClient(t.Context(), c)

		require.ErrorIs(t, err, oauth.ErrClientExists)
	})
}

func TestStandardClient(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("completes the code and refresh grants with a stock oauth client", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)
		srv := httptest.NewServer(h.handler)
		t.Cleanup(srv.Close)
		client := h.publicClient(t)
		cfg := oauth2.Config{
			ClientID:    client.ID,
			RedirectURL: callback,
			Endpoint:    oauth2.Endpoint{TokenURL: srv.URL + "/oauth/token", AuthStyle: oauth2.AuthStyleInParams},
		}
		code := h.code(t, query(client.ID, map[string]string{
			"code_challenge": oauth2.S256ChallengeFromVerifier(verifier),
		}), oauth.Approval{})

		tok, err := cfg.Exchange(t.Context(), code, oauth2.VerifierOption(verifier))
		require.NoError(t, err)
		assert.Equal(t, "Bearer", tok.TokenType)

		tok.Expiry = time.Unix(1, 0)
		fresh, err := cfg.TokenSource(t.Context(), tok).Token()
		require.NoError(t, err)
		assert.NotEqual(t, tok.RefreshToken, fresh.RefreshToken)
		assert.Equal(t, http.StatusOK, h.call(t, http.MethodGet, fresh.AccessToken).Code)

		_, err = cfg.TokenSource(t.Context(), tok).Token()
		var retrieve *oauth2.RetrieveError
		require.ErrorAs(t, err, &retrieve)
		assert.Equal(t, "invalid_grant", retrieve.ErrorCode)
	})
}
