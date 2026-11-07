/*
|--------------------------------------------------------------------------
| MCP Calls
|--------------------------------------------------------------------------
|
| A remote MCP server is called over Streamable HTTP through the same
| guarded client as any API. The model's arguments and the admin's bound
| arguments go to tools/call; structured content is mapped like a JSON
| body, plain text is passed on short, and a tool that reports an error
| is a refusal the model can explain. Each call opens its own session,
| so nothing a server sends lingers between customers.
|
*/

package actionkit

import (
	"context"
	"encoding/json"
	"errors"
	"maps"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
	"github.com/dimmihq/dimmi/framework/mcp"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionresult"
	"github.com/dimmihq/dimmi/app/support/actiontemplate"
)

/*
maxSyncedTools caps the tools one sync reads from a server.
*/
const maxSyncedTools = 200

/*
connect opens a session with the connection's server, its secret and
static headers on every request.
*/
func (k *Kit) connect(ctx context.Context, conn models.AiActionConnection, creds Credentials,
	extra fhttp.Header,
) (*mcp.Client, error) {
	headers := fhttp.Header{}
	for name, value := range defaultHeaders(conn) {
		headers.Set(name, value)
	}
	auth := applyAuth(conn, creds)
	maps.Copy(headers, auth.headers)
	maps.Copy(headers, extra)
	endpoint := conn.BaseUrl
	if len(auth.query) > 0 {
		endpoint += "?" + auth.query.Encode()
	}
	timeout := defaultTimeout
	if conn.TimeoutMs > 0 {
		timeout = msDuration(conn.TimeoutMs)
	}
	return mcp.Connect(ctx, endpoint, mcp.ClientOptions{
		Name: "dimmi-actions", Version: "1", HTTP: k.HTTP, Headers: headers, Timeout: timeout,
	})
}

/*
sendMCP calls the action's tool.
*/
func (k *Kit) sendMCP(ctx context.Context, t Target) Outcome {
	started := k.Clock.Now()
	args, err := mcpArguments(t)
	if err != nil {
		return Outcome{ErrorCode: failureCode(err), Message: err.Error()}
	}
	trace, _ := json.Marshal(map[string]any{"tool": t.Spec.MCPToolName, "arguments": args})
	out := Outcome{Request: trace}

	client, err := k.connect(ctx, t.Conn, t.Creds, callHeaders(t))
	if err != nil {
		out.Latency = k.Clock.Now().Sub(started)
		return mcpFailure(out, err)
	}
	defer func() { _ = client.Close() }()

	res, err := client.CallTool(ctx, t.Spec.MCPToolName, args)
	out.Latency, out.Reached = k.Clock.Now().Sub(started), true
	if err != nil {
		out.ErrorCode, out.Message = "upstream_rejected", "the tool could not be called"
		return out
	}
	return toolOutcome(out, t, res)
}

/*
mcpArguments are the tool's arguments: the model's input with the
admin's bound arguments laid over it.
*/
func mcpArguments(t Target) (map[string]any, error) {
	args := maps.Clone(t.Input)
	if args == nil {
		args = map[string]any{}
	}
	bound, _, err := actiontemplate.ExpandJSON(t.Spec.BoundArguments, t.vars())
	if err != nil {
		return nil, err
	}
	if m, ok := bound.(map[string]any); ok {
		maps.Copy(args, m)
	}
	return args, nil
}

/*
callHeaders name the call to the server; a write also carries its
idempotency key.
*/
func callHeaders(t Target) fhttp.Header {
	h := fhttp.Header{}
	callID := models.PublicID(models.PrefixAiActionCall, t.CallID)
	h.Set("Dimmi-Action-Call", callID)
	if t.Spec.SideEffect == SideEffectWrite {
		h.Set("Idempotency-Key", callID)
	}
	return h
}

/*
toolOutcome maps what the tool returned: structured content mapped like
a JSON body, else its text, and a tool error as a refusal.
*/
func toolOutcome(out Outcome, t Target, res mcp.ToolResult) Outcome {
	out.Response, _ = json.Marshal(map[string]any{
		"is_error": res.IsError, "text": truncateText(res.Text, maxTraceBody), "structured": res.Structured,
	})
	if res.IsError {
		out.ErrorCode, out.Message = "upstream_rejected", truncateText(res.Text, 300)
		return out
	}
	out.Succeeded = true
	out.Result = actionresult.Text(res.Text)
	if len(res.Structured) > 0 {
		if shaped, err := actionresult.Shape(res.Structured, t.Spec.ResponseFields,
			actionresult.DefaultMaxBytes); err == nil {
			out.Result = shaped
		}
	}
	return out
}

/*
mcpFailure maps a session that could not be opened.
*/
func mcpFailure(out Outcome, err error) Outcome {
	if errors.Is(err, safedial.ErrBlocked) || errors.Is(err, httpclient.ErrBlockedHost) {
		out.ErrorCode, out.Blocked = "host_blocked", true
		return out
	}
	out.ErrorCode, out.retryable = "unavailable", true
	return out
}

/*
ListTools reads every tool the connection's server offers, up to the
sync cap.
*/
func (k *Kit) ListTools(ctx context.Context, conn models.AiActionConnection) ([]mcp.ToolInfo, error) {
	creds, err := OpenCredentials(ctx, k.Keys, conn)
	if err != nil {
		return nil, err
	}
	client, err := k.connect(ctx, conn, creds, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	var tools []mcp.ToolInfo
	cursor := ""
	for len(tools) < maxSyncedTools {
		page, next, err := client.ListTools(ctx, cursor)
		if err != nil {
			return nil, err
		}
		tools = append(tools, page...)
		if next == "" {
			break
		}
		cursor = next
	}
	if len(tools) > maxSyncedTools {
		tools = tools[:maxSyncedTools]
	}
	return tools, nil
}

/*
truncateText keeps at most n runes.
*/
func truncateText(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
