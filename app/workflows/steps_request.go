/*
|--------------------------------------------------------------------------
| Calling The Customer's System
|--------------------------------------------------------------------------
|
| A request step calls a URL the team set, through the same guarded
| client as webhooks and AI actions: private and metadata addresses are
| refused when connecting, redirects are never followed, the body is
| capped and the call gives up after ten seconds at most. Variables are
| escaped for where they land, so a customer's words can neither leave
| a JSON string nor add a header. Credentials come only from a sealed
| connection and are put on the request after everything is rendered;
| the trace keeps the status and what was saved, never a body.
|
| A 2xx answer goes on and copies the mapped response values into run
| variables or contact attributes; anything else takes the failure
| path, or goes on when the step has none.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
Request limits: the default and longest wait, and the largest body read.
*/
const (
	requestTimeout     = 8 * time.Second
	requestTimeoutMost = 10 * time.Second
	requestMaxBody     = 256 << 10
)

/*
outbound is a rendered request.
*/
type outbound struct {
	method  string
	url     string
	headers map[string]string
	body    []byte
}

/*
runHTTPRequest calls the URL and saves what the step maps.
*/
func runHTTPRequest(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	h := settings[workflowspec.HTTPRequestSettings](s)
	if st.e.HTTP == nil {
		return requestFailed(s, "unavailable", 0), nil
	}

	req, code, err := st.buildRequest(ctx, h)
	if err != nil || code != "" {
		return requestFailed(s, code, 0), err
	}

	client := st.e.HTTP.New().WithoutRedirects().MaxBody(requestMaxBody).Timeout(timeoutOf(h.Timeout)).
		Accept("application/json")

	resp, err := request{client}.withHeaders(req.headers).send(ctx, req)
	if err != nil {
		return requestFailed(s, transportCode(err), 0), nil
	}

	if resp.Status < 200 || resp.Status >= 300 {
		return requestFailed(s, "http_status", resp.Status), nil
	}

	saved, err := st.saveResponse(ctx, h.Save, resp.Body)
	if err != nil {
		return outcome{}, err
	}

	detail := map[string]any{"status": resp.Status, "saved": saved}

	return outcome{outcome: "succeeded", acted: true, detail: detail}, nil
}

/*
request wraps the client's request so headers and the method read as
one chain.
*/
type request struct {
	*httpclient.Request
}

/*
withHeaders sets every header.
*/
func (r request) withHeaders(headers map[string]string) request {
	for name, value := range headers {
		r.Request = r.WithHeader(name, value)
	}

	return r
}

/*
send sends the request with its method.
*/
func (r request) send(ctx context.Context, req outbound) (*httpclient.Response, error) {
	switch req.method {
	case "POST":
		return r.Post(ctx, req.url, req.body)
	case "PUT":
		return r.Put(ctx, req.url, req.body)
	case "PATCH":
		return r.Patch(ctx, req.url, req.body)
	case "DELETE":
		return r.Delete(ctx, req.url)
	}

	return r.Get(ctx, req.url)
}

/*
requestFailed takes the failure path, or goes on, recording why.
*/
func requestFailed(s workflowspec.Step, code string, status int) outcome {
	out := fixedOrNext(s, workflowspec.OutcomeFailure)
	out.outcome, out.acted = "failed", true
	out.detail = map[string]any{"error": code}

	if status != 0 {
		out.detail["status"] = status
	}

	return out
}

/*
transportCode names a request that got no answer.
*/
func transportCode(err error) string {
	switch {
	case errors.Is(err, httpclient.ErrBlockedHost):
		return "host_blocked"
	case errors.Is(err, httpclient.ErrTooLarge):
		return "response_too_large"
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline"):
		return "timeout"
	}

	return "unreachable"
}

/*
timeoutOf is the step's timeout, eight seconds by default and ten at
most.
*/
func timeoutOf(setting string) time.Duration {
	d, err := workflowspec.ParseDuration(setting)
	if err != nil || d <= 0 {
		return requestTimeout
	}

	return min(d, requestTimeoutMost)
}

