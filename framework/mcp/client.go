/*
|--------------------------------------------------------------------------
| MCP Client
|--------------------------------------------------------------------------
|
| The application calls tools on MCP servers run by others, at URLs its
| users supply. Those URLs are untrusted, so the client only ever goes
| out through the application's guarded HTTP client, which refuses
| private addresses. Every call is bounded by the timeout so a slow
| server cannot hold the caller.
|
*/

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	stdhttp "net/http"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
ClientOptions configure a connection to a remote server.

Name and Version identify the application to the server. HTTP is the
guarded client every request goes through. Headers are added to every
request, such as the server's API key. Timeout bounds connecting and
each call; zero leaves them to the caller's context.
*/
type ClientOptions struct {
	Name    string
	Version string
	HTTP    *httpclient.Client
	Headers http.Header
	Timeout time.Duration
}

/*
Client is a connection to a remote server.
*/
type Client struct {
	session *sdk.ClientSession
	timeout time.Duration
}

/*
ToolInfo describes a tool a remote server offers. InputSchema is the
JSON schema of its arguments.
*/
type ToolInfo struct {
	Name        string
	Title       string
	Description string
	InputSchema json.RawMessage
}

/*
ToolResult is what a remote tool returned. Text joins its text content,
Structured is its structured content when it sent any, and IsError
reports that the tool failed.
*/
type ToolResult struct {
	Text       string
	Structured json.RawMessage
	IsError    bool
}

/*
headerTransport adds fixed headers to every request.
*/
type headerTransport struct {
	base    stdhttp.RoundTripper
	headers http.Header
}

/*
RoundTrip sends a copy of the request carrying the headers.
*/
func (t headerTransport) RoundTrip(req *stdhttp.Request) (*stdhttp.Response, error) {
	req = req.Clone(req.Context())

	maps.Copy(req.Header, t.headers)

	return t.base.RoundTrip(req)
}

/*
Connect connects to the server at the endpoint.
*/
func Connect(ctx context.Context, endpoint string, opts ClientOptions) (*Client, error) {
	std := *opts.HTTP.StdClient()
	if len(opts.Headers) > 0 {
		std.Transport = headerTransport{base: std.Transport, headers: opts.Headers}
	}

	client := sdk.NewClient(
		&sdk.Implementation{Name: opts.Name, Version: opts.Version},
		&sdk.ClientOptions{Capabilities: &sdk.ClientCapabilities{}},
	)

	c := &Client{timeout: opts.Timeout}

	ctx, cancel := c.bound(ctx)
	defer cancel()

	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           &std,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to mcp server %s: %w", endpoint, err)
	}

	c.session = session

	return c, nil
}

/*
ListTools returns a page of the server's tools, starting at the cursor,
and the cursor of the next page, "" after the last.
*/
func (c *Client) ListTools(ctx context.Context, cursor string) ([]ToolInfo, string, error) {
	ctx, cancel := c.bound(ctx)
	defer cancel()

	res, err := c.session.ListTools(ctx, &sdk.ListToolsParams{Cursor: cursor})
	if err != nil {
		return nil, "", fmt.Errorf("list mcp tools: %w", err)
	}

	tools := make([]ToolInfo, 0, len(res.Tools))
	for _, tool := range res.Tools {
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, "", fmt.Errorf("encode schema of mcp tool %s: %w", tool.Name, err)
		}

		tools = append(tools, ToolInfo{
			Name: tool.Name, Title: tool.Title, Description: tool.Description, InputSchema: schema,
		})
	}

	return tools, res.NextCursor, nil
}

/*
CallTool calls the tool with the arguments, any value that encodes to a
JSON object. A tool that ran and failed is a result with IsError set,
not an error.
*/
func (c *Client) CallTool(ctx context.Context, name string, args any) (ToolResult, error) {
	ctx, cancel := c.bound(ctx)
	defer cancel()

	res, err := c.session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return ToolResult{}, fmt.Errorf("call mcp tool %s: %w", name, err)
	}

	texts := []string{}
	for _, content := range res.Content {
		if text, ok := content.(*sdk.TextContent); ok {
			texts = append(texts, text.Text)
		}
	}

	out := ToolResult{Text: strings.Join(texts, "\n"), IsError: res.IsError}

	if res.StructuredContent != nil {
		out.Structured, err = json.Marshal(res.StructuredContent)
		if err != nil {
			return ToolResult{}, fmt.Errorf("encode result of mcp tool %s: %w", name, err)
		}
	}

	return out, nil
}

/*
Close ends the connection.
*/
func (c *Client) Close() error {
	if err := c.session.Close(); err != nil {
		return fmt.Errorf("close mcp connection: %w", err)
	}

	return nil
}

/*
bound applies the timeout to the context when there is one.
*/
func (c *Client) bound(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, c.timeout)
}
