/*
|--------------------------------------------------------------------------
| Tools
|--------------------------------------------------------------------------
|
| A tool is a typed function. Its input struct is the JSON schema agents
| see and the schema every call is checked against before the function
| runs, so a handler only ever receives input that fits. Its output is
| sent both as structured JSON for programs and as text for the model.
|
| A tool fails the way the rest of the application does: by returning a
| fault. The agent gets the fault's code and message as a tool error it
| can act on, while anything unexpected stays a bare internal error.
|
*/

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/http"
)

/*
ToolDef describes a tool.

Name identifies it, Title names it for people and Description tells
agents when to use it and what it returns. Toolset is the toolset that
must be enabled to see it, none for an always enabled tool. Abilities
lists what the caller's credential must carry. Confirm makes every call
pass the server's confirmation hook first. ReadOnly and Destructive are
hints shown to clients.
*/
type ToolDef struct {
	Name        string
	Title       string
	Description string
	Toolset     string
	Abilities   []string
	Confirm     bool
	ReadOnly    bool
	Destructive bool
}

/*
Texter is a tool output that writes the text the model reads. Outputs
that do not implement it are sent as their JSON.
*/
type Texter interface {
	/*
		Text returns the output as text for the model.
	*/
	Text() string
}

/*
prepared is a call whose input passed the schema: the decoded input and
the tool's function bound to it.
*/
type prepared struct {
	input any
	run   func(ctx context.Context) (any, error)
}

/*
Tool registers the function as the tool. The function's input type
must be a struct or a map and becomes the tool's input schema; its
output type becomes the output schema.
*/
func Tool[I, O any](s *Server, def ToolDef, fn func(ctx context.Context, in I) (O, error)) error {
	input, err := jsonschema.For[I](nil)
	if err != nil {
		return fmt.Errorf("%w: tool %s input: %w", ErrInvalidDefinition, def.Name, err)
	}

	if input.Type != "object" {
		return fmt.Errorf("%w: tool %s input must be an object", ErrInvalidDefinition, def.Name)
	}

	resolved, err := input.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return fmt.Errorf("%w: tool %s input: %w", ErrInvalidDefinition, def.Name, err)
	}

	output, err := jsonschema.For[O](nil)
	if err != nil {
		return fmt.Errorf("%w: tool %s output: %w", ErrInvalidDefinition, def.Name, err)
	}

	prepare := func(raw json.RawMessage) (prepared, error) {
		in, err := decode[I](resolved, raw)
		if err != nil {
			return prepared{}, err
		}

		return prepared{input: in, run: func(ctx context.Context) (any, error) { return fn(ctx, in) }}, nil
	}

	s.addTool(def, input, output, prepare)

	return nil
}

/*
addTool registers the tool with the protocol server behind the checks
every call goes through.
*/
func (s *Server) addTool(
	def ToolDef, input, output *jsonschema.Schema, prepare func(json.RawMessage) (prepared, error),
) {
	a := access{toolset: def.Toolset, abilities: def.Abilities}
	s.register(s.tools, def.Name, a)

	tool := &sdk.Tool{
		Name:         def.Name,
		Title:        def.Title,
		Description:  def.Description,
		InputSchema:  input,
		OutputSchema: output,
		Annotations:  &sdk.ToolAnnotations{ReadOnlyHint: def.ReadOnly, DestructiveHint: &def.Destructive},
	}

	s.sdk.AddTool(tool, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		if !enabled(ctx, a) {
			return nil, invalidParams("unknown tool %q", def.Name)
		}

		if ability := missingAbility(ctx, a); ability != "" {
			return s.failure(ctx, auth.ErrMissingAbility.With("required_ability", ability))
		}

		call, err := prepare(req.Params.Arguments)
		if err != nil {
			return nil, invalidParams("invalid arguments: %v", err)
		}

		if def.Confirm {
			if res, err := s.confirmation(ctx, req, def.Name, call.input); res != nil || err != nil {
				return res, err
			}
		}

		out, err := call.run(ctx)
		if err != nil {
			return s.failure(ctx, err)
		}

		return success(out)
	})
}

/*
decode checks the raw arguments against the input schema, filling in
its defaults, and decodes them into the input type. Missing arguments
are an empty object.
*/
func decode[I any](schema *jsonschema.Resolved, raw json.RawMessage) (I, error) {
	var in I

	args := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return in, fmt.Errorf("decode arguments: %w", err)
		}
	}

	if err := schema.ApplyDefaults(&args); err != nil {
		return in, fmt.Errorf("apply defaults: %w", err)
	}

	if err := schema.Validate(args); err != nil {
		return in, fmt.Errorf("validate arguments: %w", err)
	}

	data, err := json.Marshal(args)
	if err != nil {
		return in, fmt.Errorf("encode arguments: %w", err)
	}

	if err := json.Unmarshal(data, &in); err != nil {
		return in, fmt.Errorf("decode arguments: %w", err)
	}

	return in, nil
}

/*
success returns the result of a tool that ran: the output as structured
JSON, and as text through Texter or else as the same JSON.
*/
func success(out any) (*sdk.CallToolResult, error) {
	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode tool output: %w", err)
	}

	text := string(data)
	if texter, ok := out.(Texter); ok {
		text = texter.Text()
	}

	return &sdk.CallToolResult{
		Content:           []sdk.Content{&sdk.TextContent{Text: text}},
		StructuredContent: json.RawMessage(data),
	}, nil
}

/*
failure turns a tool's error into what the agent receives. A fault
becomes a tool error carrying its code, message and details, so the
agent can correct itself; any other error is reported and becomes an
internal protocol error that reveals nothing.
*/
func (s *Server) failure(ctx context.Context, err error) (*sdk.CallToolResult, error) {
	res := s.errors.Resolve(ctx, err)
	if res.Status >= http.StatusInternalServerError {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: res.Message}
	}

	return &sdk.CallToolResult{
		Content:           []sdk.Content{&sdk.TextContent{Text: res.Message}},
		StructuredContent: map[string]any{"error": res},
		IsError:           true,
	}, nil
}

/*
invalidParams returns the protocol error for a request naming something
that does not exist or carrying arguments that do not fit.
*/
func invalidParams(format string, args ...any) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: fmt.Sprintf(format, args...)}
}
