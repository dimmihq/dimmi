package mcp_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mcp"
)

func TestResource(t *testing.T) {
	t.Parallel()

	ts := newHTTP(t, newServer(t, nil))

	t.Run("list only the resources the caller may see", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		resources, err := cs.ListResources(t.Context(), nil)
		require.NoError(t, err)
		templates, err := cs.ListResourceTemplates(t.Context(), nil)
		require.NoError(t, err)

		require.Len(t, resources.Resources, 1)
		assert.Equal(t, "files://readme", resources.Resources[0].URI)
		assert.Empty(t, templates.ResourceTemplates)
	})

	t.Run("list templates to a caller holding their abilities", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_files:read", nil)

		templates, err := cs.ListResourceTemplates(t.Context(), nil)
		require.NoError(t, err)

		require.Len(t, templates.ResourceTemplates, 1)
		assert.Equal(t, "files://docs/{name}", templates.ResourceTemplates[0].URITemplate)
	})

	t.Run("read a resource", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		res, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "files://readme"})
		require.NoError(t, err)

		require.Len(t, res.Contents, 1)
		assert.Equal(t, "Read me.", res.Contents[0].Text)
		assert.Equal(t, "text/plain", res.Contents[0].MIMEType)
	})

	t.Run("read a resource of a template with the values its variables matched", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_files:read", nil)

		res, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "files://docs/setup"})
		require.NoError(t, err)

		assert.Equal(t, "# setup", res.Contents[0].Text)
	})

	t.Run("answer a resource the caller may not see as not found", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		_, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "files://docs/setup"})

		require.Error(t, err)
		assert.Equal(t, int64(jsonrpc.CodeInvalidParams), wireCode(t, err))
	})

	t.Run("answer a fault with its message", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_files:read", nil)

		_, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "files://docs/missing"})

		require.Error(t, err)
		assert.Equal(t, int64(jsonrpc.CodeInvalidParams), wireCode(t, err))
		assert.Contains(t, err.Error(), "No such doc.")
	})

	t.Run("hide an unexpected error behind an internal error", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_files:read", nil)

		_, err := cs.ReadResource(t.Context(), &sdk.ReadResourceParams{URI: "files://docs/broken"})

		require.Error(t, err)
		assert.Equal(t, int64(jsonrpc.CodeInternalError), wireCode(t, err))
		assert.NotContains(t, err.Error(), "fire")
	})

	t.Run("refuse a relative URI", func(t *testing.T) {
		t.Parallel()

		err := mcp.Resource(mcp.New(mcp.ServerOptions{}), mcp.ResourceDef{URI: "readme"}, readme)

		require.ErrorIs(t, err, mcp.ErrInvalidDefinition)
	})

	t.Run("refuse a malformed template", func(t *testing.T) {
		t.Parallel()

		err := mcp.Resource(mcp.New(mcp.ServerOptions{}), mcp.ResourceDef{URI: "files://{name"}, readme)

		require.ErrorIs(t, err, mcp.ErrInvalidDefinition)
	})
}

func TestPrompt(t *testing.T) {
	t.Parallel()

	ts := newHTTP(t, newServer(t, nil))

	t.Run("list only the prompts of enabled toolsets", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		res, err := cs.ListPrompts(t.Context(), nil)
		require.NoError(t, err)

		require.Len(t, res.Prompts, 1)
		assert.Equal(t, "greet", res.Prompts[0].Name)
		assert.True(t, res.Prompts[0].Arguments[0].Required)
	})

	t.Run("produce the user message from the arguments", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		params := &sdk.GetPromptParams{Name: "greet", Arguments: map[string]string{"name": "Ada"}}
		res, err := cs.GetPrompt(t.Context(), params)
		require.NoError(t, err)

		require.Len(t, res.Messages, 1)
		assert.Equal(t, sdk.Role("user"), res.Messages[0].Role)
		assert.Equal(t, "Say hello to Ada.", contentText(t, res.Messages[0].Content))
	})

	t.Run("answer a fault with its message", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		params := &sdk.GetPromptParams{Name: "greet", Arguments: map[string]string{"name": ""}}
		_, err := cs.GetPrompt(t.Context(), params)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Name someone to greet.")
	})

	t.Run("treat a prompt of a toolset the caller did not enable as unknown", func(t *testing.T) {
		t.Parallel()
		cs := connect(t, ts.URL+"/mcp", "tok_", nil)

		_, err := cs.GetPrompt(t.Context(), &sdk.GetPromptParams{Name: "report"})

		require.Error(t, err)
		assert.Equal(t, int64(jsonrpc.CodeInvalidParams), wireCode(t, err))
	})
}

func readme(context.Context, map[string]string) (string, error) {
	return "Read me.", nil
}
