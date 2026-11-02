package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mcp"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
metadataURL is where the test server says its protected resource
metadata lives.
*/
const metadataURL = "https://api.example.com/.well-known/oauth-protected-resource/mcp"

/*
tokenUser is the user every test token acts for.
*/
const tokenUser = "019a4a40-0000-7000-8000-000000000001"

/*
resolveToken resolves the test tokens: "tok_" tokens carry the
abilities listed after the prefix, joined by "+", and are bound to a
tenant; "tok_revoked" is refused. Other tokens are not its kind.
*/
func resolveToken(_ *http.Request, token string) (auth.Principal, error) {
	switch {
	case token == "tok_revoked":
		return nil, auth.ErrInvalidToken
	case strings.HasPrefix(token, "tok_"):
		user, err := ids.Parse(tokenUser)
		if err != nil {
			return nil, fmt.Errorf("parse test user: %w", err)
		}

		abilities := strings.Split(strings.TrimPrefix(token, "tok_"), "+")

		return &auth.Identity{User: user, Via: "token", TenantID: "tenant-1", Granted: abilities}, nil
	default:
		return nil, auth.ErrNoCredentials
	}
}

/*
echoInput is the input of the echo tool.
*/
type echoInput struct {
	Message string `json:"message"          jsonschema:"what to repeat"`
	Times   int    `json:"times,omitempty"`
}

/*
echoOutput is the output of the echo tool, which renders its own text.
*/
type echoOutput struct {
	Echo string `json:"echo"`
}

func (o echoOutput) Text() string {
	return "echo: " + o.Echo
}

/*
caller is what the whoami tool answers with.
*/
type caller struct {
	ID     string `json:"id"`
	Tenant string `json:"tenant"`
}

/*
deleteInput is the input of the tool that needs confirmation. Confirm
is the agent saying the person approved.
*/
type deleteInput struct {
	ID      string `json:"id"`
	Confirm bool   `json:"confirm,omitempty"`
}

/*
status is the structured result of the tool that needs confirmation.
*/
type status struct {
	Status string `json:"status"`
}

/*
confirmRecords is the test confirmation policy. A retry carrying an
accepted answer and the state handed out runs; any other answer is a
decline. A client that can ask is asked. Otherwise the agent's own
confirm flag runs the tool, and without it the agent gets a preview.
Deleting "locked" fails.
*/
func confirmRecords(_ context.Context, c mcp.Confirmation) (*mcp.Unconfirmed, error) {
	in, ok := c.Input.(deleteInput)
	if !ok || in.ID == "locked" {
		return nil, fault.New(fault.Locked, "record_locked", "The record is locked.")
	}

	if answer, ok := c.Answers["confirm"]; ok {
		if answer.Action == "accept" && answer.Content["delete"] == true && c.State == "state-"+in.ID {
			return nil, nil
		}

		return &mcp.Unconfirmed{Message: "Not deleted.", Result: status{Status: "declined"}}, nil
	}

	if c.CanAsk {
		return &mcp.Unconfirmed{State: "state-" + in.ID, Ask: &mcp.Question{
			ID:      "confirm",
			Message: "Delete record " + in.ID + "?",
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"delete": map[string]any{"type": "boolean"}},
				"required":   []string{"delete"},
			},
		}}, nil
	}

	if in.Confirm {
		return nil, nil
	}

	return &mcp.Unconfirmed{
		Message: "Not deleted. Ask the person, then call again with confirm: true.",
		Result:  status{Status: "confirmation_required"},
	}, nil
}