/*
buildRequest renders the request, through its connection when it has
one. A code names a request that cannot be made.
*/
func (st *state) buildRequest(ctx context.Context, h workflowspec.HTTPRequestSettings) (outbound, string, error) {
	req := outbound{method: h.Method, headers: map[string]string{}}
	if req.method == "" {
		req.method = "GET"
	}

	target, err := st.renderEscaped(ctx, h.URL, url.QueryEscape)
	if err != nil {
		return req, "", err
	}

	for _, header := range h.Headers {
		value, err := st.renderEscaped(ctx, header.Value, headerSafe)
		if err != nil {
			return req, "", err
		}

		req.headers[header.Name] = value
	}

	if h.Body != "" {
		body, err := st.renderEscaped(ctx, h.Body, jsonSafe)
		if err != nil {
			return req, "", err
		}

		if !json.Valid([]byte(body)) {
			return req, "body_invalid", nil
		}

		req.body, req.headers["Content-Type"] = []byte(body), "application/json"
	}

	if h.ConnectionID == "" {
		req.url = target
		return req, "", nil
	}

	return st.throughConnection(ctx, h.ConnectionID, target, req)
}

/*
throughConnection puts the request under the connection's base URL and
adds its headers and secret.
*/
func (st *state) throughConnection(ctx context.Context, public, path string, req outbound) (outbound, string, error) {
	id, ok := st.decode(models.PrefixAiActionConnection, public)
	if !ok {
		return req, outcomeMissingRef, nil
	}

	conn, err := st.q.FindActionConnection(ctx, models.FindActionConnectionParams{
		WorkspaceID: st.conv.WorkspaceID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) || err == nil && conn.Kind != actionkit.KindHTTP {
		return req, outcomeMissingRef, nil
	}

	if err != nil {
		return req, "", fmt.Errorf("find connection: %w", err)
	}

	if conn.Status == "disabled" {
		return req, "connection_disabled", nil
	}

	u, code := st.authorizedURL(ctx, conn, path, req.headers)
	if code != "" {
		return req, code, nil
	}

	req.url = u.String()

	return req, "", nil
}

/*
authorizedURL opens the connection's secret, joins its base URL with
the path and adds its headers and query. A code names a connection
that cannot be used, so the step takes its failure path.
*/
func (st *state) authorizedURL(
	ctx context.Context, conn models.AiActionConnection, path string, headers map[string]string,
) (*url.URL, string) {
	creds, err := actionkit.OpenCredentials(ctx, st.e.Keys, conn)
	if err != nil {
		return nil, "credentials_unreadable"
	}

	u, err := url.Parse(strings.TrimRight(conn.BaseUrl, "/") + path)
	if err != nil {
		return nil, "url_invalid"
	}

	authHeaders, query := actionkit.Authorize(conn, creds)
	maps.Copy(headers, authHeaders)

	q := u.Query()
	maps.Copy(q, query)
	u.RawQuery = q.Encode()

	return u, ""
}

/*
saveResponse copies the mapped values of a JSON response to their
fields and returns the fields it saved.
*/
func (st *state) saveResponse(ctx context.Context, saves []workflowspec.ResponseSave, body []byte) ([]string, error) {
	saved := []string{}
	if len(saves) == 0 || !json.Valid(body) {
		return saved, nil
	}

	for _, sv := range saves {
		value, found := responseValue(body, sv.Path)
		if !found {
			continue
		}

		if err := st.store(ctx, sv.Field, value); err != nil {
			return nil, err
		}

		saved = append(saved, sv.Field)
	}

	return saved, nil
}

/*
renderEscaped fills a text's variables, escaping each value for where
it lands.
*/
func (st *state) renderEscaped(ctx context.Context, text string, escape func(string) string) (string, error) {
	if !strings.Contains(text, "{{") {
		return text, nil
	}

	s, err := st.subject(ctx)
	if err != nil {
		return "", err
	}

	return variablePattern().ReplaceAllStringFunc(text, func(found string) string {
		m := variablePattern().FindStringSubmatch(found)
		if v := st.variable(s, m[1]); v != "" {
			return escape(v)
		}

		return escape(m[2])
	}), nil
}

/*
jsonSafe escapes a value for the inside of a JSON string.
*/
func jsonSafe(value string) string {
	raw, _ := json.Marshal(value)

	return string(raw[1 : len(raw)-1])
}

/*
headerSafe drops the characters that would end a header.
*/
func headerSafe(value string) string {
	return strings.NewReplacer("\r", "", "\n", "", "\x00", "").Replace(value)
}
