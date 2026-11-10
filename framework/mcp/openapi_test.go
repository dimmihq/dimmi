package mcp_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
apiDoc is a small API document: a list with a query filter, a show by
path parameter, an update whose body sits beside the parameter, one
whose body field shares a parameter's name, a self-referring schema and
an operation without an id.
*/
const apiDoc = `{
  "paths": {
    "/items": {
      "get": {"operationId": "items.index", "summary": "List items", "x-abilities": ["items:read"],
        "parameters": [{"in": "query", "name": "status", "description": "Only these.",
          "schema": {"type": "string", "enum": ["open", "closed"]}},
          {"in": "header", "name": "X-Trace", "schema": {"type": "string"}}]},
      "post": {"summary": "No id"}
    },
    "/items/{item}": {
      "patch": {"operationId": "items.update",
        "parameters": [{"in": "path", "name": "item", "required": true, "schema": {"type": "string"}}],
        "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Change"}}}}},
      "put": {"operationId": "items.replace",
        "parameters": [{"in": "path", "name": "item", "required": true, "schema": {"type": "string"}}],
        "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Clash"}}}}}
    },
    "/files": {
      "post": {"operationId": "files.store",
        "requestBody": {"content": {"multipart/form-data": {"schema": {"type": "object"}}}}}
    },
    "/trees": {
      "post": {"operationId": "trees.store",
        "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Tree"}}}}}
    }
  },
  "components": {"schemas": {
    "Change": {"type": "object", "required": ["name"], "properties": {
      "$schema": {"type": "string"}, "name": {"type": "string"},
      "tags": {"type": "array", "items": {"type": "string"}}}},
    "Clash": {"type": "object", "required": ["item"], "properties": {"item": {"type": "string"}}},
    "Tree": {"type": "object", "properties": {"child": {"$ref": "#/components/schemas/Tree"},
      "meta": {"type": "object", "additionalProperties": {"type": "string"}}}}
  }}
}`

func operations(t *testing.T) map[string]mcp.Operation {
	t.Helper()
	ops, err := mcp.Operations([]byte(apiDoc), nil)
	require.NoError(t, err)

	out := map[string]mcp.Operation{}
	for _, op := range ops {
		out[op.ID] = op
	}

	return out
}

