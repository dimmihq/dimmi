package mcp_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
dial connects the client to the test server with a token holding no
ability.
*/
func dial(t *testing.T, url string) *mcp.Client {
	t.Helper()

	hc, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	c, err := mcp.Connect(t.Context(), url, mcp.ClientOptions{
		Name:    "app",
		Version: "1.0.0",
		HTTP:    hc,
		Headers: http.Header{"Authorization": {"Bearer tok_"}},
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	return c
}

func TestClient(t *testing.T) {
	t.Parallel()

	ts := newHTTP(t, newServer(t, nil))

	t.Run("list the tools the server offers", func(t *testing.T) {
		t.Parallel()
		c := dial(t, ts.URL+"/mcp")

		tools, next, err := c.ListTools(t.Context(), "")
		require.NoError(t, err)

		assert.Empty(t, next)
		require.Len(t, tools, 4)

		byName := map[string]mcp.ToolInfo{}
		for _, tool := range tools {
			byName[tool.Name] = tool
		}

		assert.Equal(t, "Repeat a message.", byName["echo"].Description)
		assert.Contains(t, string(byName["echo"].InputSchema), `"message"`)
	})

	t.Run("call a tool and read its text and structured content", func(t *testing.T) {
		t.Parallel()
		c := dial(t, ts.URL+"/mcp")

		res, err := c.CallTool(t.Context(), "echo", map[string]any{"message": "hi"})
		require.NoError(t, err)

		assert.Equal(t, mcp.ToolResult{Text: "echo: hi", Structured: []byte(`{"echo":"hi"}`)}, res)
	})

	t.Run("report a tool that failed as a result", func(t *testing.T) {
		t.Parallel()
		c := dial(t, ts.URL+"/mcp")

		res, err := c.CallTool(t.Context(), "fail", nil)
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Equal(t, "Try again later.", res.Text)
	})

	t.Run("return an error for a tool the server does not have", func(t *testing.T) {
		t.Parallel()
		c := dial(t, ts.URL+"/mcp")

		_, err := c.CallTool(t.Context(), "nope", nil)

		require.ErrorContains(t, err, "call mcp tool nope")
	})

	t.Run("refuse to connect to a private address", func(t *testing.T) {
		t.Parallel()

		hc, err := httpclient.New(httpclient.Config{})
		require.NoError(t, err)

		_, err = mcp.Connect(t.Context(), ts.URL+"/mcp", mcp.ClientOptions{Name: "app", Version: "1.0.0", HTTP: hc})

		require.ErrorIs(t, err, safedial.ErrBlocked)
	})

	t.Run("fail calls once the connection is closed", func(t *testing.T) {
		t.Parallel()
		c := dial(t, ts.URL+"/mcp")
		require.NoError(t, c.Close())

		_, _, err := c.ListTools(t.Context(), "")

		require.Error(t, err)
	})
}
