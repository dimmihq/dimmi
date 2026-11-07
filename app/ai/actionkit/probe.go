/*
|--------------------------------------------------------------------------
| Probes And Dry Runs
|--------------------------------------------------------------------------
|
| An admin checks a connection before trusting it and sees exactly what
| an action would send before it sends anything. A connection probe
| reaches the API's base URL with its credential, or lists an MCP
| server's tools, and clears or sets the connection's error status. A
| dry run renders the request, credential masked, and sends nothing.
|
*/

package actionkit

import (
	"context"
	"encoding/json"
	"maps"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
probeMaxBody caps the body a probe reads.
*/
const probeMaxBody = 64 << 10

/*
ProbeResult is how a connection probe went: ok or error, why, the
upstream status, how long it took and, for an MCP server, how many
tools it offers.
*/
type ProbeResult struct {
	Status     string
	Reason     string
	HTTPStatus int
	Latency    time.Duration
	ToolsCount int
}

/*
Probe checks a connection and records what it found on the
connection's status.
*/
func (k *Kit) Probe(ctx context.Context, conn models.AiActionConnection, path string) (ProbeResult, error) {
	started := k.Clock.Now()
	var res ProbeResult
	var out Outcome
	if conn.Kind == "mcp" {
		res, out = k.probeMCP(ctx, conn)
	} else {
		res, out = k.probeHTTP(ctx, conn, path)
	}
	res.Latency = k.Clock.Now().Sub(started)

	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		call := models.AiActionCall{WorkspaceID: conn.WorkspaceID, ConnectionID: conn.ID}
		return k.health(ctx, models.New(tx), call, out)
	})
	return res, err
}

/*
probeHTTP reaches the base URL, or a path under it, with the credential.
Any answer below 500 but a refused credential means it works.
*/
func (k *Kit) probeHTTP(ctx context.Context, conn models.AiActionConnection, path string) (ProbeResult, Outcome) {
	creds, err := OpenCredentials(ctx, k.Keys, conn)
	if err != nil {
		return ProbeResult{Status: "error", Reason: "credentials"}, Outcome{}
	}
	u, err := targetURL(conn.BaseUrl, "/"+strings.TrimLeft(path, "/"))
	if err != nil {
		return ProbeResult{Status: "error", Reason: "url_escape"}, Outcome{}
	}
	auth := applyAuth(conn, creds)
	q := u.Query()
	for name := range auth.query {
		q.Set(name, auth.query.Get(name))
	}
	u.RawQuery = q.Encode()

	r := k.HTTP.New().WithoutRedirects().MaxBody(probeMaxBody).Timeout(msDuration(conn.TimeoutMs)).
		WithHeader("User-Agent", userAgent)
	headers := defaultHeaders(conn)
	for name := range auth.headers {
		headers[name] = auth.headers.Get(name)
	}
	for name, value := range headers {
		r = r.WithHeader(name, value)
	}
	resp, err := r.Get(ctx, u.String())
	if err != nil {
		out := transportFailure(Outcome{}, err)
		return ProbeResult{Status: "error", Reason: probeReason(out)}, out
	}
	res := ProbeResult{Status: "ok", HTTPStatus: resp.Status}
	out := Outcome{Reached: true, HTTPStatus: resp.Status}
	switch {
	case resp.Status == 401 || resp.Status == 403:
		res.Status, res.Reason, out.AuthFailure = "error", "auth", true
	case resp.Status >= 500:
		res.Status, res.Reason, out.Reached, out.ErrorCode = "error", "unreachable", false, "unavailable"
	}
	return res, out
}

/*
probeMCP opens a session and lists the server's tools.
*/
func (k *Kit) probeMCP(ctx context.Context, conn models.AiActionConnection) (ProbeResult, Outcome) {
	tools, err := k.ListTools(ctx, conn)
	if err != nil {
		out := mcpFailure(Outcome{}, err)
		return ProbeResult{Status: "error", Reason: probeReason(out)}, out
	}
	return ProbeResult{Status: "ok", ToolsCount: len(tools)}, Outcome{Reached: true}
}

/*
probeReason names why a probe failed.
*/
func probeReason(out Outcome) string {
	if out.Blocked {
		return "host_blocked"
	}
	return "unreachable"
}

/*
DryRun renders what the target would send, the credential masked, and
sends nothing.
*/
func DryRun(t Target) (json.RawMessage, error) {
	if t.Spec.Kind == KindMCPTool {
		args := maps.Clone(t.Input)
		if args == nil {
			args = map[string]any{}
		}
		return json.Marshal(map[string]any{"tool": t.Spec.MCPToolName, "arguments": args})
	}
	req, err := render(t)
	if err != nil {
		return nil, err
	}
	return req.trace(), nil
}

/*
RenderCode maps a rendering failure to the error code a test shows.
*/
func RenderCode(err error) string {
	return failureCode(err)
}