func TestOperations(t *testing.T) {
	t.Parallel()

	t.Run("read every operation with an id, in order", func(t *testing.T) {
		t.Parallel()
		ops, err := mcp.Operations([]byte(apiDoc), nil)
		require.NoError(t, err)

		ids := make([]string, len(ops))
		for i, op := range ops {
			ids[i] = op.ID
		}

		assert.Equal(t, []string{"files.store", "items.index", "items.replace", "items.update", "trees.store"}, ids)
	})

	t.Run("keep only the operations asked for", func(t *testing.T) {
		t.Parallel()
		ops, err := mcp.Operations([]byte(apiDoc), func(op mcp.Operation) bool { return len(op.Abilities) > 0 })
		require.NoError(t, err)

		require.Len(t, ops, 1)
		assert.Equal(t, "GET", ops[0].Method)
		assert.Equal(t, "List items", ops[0].Summary)
		assert.Equal(t, []string{"items:read"}, ops[0].Abilities)
	})

	t.Run("refuse a document that is not JSON", func(t *testing.T) {
		t.Parallel()
		_, err := mcp.Operations([]byte("nope"), nil)

		require.ErrorIs(t, err, mcp.ErrInvalidDocument)
	})

	t.Run("put the body's fields beside the parameters", func(t *testing.T) {
		t.Parallel()
		input := operations(t)["items.update"].Input
		props, _ := input["properties"].(map[string]any)

		assert.ElementsMatch(t, []any{"item", "name"}, input["required"])
		assert.Contains(t, props, "tags")
		assert.NotContains(t, props, "$schema")
	})

	t.Run("keep a parameter's description and leave headers out", func(t *testing.T) {
		t.Parallel()
		props, _ := operations(t)["items.index"].Input["properties"].(map[string]any)

		assert.Equal(t, map[string]any{"type": "string", "enum": []any{"open", "closed"}, "description": "Only these."},
			props["status"])
		assert.NotContains(t, props, "X-Trace")
	})

	t.Run("keep a body field named body beside the parameters", func(t *testing.T) {
		t.Parallel()
		ops, err := mcp.Operations([]byte(`{"paths":{"/notes/{note}":{"post":{"operationId":"notes.store",
			"parameters":[{"in":"path","name":"note","required":true,"schema":{"type":"string"}}],
			"requestBody":{"content":{"application/json":{"schema":{"type":"object","required":["body"],
			"properties":{"body":{"type":"string"}}}}}}}}}}`), nil)
		require.NoError(t, err)
		props, _ := ops[0].Input["properties"].(map[string]any)

		assert.Equal(t, map[string]any{"type": "string"}, props["body"])
		assert.ElementsMatch(t, []any{"note", "body"}, ops[0].Input["required"])
	})

	t.Run("put a body under its own argument when a field shares a parameter's name", func(t *testing.T) {
		t.Parallel()
		input := operations(t)["items.replace"].Input
		props, _ := input["properties"].(map[string]any)

		assert.Contains(t, props, "request_body")
		assert.ElementsMatch(t, []any{"item", "request_body"}, input["required"])
	})

	t.Run("mark an operation that takes a file upload", func(t *testing.T) {
		t.Parallel()
		ops := operations(t)

		assert.True(t, ops["files.store"].Upload)
		assert.False(t, ops["items.update"].Upload)
	})

	t.Run("end a schema that refers to itself", func(t *testing.T) {
		t.Parallel()
		data, err := json.Marshal(operations(t)["trees.store"].Input)
		require.NoError(t, err)

		assert.Contains(t, string(data), `"child"`)
		assert.Contains(t, string(data), `"additionalProperties"`)
	})
}

func TestOperationRequest(t *testing.T) {
	t.Parallel()

	t.Run("fill the path and send the rest as the body", func(t *testing.T) {
		t.Parallel()
		e, err := operations(t)["items.update"].Encode(map[string]any{
			"item": "it/1", "name": "Box", "tags": []any{"a"},
		})
		require.NoError(t, err)

		assert.Equal(t, "/items/it%2F1", e.Path)
		assert.Empty(t, e.Query)
		assert.JSONEq(t, `{"name":"Box","tags":["a"]}`, string(e.Body))
	})

	t.Run("send query arguments, lists as repeated values", func(t *testing.T) {
		t.Parallel()
		e, err := operations(t)["items.index"].Encode(map[string]any{"status": []any{"open", 2.5}})
		require.NoError(t, err)

		assert.Equal(t, "/items", e.Path)
		assert.Equal(t, url.Values{"status": {"open", "2.5"}}, e.Query)
		assert.Nil(t, e.Body)
	})

	t.Run("leave out query arguments not given", func(t *testing.T) {
		t.Parallel()
		e, err := operations(t)["items.index"].Encode(map[string]any{"status": nil})
		require.NoError(t, err)

		assert.Empty(t, e.Query)
	})

	t.Run("send a nested body as given", func(t *testing.T) {
		t.Parallel()
		e, err := operations(t)["items.replace"].Encode(map[string]any{
			"item": "it_1", "request_body": map[string]any{"item": "new"},
		})
		require.NoError(t, err)

		assert.JSONEq(t, `{"item":"new"}`, string(e.Body))
	})

	t.Run("send an empty body when nothing is given", func(t *testing.T) {
		t.Parallel()
		e, err := operations(t)["trees.store"].Encode(nil)
		require.NoError(t, err)

		assert.JSONEq(t, `{}`, string(e.Body))
	})

	t.Run("refuse a call missing a path argument", func(t *testing.T) {
		t.Parallel()
		_, err := operations(t)["items.update"].Encode(map[string]any{"name": "Box"})

		require.ErrorContains(t, err, `missing argument "item"`)
	})

	t.Run("write non-text path values as text", func(t *testing.T) {
		t.Parallel()
		e, err := operations(t)["items.update"].Encode(map[string]any{"item": 42.0, "name": "x"})
		require.NoError(t, err)

		assert.Equal(t, "/items/42", e.Path)
		e, err = operations(t)["items.update"].Encode(map[string]any{"item": true, "name": "x"})
		require.NoError(t, err)
		assert.Equal(t, "/items/true", e.Path)
	})
}