/*
newServer returns the test server with its tools, resources and prompts.
*/
func newServer(t *testing.T, confirm mcp.Confirmer) *mcp.Server {
	t.Helper()

	s := mcp.New(mcp.ServerOptions{
		Name: "test", Version: "1.0.0", Instructions: "Use the tools.", Errors: http.Errors{}, Confirm: confirm,
	})

	require.NoError(t, mcp.Tool(s, mcp.ToolDef{Name: "echo", Description: "Repeat a message.", ReadOnly: true},
		func(_ context.Context, in echoInput) (echoOutput, error) {
			return echoOutput{Echo: strings.Repeat(in.Message, max(in.Times, 1))}, nil
		}))

	require.NoError(t, mcp.Tool(s, mcp.ToolDef{Name: "whoami", Description: "Name the caller."}, whoami))

	require.NoError(t, mcp.Tool(s, mcp.ToolDef{Name: "fail", Description: "Fail with a fault."},
		func(context.Context, struct{}) (status, error) {
			return status{}, fault.New(fault.Conflict, "busy", "Try again later.").With("retry", 5)
		}))

	require.NoError(t, mcp.Tool(s, mcp.ToolDef{Name: "crash", Description: "Fail unexpectedly."},
		func(context.Context, struct{}) (status, error) {
			return status{}, errors.New("database password is hunter2")
		}))

	require.NoError(t, mcp.Tool(s, mcp.ToolDef{
		Name: "records_read", Description: "Read records.", Toolset: "records", Abilities: []string{"records:read"},
	}, func(context.Context, struct{}) (status, error) { return status{Status: "read"}, nil }))

	require.NoError(t, mcp.Tool(s, mcp.ToolDef{
		Name: "records_delete", Description: "Delete a record.", Toolset: "records",
		Abilities: []string{"records:write"}, Confirm: true, Destructive: true,
	}, func(_ context.Context, in deleteInput) (status, error) {
		return status{Status: "deleted " + in.ID}, nil
	}))

	require.NoError(t, mcp.Tool(s, mcp.ToolDef{Name: "reports_view", Description: "View reports.", Toolset: "reports"},
		func(context.Context, struct{}) (status, error) { return status{Status: "viewed"}, nil }))

	addContent(t, s)

	return s
}

/*
whoami answers with the caller in the context and its tenant.
*/
func whoami(ctx context.Context, _ struct{}) (caller, error) {
	p, ok := auth.From(ctx)
	if !ok {
		return caller{}, nil
	}

	tenant, _ := tenancy.From(ctx)

	return caller{ID: p.ID().String(), Tenant: tenant}, nil
}

/*
addContent registers the test resources and prompts.
*/
func addContent(t *testing.T, s *mcp.Server) {
	t.Helper()

	require.NoError(t, mcp.Resource(s, mcp.ResourceDef{URI: "files://readme", Name: "readme", MIMEType: "text/plain"},
		func(context.Context, map[string]string) (string, error) { return "Read me.", nil }))

	require.NoError(t, mcp.Resource(s, mcp.ResourceDef{
		URI: "files://docs/{name}", Name: "doc", MIMEType: "text/markdown", Abilities: []string{"files:read"},
	}, func(_ context.Context, params map[string]string) (string, error) {
		if params["name"] == "missing" {
			return "", fault.New(fault.NotFound, "doc_not_found", "No such doc.")
		}

		if params["name"] == "broken" {
			return "", errors.New("disk on fire")
		}

		return "# " + params["name"], nil
	}))

	mcp.Prompt(s, mcp.PromptDef{
		Name: "greet", Description: "Greet someone.", Arguments: []mcp.Argument{{Name: "name", Required: true}},
	}, func(_ context.Context, args map[string]string) (string, error) {
		if args["name"] == "" {
			return "", fault.New(fault.Invalid, "name_required", "Name someone to greet.")
		}

		return "Say hello to " + args["name"] + ".", nil
	})

	mcp.Prompt(s, mcp.PromptDef{Name: "report", Description: "Write a report.", Toolset: "reports"},
		func(context.Context, map[string]string) (string, error) { return "Write the report.", nil })
}

