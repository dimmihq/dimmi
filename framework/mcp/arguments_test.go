package mcp_test

import (
	"context"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
)

func TestToolArguments(t *testing.T) {
	t.Parallel()

	s := mcp.New(mcp.ServerOptions{Name: "test", Version: "1.0.0", Errors: http.Errors{}})
	input := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"}, "n": map[string]any{"type": "integer"},
		},
		"required": []any{"name"},
	}
	def := mcp.ToolDef{
		Name: "greet", Description: "Greet.",
		Arguments: func(args map[string]any) (map[string]any, error) {
			if who, ok := args["who"]; ok {
				args["name"] = who
				delete(args, "who")
			}

			if _, ok := args["name"]; !ok {
				return nil, explained{}
			}

			return args, nil
		},
		Invalid: func(error) error { return explained{} },
	}
	require.NoError(t, mcp.SchemaTool(s, def, input, func(_ context.Context, args map[string]any) (any, error) {
		return map[string]any{"hello": args["name"]}, nil
	}))
	ts := newHTTP(t, s)

	t.Run("read the arguments before they are checked", func(t *testing.T) {
		t.Parallel()
		res := call(t, connect(t, ts.URL+"/mcp", "tok_", nil), "greet", map[string]any{"who": "Ana"})

		assert.Equal(t, map[string]any{"hello": "Ana"}, structured(t, res))
	})

	t.Run("answer with the tool's own words when it refuses the arguments", func(t *testing.T) {
		t.Parallel()
		res, err := connect(t, ts.URL+"/mcp", "tok_", nil).CallTool(t.Context(),
			&sdk.CallToolParams{Name: "greet", Arguments: map[string]any{}})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Equal(t, "Not found. Search first.", text(t, res))
	})

	t.Run("answer with the tool's own words when the schema refuses them", func(t *testing.T) {
		t.Parallel()
		res, err := connect(t, ts.URL+"/mcp", "tok_", nil).CallTool(t.Context(),
			&sdk.CallToolParams{Name: "greet", Arguments: map[string]any{"name": "Ana", "n": "many"}})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Equal(t, "Not found. Search first.", text(t, res))
	})
}
