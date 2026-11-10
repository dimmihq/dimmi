package slack_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivers/slack"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivertest"
)

/*
slackServer plays Slack's install endpoints: codes bound to their
redirect URL and used once, rotating refresh tokens, and auth.revoke.
*/
type slackServer struct {
	url    string
	mu     sync.Mutex
	codes  map[string]url.Values
	tokens map[string]bool
	n      int
}

func newSlackServer(t *testing.T) *slackServer {
	t.Helper()

	s := &slackServer{codes: map[string]url.Values{}, tokens: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/oauth.v2.access", s.access)
	mux.HandleFunc("POST /api/auth.revoke", s.revoke)
	mux.HandleFunc("POST /broken/oauth.v2.access", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("POST /garbage/", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{")) })

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s.url = srv.URL

	return s
}

func (s *slackServer) approve(t *testing.T, authURL string) string {
	t.Helper()

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	query := parsed.Query()
	require.Equal(t, "app-id", query.Get("client_id"))

	s.mu.Lock()
	s.n++
	code := "code-" + strings.Repeat("x", s.n)
	s.codes[code] = query
	s.mu.Unlock()

	callback, err := url.Parse(query.Get("redirect_uri"))
	require.NoError(t, err)
	callback.RawQuery = url.Values{"code": {code}, "state": {query.Get("state")}}.Encode()

	return callback.String()
}

func (s *slackServer) access(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if r.PostForm.Get("client_id") != "app-id" || r.PostForm.Get("client_secret") != "app-secret" {
		reply(w, map[string]any{"ok": false, "error": "invalid_client_id"})

		return
	}

	if r.PostForm.Get("grant_type") == "refresh_token" {
		s.refresh(w, r.PostForm.Get("refresh_token"))

		return
	}

	s.mu.Lock()
	query, ok := s.codes[r.PostForm.Get("code")]
	delete(s.codes, r.PostForm.Get("code"))
	s.mu.Unlock()

	if !ok || query.Get("redirect_uri") != r.PostForm.Get("redirect_uri") {
		reply(w, map[string]any{"ok": false, "error": "invalid_code"})

		return
	}

	reply(w, s.install(query))
}

func (s *slackServer) refresh(w http.ResponseWriter, refresh string) {
	s.mu.Lock()
	ok := s.tokens[refresh]
	delete(s.tokens, refresh)
	s.mu.Unlock()

	if !ok {
		reply(w, map[string]any{"ok": false, "error": "invalid_refresh_token"})

		return
	}

	reply(w, map[string]any{"ok": true, "access_token": s.mint("xoxe.xoxb-"), "expires_in": 43200})
}

func (s *slackServer) install(query url.Values) map[string]any {
	out := map[string]any{
		"ok": true, "app_id": "A1", "bot_user_id": "UB1", "team": map[string]any{"id": "T1", "name": "Acme"},
		"enterprise": nil, "is_enterprise_install": false,
		"authed_user": map[string]any{"id": "U1"},
	}

	if scope := query.Get("scope"); scope != "" {
		out["access_token"], out["scope"] = s.mint("xoxb-"), scope
		out["refresh_token"], out["expires_in"] = s.mint("xoxe-1-"), 43200
	}

	if scope := query.Get("user_scope"); scope != "" {
		out["authed_user"] = map[string]any{"id": "U1", "scope": scope, "access_token": s.mint("xoxp-")}
	}

	return out
}

func (s *slackServer) mint(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.n++
	token := prefix + strings.Repeat("t", s.n)
	s.tokens[token] = true

	return token
}

func (s *slackServer) revoke(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "boom" {
		reply(w, map[string]any{"ok": false, "error": "internal_error"})

		return
	}

	s.mu.Lock()
	known := s.tokens[token]
	delete(s.tokens, token)
	s.mu.Unlock()

	if !known {
		reply(w, map[string]any{"ok": false, "error": "invalid_auth"})

		return
	}

	reply(w, map[string]any{"ok": true, "revoked": true})
}

func reply(w http.ResponseWriter, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func newDriver(t *testing.T, s *slackServer, extra config.Section) oauthclient.Driver {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	section := config.Section{
		"client_id": "app-id", "client_secret": "app-secret",
		"auth_url": s.url + "/oauth/v2/authorize", "api_url": s.url + "/api/",
	}
	maps.Copy(section, extra)

	driver, err := slack.Factory(client, clock.NewFake(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))("slack", section)
	require.NoError(t, err)

	return driver
}

func TestDriver(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, func(t *testing.T) drivertest.Harness {
		t.Helper()
		s := newSlackServer(t)

		return drivertest.Harness{Driver: newDriver(t, s, nil), Approve: s.approve, Refreshes: true, Revokes: true}
	})
}

func TestSlack(t *testing.T) {
	t.Parallel()

	t.Run("asks for bot and user scopes as comma lists", func(t *testing.T) {
		t.Parallel()
		driver := newDriver(t, newSlackServer(t), nil)
		attempt := drivertest.NewAttempt("chat:write", "channels:read")
		attempt.UserScopes = []string{"chat:write", "reactions:write"}
		attempt.Extra = map[string]string{"team": "T1"}

		authURL, err := driver.AuthURL(t.Context(), attempt)
		require.NoError(t, err)

		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, "chat:write,channels:read", parsed.Query().Get("scope"))
		assert.Equal(t, "chat:write,reactions:write", parsed.Query().Get("user_scope"))
		assert.Equal(t, "T1", parsed.Query().Get("team"))
	})

	t.Run("falls back to the configured scopes", func(t *testing.T) {
		t.Parallel()
		defaults := config.Section{"scopes": "chat:write", "user_scopes": "reactions:write"}
		driver := newDriver(t, newSlackServer(t), defaults)

		authURL, err := driver.AuthURL(t.Context(), drivertest.NewAttempt())
		require.NoError(t, err)

		parsed, _ := url.Parse(authURL)
		assert.Equal(t, "chat:write", parsed.Query().Get("scope"))
		assert.Equal(t, "reactions:write", parsed.Query().Get("user_scope"))
	})

	t.Run("reads the install without keeping a token in raw", func(t *testing.T) {
		t.Parallel()
		s := newSlackServer(t)
		h := drivertest.Harness{Driver: newDriver(t, s, nil), Approve: s.approve}

		grant := drivertest.Connect(t, h)

		assert.Equal(t, "T1", grant.Account)
		assert.Equal(t, "Acme", grant.AccountName)
		assert.Equal(t, "A1", grant.App)
		assert.Equal(t, "U1", grant.UserID)
		assert.Equal(t, []string{"read"}, grant.Scopes)
		assert.Equal(t, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), grant.Access.Expiry)
		assert.Equal(t, false, grant.Raw["is_enterprise_install"])
		assert.Equal(t, "UB1", grant.Raw["bot_user_id"])
		encoded, err := json.Marshal(grant.Raw)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "xox")
	})

	t.Run("reads a member's own token", func(t *testing.T) {
		t.Parallel()
		s := newSlackServer(t)
		driver := newDriver(t, s, nil)
		attempt := drivertest.NewAttempt()
		attempt.UserScopes = []string{"chat:write"}
		authURL, err := driver.AuthURL(t.Context(), attempt)
		require.NoError(t, err)
		callback, _ := url.Parse(s.approve(t, authURL))

		grant, err := driver.Exchange(t.Context(), callback.Query().Get("code"), attempt)

		require.NoError(t, err)
		assert.Nil(t, grant.Access)
		require.NotNil(t, grant.UserAccess)
		assert.True(t, strings.HasPrefix(grant.UserAccess.Value, "xoxp-"))
		assert.Equal(t, []string{"chat:write"}, grant.UserScopes)
		encoded, _ := json.Marshal(grant.Raw)
		assert.NotContains(t, string(encoded), "xoxp")
	})

	t.Run("keeps the refresh token slack did not rotate", func(t *testing.T) {
		t.Parallel()
		s := newSlackServer(t)
		h := drivertest.Harness{Driver: newDriver(t, s, nil), Approve: s.approve}
		grant := drivertest.Connect(t, h)

		renewed, err := h.Driver.Refresh(t.Context(), grant.Refresh.Value)

		require.NoError(t, err)
		assert.Equal(t, grant.Refresh.Value, renewed.Refresh.Value)
	})

	t.Run("reports slack failing", func(t *testing.T) {
		t.Parallel()
		s := newSlackServer(t)
		broken := newDriver(t, s, config.Section{"api_url": s.url + "/broken"})
		garbage := newDriver(t, s, config.Section{"api_url": s.url + "/garbage"})
		gone := newDriver(t, s, config.Section{"api_url": "http://127.0.0.1:1"})
		ok := newDriver(t, s, nil)

		_, err := broken.Exchange(t.Context(), "c", drivertest.NewAttempt())
		require.ErrorIs(t, err, oauthclient.ErrProvider)
		_, err = garbage.Exchange(t.Context(), "c", drivertest.NewAttempt())
		require.ErrorIs(t, err, oauthclient.ErrProvider)
		_, err = gone.Refresh(t.Context(), "r")
		require.ErrorIs(t, err, oauthclient.ErrProvider)
		require.ErrorIs(t, ok.Revoke(t.Context(), "boom"), oauthclient.ErrProvider)
		require.NoError(t, ok.Revoke(t.Context(), "unknown"))
	})

	t.Run("refuses an incomplete configuration", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{})
		require.NoError(t, err)

		_, err = slack.Factory(client, clock.System())("slack", config.Section{"client_id": "x"})
		require.ErrorIs(t, err, oauthclient.ErrMisconfigured)
		_, err = slack.Factory(client, clock.System())("slack", config.Section{"scopes": map[string]any{"a": 1}})
		require.Error(t, err)
	})
}
