package oauth_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/oauth"
)

func TestMigrations(t *testing.T) {
	t.Parallel()

	t.Run("pass the migration lint", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, migrate.Lint(oauth.Migrations()))
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	refused := map[string]func(*oauth.Options){
		"an issuer with a path":    func(o *oauth.Options) { o.Issuer = issuer + "/tenant" },
		"an issuer that is no url": func(o *oauth.Options) { o.Issuer = "::" },
		"an issuer with another scheme": func(o *oauth.Options) {
			o.Issuer = "ftp://auth.example.com"
		},
		"a prefix with dashes":                func(o *oauth.Options) { o.Prefix = "app-" },
		"registration without a rate limiter": func(o *oauth.Options) { o.Registration.Enabled = true },
		"a configured client without an id":   func(o *oauth.Options) { o.Clients[0].ID = "" },
		"a configured confidential client":    func(o *oauth.Options) { o.Clients[0].Confidential = true },
		"a configured client with a bad redirect": func(o *oauth.Options) {
			o.Clients[0].RedirectURIs = []string{"javascript:alert(1)"}
		},
	}

	for name, change := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			t.Parallel()
			opts := baseOptions()
			opts.Registration.Enabled = false
			change(&opts)

			_, err := oauth.New(nil, clock.System(), nil, opts)

			require.ErrorIs(t, err, oauth.ErrInvalidOptions)
		})
	}

	t.Run("names the issuer without a trailing slash", func(t *testing.T) {
		t.Parallel()
		opts := baseOptions()
		opts.Registration.Enabled = false

		server, err := oauth.New(nil, clock.System(), nil, opts)

		require.NoError(t, err)
		assert.Equal(t, issuer, server.Issuer())
		assert.Equal(t, issuer+"/.well-known/oauth-authorization-server", server.MetadataURL())
	})
}

func TestMetadata(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("describes every endpoint and requires S256", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db)

		rec := get(t, h.handler, "/.well-known/oauth-authorization-server")

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "public, max-age=3600", rec.Header().Get("Cache-Control"))
		assert.JSONEq(t, `{
			"issuer": "https://auth.example.com",
			"authorization_endpoint": "https://auth.example.com/oauth/authorize",
			"token_endpoint": "https://auth.example.com/oauth/token",
			"revocation_endpoint": "https://auth.example.com/oauth/revoke",
			"registration_endpoint": "https://auth.example.com/oauth/register",
			"scopes_supported": ["records:read", "records:write"],
			"response_types_supported": ["code"],
			"response_modes_supported": ["query"],
			"grant_types_supported": ["authorization_code", "refresh_token"],
			"token_endpoint_auth_methods_supported": ["none", "client_secret_basic", "client_secret_post"],
			"revocation_endpoint_auth_methods_supported": ["none", "client_secret_basic", "client_secret_post"],
			"code_challenge_methods_supported": ["S256"],
			"authorization_response_iss_parameter_supported": true
		}`, rec.Body.String())
	})

	t.Run("leaves registration out when it is disabled", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, db, func(o *oauth.Options) {
			o.Registration.Enabled = false
			o.AuthorizationPath = "/consent"
		})

		var doc oauth.Metadata
		require.NoError(t, json.Unmarshal(get(t, h.handler, "/.well-known/oauth-authorization-server").Body.Bytes(),
			&doc))

		assert.Empty(t, doc.RegistrationEndpoint)
		assert.Equal(t, issuer+"/consent", doc.AuthorizationEndpoint)
		assert.Equal(t, doc, h.server.Metadata())
		assert.Equal(t, http.StatusNotFound, h.post(t, "/oauth/register", nil).Code)
	})
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))

	return rec
}
