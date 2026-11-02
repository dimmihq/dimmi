package mcp_test

import (
	"context"
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
)

func TestTool(t *testing.T) {
	t.Parallel()

	ts := newHTTP(t, newServer(t, confirmRecords))

	t.Run("list only the tools outside toolsets to a caller who enabled none", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_records:read", nil)

		assert.ElementsMatch(t, []string{"crash", "echo", "fail", "whoami"}, toolNames(t, cs))
	})

	t.Run("list a toolset's tools whose abilities the caller holds", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp?toolsets=records", "tok_records:read", nil)

		assert.ElementsMatch(t, []string{"crash", "echo", "fail", "whoami", "records_read"}, toolNames(t, cs))
	})

	t.Run("list every tool to a caller holding every ability with every toolset", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp?toolsets=all", "tok_*", nil)

		assert.ElementsMatch(t, []string{
			"crash", "echo", "fail", "whoami", "records_read", "records_delete", "reports_view",
		}, toolNames(t, cs))
	})

	t.Run("describe the input schema and hints of a tool", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp?toolsets=records", "tok_*", nil)

		res, err := cs.ListTools(t.Context(), nil)
		require.NoError(t, err)

		tools := map[string]*sdk.Tool{}
		for _, tool := range res.Tools {
			tools[tool.Name] = tool
		}

		schema, ok := tools["echo"].InputSchema.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, []any{"message"}, schema["required"])
		assert.True(t, tools["echo"].Annotations.ReadOnlyHint)
		assert.True(t, *tools["records_delete"].Annotations.DestructiveHint)
		assert.Equal(t, "private", res.CacheScope)
	})

	t.Run("pass the caller and its tenant to the tool", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		res := call(t, cs, "whoami", map[string]any{})

		assert.Equal(t, map[string]any{"id": tokenUser, "tenant": "tenant-1"}, structured(t, res))
	})

	t.Run("send the output as structured content and as the text it renders", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		res := call(t, cs, "echo", map[string]any{"message": "hi", "times": 2})

		assert.False(t, res.IsError)
		assert.Equal(t, "echo: hihi", text(t, res))
		assert.Equal(t, map[string]any{"echo": "hihi"}, structured(t, res))
	})

	t.Run("send the output as JSON text when it renders none", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp?toolsets=reports", "tok_", nil)

		res := call(t, cs, "reports_view", nil)

		assert.JSONEq(t, `{"status":"viewed"}`, text(t, res))
	})

	t.Run("refuse arguments that break the input schema", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		_, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: "echo", Arguments: map[string]any{"times": 2}})

		require.Error(t, err)
		assert.Equal(t, int64(jsonrpc.CodeInvalidParams), wireCode(t, err))
		assert.Contains(t, err.Error(), "message")
	})

	t.Run("refuse arguments that are not an object", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		_, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: "echo", Arguments: []string{"hi"}})

		require.Error(t, err)
		assert.Equal(t, int64(jsonrpc.CodeInvalidParams), wireCode(t, err))
	})

	t.Run("treat a tool of a toolset the caller did not enable as unknown", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_*", nil)

		_, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: "records_read"})

		require.Error(t, err)
		assert.Equal(t, int64(jsonrpc.CodeInvalidParams), wireCode(t, err))
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("answer a caller lacking an ability with a tool error naming it", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp?toolsets=records", "tok_records:read", nil)

		res := call(t, cs, "records_delete", map[string]any{"id": "1"})

		assert.True(t, res.IsError)
		assert.Equal(t, map[string]any{
			"code": "missing_ability", "message": text(t, res), "required_ability": "records:write",
		}, structured(t, res)["error"])
	})

	t.Run("turn a fault into a tool error carrying its code and details", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		res := call(t, cs, "fail", nil)

		assert.True(t, res.IsError)
		assert.Equal(t, "Try again later.", text(t, res))
		assert.Equal(t, map[string]any{"code": "busy", "message": "Try again later.", "retry": float64(5)},
			structured(t, res)["error"])
	})

	t.Run("hide an unexpected error behind an internal error", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		_, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: "crash"})

		require.Error(t, err)
		assert.Equal(t, int64(jsonrpc.CodeInternalError), wireCode(t, err))
		assert.NotContains(t, err.Error(), "hunter2")
	})

	t.Run("refuse an input that is not an object", func(t *testing.T) {
		t.Parallel()
		s := mcp.New(mcp.ServerOptions{})

		err := mcp.Tool(s, mcp.ToolDef{Name: "bad"}, func(context.Context, string) (string, error) { return "", nil })

		require.ErrorIs(t, err, mcp.ErrInvalidDefinition)
	})

	t.Run("refuse an input type with no schema", func(t *testing.T) {
		t.Parallel()
		s := mcp.New(mcp.ServerOptions{})

		err := mcp.Tool(s, mcp.ToolDef{Name: "bad"}, func(context.Context, map[int]string) (string, error) {
			return "", nil
		})

		require.ErrorIs(t, err, mcp.ErrInvalidDefinition)
	})

	t.Run("refuse an output type with no schema", func(t *testing.T) {
		t.Parallel()
		s := mcp.New(mcp.ServerOptions{})

		err := mcp.Tool(s, mcp.ToolDef{Name: "bad"}, func(context.Context, struct{}) (chan int, error) {
			return nil, nil
		})

		require.ErrorIs(t, err, mcp.ErrInvalidDefinition)
	})

	t.Run("fail a call whose output cannot be encoded", func(t *testing.T) {
		t.Parallel()
		s := mcp.New(mcp.ServerOptions{Errors: http.Errors{}})
		require.NoError(t, mcp.Tool(s, mcp.ToolDef{Name: "nan"}, func(context.Context, struct{}) (float64, error) {
			return math.NaN(), nil
		}))
		other := newHTTP(t, s)
		cs := connect(t, other.URL+"/mcp", "tok_", nil)

		_, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: "nan"})

		require.Error(t, err)
	})
}