/*
newHTTP serves the server on a test HTTP server through the router: at
/mcp behind its own guard, at /routed behind the route's auth
middleware, and at /open with no authentication at all.
*/
func newHTTP(t *testing.T, s *mcp.Server) *httptest.Server {
	t.Helper()

	guards := auth.NewGuards(http.Errors{})
	guards.Register("token", auth.BearerGuard(auth.TokenResolverFunc(resolveToken)))

	kernel := http.NewKernel()
	kernel.Alias("auth", guards.Middleware)

	opts := mcp.Options{
		MaxBody:          64 << 10,
		Auth:             guards,
		ResourceMetadata: metadataURL,
		AllowedOrigins:   []string{"https://app.example.com"},
	}

	r := routing.New(routing.Options{Middleware: kernel})
	r.Handle(http.MethodPost, "/mcp", "mcp", s.HTTPHandler(opts))
	r.Handle(http.MethodGet, "/mcp", "mcp.stream", s.HTTPHandler(opts))
	r.Handle(http.MethodPost, "/routed", "mcp.routed", s.HTTPHandler(mcp.Options{})).Middleware("auth")
	r.Handle(http.MethodPost, "/open", "mcp.open", s.HTTPHandler(mcp.Options{ResourceMetadata: metadataURL}))
	r.Handle(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", "mcp.metadata",
		mcp.ProtectedResourceHandler(mcp.ResourceMetadata{
			Resource:            "https://api.example.com/mcp",
			AuthorizationServer: "https://auth.example.com",
			Scopes:              []string{"records:read", "records:write"},
			Name:                "Test",
		}))

	h, err := r.Handler()
	require.NoError(t, err)

	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	return ts
}

/*
bearer adds a bearer token to every request.
*/
type bearer struct {
	base  stdhttp.RoundTripper
	token string
}

func (b bearer) RoundTrip(req *stdhttp.Request) (*stdhttp.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+b.token)

	return b.base.RoundTrip(req)
}

/*
connect opens a protocol client session to the URL with the token.
*/
func connect(t *testing.T, url, token string, opts *sdk.ClientOptions) *sdk.ClientSession {
	t.Helper()

	hc, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	client := *hc.StdClient()
	client.Transport = bearer{base: client.Transport, token: token}

	cs, err := sdk.NewClient(&sdk.Implementation{Name: "agent", Version: "1.0.0"}, opts).Connect(t.Context(),
		&sdk.StreamableClientTransport{Endpoint: url, HTTPClient: &client, DisableStandaloneSSE: true, MaxRetries: -1},
		nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	return cs
}

/*
toolNames lists the names of the tools the session sees.
*/
func toolNames(t *testing.T, cs *sdk.ClientSession) []string {
	t.Helper()

	res, err := cs.ListTools(t.Context(), nil)
	require.NoError(t, err)

	names := []string{}
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}

	return names
}

/*
call calls the tool and returns its result.
*/
func call(t *testing.T, cs *sdk.ClientSession, name string, args any) *sdk.CallToolResult {
	t.Helper()

	res, err := cs.CallTool(t.Context(), &sdk.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)

	return res
}

/*
structured decodes a result's structured content.
*/
func structured(t *testing.T, res *sdk.CallToolResult) map[string]any {
	t.Helper()

	data, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))

	return out
}

/*
errorCode returns the code of a tool error's structured content.
*/
func errorCode(t *testing.T, res *sdk.CallToolResult) any {
	t.Helper()

	failure, ok := structured(t, res)["error"].(map[string]any)
	require.True(t, ok)

	return failure["code"]
}

/*
text returns a result's first text content.
*/
func text(t *testing.T, res *sdk.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, res.Content)

	return contentText(t, res.Content[0])
}

/*
contentText returns the text of a text content.
*/
func contentText(t *testing.T, content sdk.Content) string {
	t.Helper()

	text, ok := content.(*sdk.TextContent)
	require.True(t, ok)

	return text.Text
}

/*
wireCode returns the protocol error code of a failed call.
*/
func wireCode(t *testing.T, err error) int64 {
	t.Helper()

	wire, ok := errors.AsType[*jsonrpc.Error](err)
	require.True(t, ok, "not a protocol error: %v", err)

	return wire.Code
}
