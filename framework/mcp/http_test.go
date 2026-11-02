package mcp_test

import (
	"encoding/json"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
reply is a response as the tests read it: status, headers and the body
decoded when it is JSON.
*/
type reply struct {
	status int
	header stdhttp.Header
	body   map[string]any
}

/*
send posts a JSON-RPC body to the URL with the headers.
*/
func send(t *testing.T, url, body string, headers map[string]string) reply {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, url, strings.NewReader(body))
	req.RequestURI = ""
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	hc, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	res, err := hc.StdClient().Do(req)
	require.NoError(t, err)

	defer func() { _ = res.Body.Close() }()

	data, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	out := reply{status: res.StatusCode, header: res.Header}
	_ = json.Unmarshal(data, &out.body)

	return out
}

/*
ping is a JSON-RPC request every server answers.
*/
const ping = `{"jsonrpc":"2.0","id":1,"method":"ping"}`

func TestHTTPHandler(t *testing.T) {
	t.Parallel()

	ts := newHTTP(t, newServer(t, nil))

	t.Run("refuse a request without a token, naming the resource metadata", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/mcp", ping, nil)

		assert.Equal(t, http.StatusUnauthorized, res.status)
		assert.Equal(t, `Bearer resource_metadata="`+metadataURL+`"`, res.header.Get("WWW-Authenticate"))
		assert.Equal(t, "unauthenticated", res.body["code"])
	})

	t.Run("say a token that does not check out was refused", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/mcp", ping, map[string]string{"Authorization": "Bearer tok_revoked"})

		assert.Equal(t, http.StatusUnauthorized, res.status)
		assert.Equal(t, `Bearer error="invalid_token", resource_metadata="`+metadataURL+`"`,
			res.header.Get("WWW-Authenticate"))
		assert.Equal(t, "invalid_token", res.body["code"])
	})

	t.Run("treat a token no resolver knows as refused", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/mcp", ping, map[string]string{"Authorization": "Bearer other_1"})

		assert.Equal(t, http.StatusUnauthorized, res.status)
		assert.Contains(t, res.header.Get("WWW-Authenticate"), `error="invalid_token"`)
	})

	t.Run("answer a caller with a valid token", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/mcp", ping, map[string]string{"Authorization": "Bearer tok_"})

		assert.Equal(t, http.StatusOK, res.status)
		assert.Equal(t, map[string]any{}, res.body["result"])
	})

	t.Run("refuse a browser on a site that is not allowed", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/mcp", ping, map[string]string{
			"Authorization": "Bearer tok_", "Origin": "https://evil.example.com",
		})

		assert.Equal(t, http.StatusForbidden, res.status)
		assert.Equal(t, "origin_not_allowed", res.body["code"])
	})

	t.Run("refuse a malformed origin", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/mcp", ping, map[string]string{"Authorization": "Bearer tok_", "Origin": "file:///x"})

		assert.Equal(t, http.StatusForbidden, res.status)
	})

	t.Run("let in a browser on an allowed site", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/mcp", ping, map[string]string{
			"Authorization": "Bearer tok_", "Origin": "https://app.example.com",
		})

		assert.Equal(t, http.StatusOK, res.status)
	})

	t.Run("refuse a toolset nothing was registered under, listing the valid ones", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/mcp?toolsets=records,nope", ping, map[string]string{"Authorization": "Bearer tok_"})

		assert.Equal(t, http.StatusUnprocessableEntity, res.status)
		assert.Equal(t, "unknown_toolset", res.body["code"])
		assert.Equal(t, "nope", res.body["toolset"])
		assert.Equal(t, []any{"records", "reports"}, res.body["toolsets"])
	})

	t.Run("refuse a body over the limit", func(t *testing.T) {
		t.Parallel()

		pad := strings.Repeat("x", 70<<10)
		big := `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"_meta":{"pad":"` + pad + `"}}}`
		res := send(t, ts.URL+"/mcp", big, map[string]string{"Authorization": "Bearer tok_"})

		assert.Equal(t, http.StatusRequestEntityTooLarge, res.status)
	})

	t.Run("refuse streams, as it keeps no sessions", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/mcp", nil)
		req.RequestURI = ""
		req.Header.Set("Authorization", "Bearer tok_")

		res, err := ts.Client().Do(req)
		require.NoError(t, err)
		_ = res.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
	})

	t.Run("serve the caller the route's auth middleware let in", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/routed", "tok_", nil)

		res := call(t, cs, "whoami", nil)

		assert.Equal(t, tokenUser, structured(t, res)["id"])
	})

	t.Run("challenge a request that reached the handler without a caller", func(t *testing.T) {
		t.Parallel()

		res := send(t, ts.URL+"/open", ping, nil)

		assert.Equal(t, http.StatusUnauthorized, res.status)
		assert.Equal(t, `Bearer resource_metadata="`+metadataURL+`"`, res.header.Get("WWW-Authenticate"))
	})
}

func TestProtectedResourceHandler(t *testing.T) {
	t.Parallel()

	ts := newHTTP(t, newServer(t, nil))

	t.Run("describe the resource and the server that grants its tokens", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			ts.URL+"/.well-known/oauth-protected-resource/mcp", nil)
		req.RequestURI = ""

		res, err := ts.Client().Do(req)
		require.NoError(t, err)
		t.Cleanup(func() { _ = res.Body.Close() })

		var body map[string]any
		require.NoError(t, json.NewDecoder(res.Body).Decode(&body))

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, map[string]any{
			"resource":                 "https://api.example.com/mcp",
			"authorization_servers":    []any{"https://auth.example.com"},
			"scopes_supported":         []any{"records:read", "records:write"},
			"bearer_methods_supported": []any{"header"},
			"resource_name":            "Test",
		}, body)
	})
}