/*
explained is an error that explains itself to the agent.
*/
type explained struct{}

func (explained) Error() string { return "explained" }

func (explained) ToolResult() (string, any) {
	return "Not found. Search first.", map[string]any{"error": map[string]any{"code": "not_found"}}
}

func TestSchemaTool(t *testing.T) {
	t.Parallel()

	s := mcp.New(mcp.ServerOptions{Name: "test", Version: "1.0.0", Errors: http.Errors{}})
	input := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"}, "n": map[string]any{"type": "integer", "default": 3},
		},
		"required": []any{"name"},
	}
	require.NoError(t, mcp.SchemaTool(s, mcp.ToolDef{Name: "greet", Description: "Greet.", Idempotent: true},
		input, func(_ context.Context, args map[string]any) (any, error) {
			if args["name"] == "missing" {
				return nil, explained{}
			}

			return map[string]any{"hello": args["name"], "n": args["n"]}, nil
		}))
	ts := newHTTP(t, s)

	t.Run("pass the checked arguments with defaults filled in", func(t *testing.T) {
		t.Parallel()
		res := call(t, connect(t, ts.URL+"/mcp", "tok_", nil), "greet", map[string]any{"name": "Ana"})

		assert.Equal(t, map[string]any{"hello": "Ana", "n": 3.0}, structured(t, res))
	})

	t.Run("describe the schema and hints", func(t *testing.T) {
		t.Parallel()
		res, err := connect(t, ts.URL+"/mcp", "tok_", nil).ListTools(t.Context(), nil)
		require.NoError(t, err)

		require.Len(t, res.Tools, 1)
		assert.True(t, res.Tools[0].Annotations.IdempotentHint)
		assert.Nil(t, res.Tools[0].OutputSchema)
	})

	t.Run("refuse arguments that break the schema", func(t *testing.T) {
		t.Parallel()
		_, err := connect(t, ts.URL+"/mcp", "tok_", nil).CallTool(t.Context(),
			&sdk.CallToolParams{Name: "greet", Arguments: map[string]any{}})

		require.Error(t, err)
	})

	t.Run("send an error that explains itself as written", func(t *testing.T) {
		t.Parallel()
		res := call(t, connect(t, ts.URL+"/mcp", "tok_", nil), "greet", map[string]any{"name": "missing"})

		assert.True(t, res.IsError)
		assert.Equal(t, "Not found. Search first.", text(t, res))
		assert.Equal(t, map[string]any{"error": map[string]any{"code": "not_found"}}, structured(t, res))
	})

	t.Run("refuse a schema that is not an object", func(t *testing.T) {
		t.Parallel()
		err := mcp.SchemaTool(s, mcp.ToolDef{Name: "bad"}, map[string]any{"type": "string"}, nil)

		require.ErrorIs(t, err, mcp.ErrInvalidDefinition)
	})

	t.Run("refuse a schema that does not resolve", func(t *testing.T) {
		t.Parallel()
		err := mcp.SchemaTool(s, mcp.ToolDef{Name: "bad"}, map[string]any{
			"type": "object", "properties": map[string]any{"x": map[string]any{"$ref": "#/nowhere"}},
		}, nil)

		require.ErrorIs(t, err, mcp.ErrInvalidDefinition)
	})

	t.Run("refuse a schema that cannot be encoded", func(t *testing.T) {
		t.Parallel()
		err := mcp.SchemaTool(s, mcp.ToolDef{Name: "bad"}, map[string]any{"type": make(chan int)}, nil)

		require.ErrorIs(t, err, mcp.ErrInvalidDefinition)
	})
}
