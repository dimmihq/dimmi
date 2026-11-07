/*
|--------------------------------------------------------------------------
| Sending A Call
|--------------------------------------------------------------------------
|
| Every request leaves through the guarded HTTP client: private and
| metadata addresses refused at connect, redirects never followed, the
| body capped and the clock running. A read that met a passing outage
| is tried once more; a write never is, because bytes that left may
| already have changed the customer's account. Whatever came back is
| turned into a stable error code the model and the teammates read,
| never the raw upstream body.
|
*/

package actionkit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/support/actionresult"
)

/*
Outcome is how one execution ended: success or a stable error code,
the upstream status and message, the redacted request and response
kept for teammates, the result the model reads, and what it says about
the connection's health.
*/
type Outcome struct {
	Succeeded   bool
	ErrorCode   string
	HTTPStatus  int
	Message     string
	Request     json.RawMessage
	Response    json.RawMessage
	Result      json.RawMessage
	Latency     time.Duration
	Reached     bool
	AuthFailure bool
	Blocked     bool
	retryable   bool
}

/*
Send runs the target once, or twice for a read that met a passing
outage.
*/
func (k *Kit) Send(ctx context.Context, t Target) Outcome {
	send := k.sendHTTP
	if t.Spec.Kind == KindMCPTool {
		send = k.sendMCP
	}
	out := send(ctx, t)
	if out.retryable && t.Spec.SideEffect == SideEffectRead {
		out = send(ctx, t)
	}
	return out.withResult()
}

/*
sendHTTP renders and sends one HTTP request.
*/
func (k *Kit) sendHTTP(ctx context.Context, t Target) Outcome {
	started := k.Clock.Now()
	req, err := render(t)
	if err != nil {
		return Outcome{ErrorCode: failureCode(err), Message: err.Error()}
	}

	r := k.HTTP.New().WithoutRedirects().MaxBody(int64(t.Spec.ResponseMaxBytes)).Timeout(timeoutFor(t))
	for name := range req.headers {
		r = r.WithHeader(name, req.headers.Get(name))
	}
	resp, err := dispatch(ctx, r, req)

	out := Outcome{Request: req.trace(), Latency: k.Clock.Now().Sub(started)}
	if err != nil {
		return transportFailure(out, err)
	}
	out.Reached, out.HTTPStatus = true, resp.Status
	out.Response = responseTrace(resp)
	return classify(out, t, resp)
}

/*
dispatch sends the request with its method.
*/
func dispatch(ctx context.Context, r *httpclient.Request, req outbound) (*httpclient.Response, error) {
	switch req.method {
	case "POST":
		return r.Post(ctx, req.url, bodyOf(req))
	case "PUT":
		return r.Put(ctx, req.url, bodyOf(req))
	case "PATCH":
		return r.Patch(ctx, req.url, bodyOf(req))
	case "DELETE":
		return r.Delete(ctx, req.url)
	}
	return r.Get(ctx, req.url)
}

/*
bodyOf is the body to send: the rendered bytes, or an empty body for a
write with no template.
*/
func bodyOf(req outbound) []byte {
	if req.body == nil {
		return []byte{}
	}
	return req.body
}

/*
transportFailure maps a request that got no response.
*/
func transportFailure(out Outcome, err error) Outcome {
	switch {
	case errors.Is(err, httpclient.ErrBlockedHost):
		out.ErrorCode, out.Blocked = "host_blocked", true
	case errors.Is(err, httpclient.ErrTooLarge):
		out.ErrorCode, out.Reached = "response_too_large", true
	default:
		out.ErrorCode, out.retryable = "unavailable", true
	}
	return out
}

/*
classify maps the response's status and body to the outcome.
*/
func classify(out Outcome, t Target, resp *httpclient.Response) Outcome {
	status := resp.Status
	switch {
	case status >= 200 && status < 300:
		out.Succeeded = true
		out.Result = shape(resp.Body, t.Spec)
	case status >= 300 && status < 400:
		out.ErrorCode = "redirect_not_followed"
	case status == 401 || status == 403:
		out.ErrorCode, out.AuthFailure = "auth_failed", true
	case status == 404:
		out.ErrorCode = "not_found"
		out.Message = upstreamMessage(resp.Body, t.Spec, "not found")
	case status == 400 || status == 409 || status == 422:
		out.ErrorCode = "upstream_rejected"
		out.Message = upstreamMessage(resp.Body, t.Spec, "the request was refused")
	case status == 429:
		out.ErrorCode = "rate_limited"
	default:
		out.ErrorCode = "unavailable"
		out.retryable = status == 502 || status == 503 || status == 504
	}
	return out
}

/*
shape maps a successful body to the model's result: mapped JSON, or
text when the body is not JSON.
*/
func shape(body []byte, s Spec) json.RawMessage {
	if strings.TrimSpace(string(body)) == "" {
		return json.RawMessage(`{"ok":true}`)
	}
	out, err := actionresult.Shape(body, s.ResponseFields, actionresult.DefaultMaxBytes)
	if err != nil {
		return actionresult.Text(string(body))
	}
	return out
}

/*
upstreamMessage reads the upstream's own explanation at the action's
error path, or a plain fallback.
*/
func upstreamMessage(body []byte, s Spec, fallback string) string {
	if msg := actionresult.ErrorMessage(body, s.ErrorMessagePath); msg != "" {
		return msg
	}
	return fallback
}

/*
withResult gives a failed outcome the result the model reads: the code
and, for refusals the customer can act on, the upstream's message.
*/
func (o Outcome) withResult() Outcome {
	if o.Succeeded {
		return o
	}
	msg := "unavailable"
	switch o.ErrorCode {
	case "not_found", "upstream_rejected":
		msg = o.Message
	case "invalid_arguments", "invalid_path_value", "invalid_header_value", "template_type_mismatch", "url_escape":
		msg = "invalid_arguments: " + o.Message
	}
	o.Result, _ = json.Marshal(map[string]string{"error": o.ErrorCode, "message": msg})
	return o
}

/*
responseTrace is the copy of a response kept on the call.
*/
func responseTrace(resp *httpclient.Response) json.RawMessage {
	out, err := json.Marshal(map[string]any{
		"status": resp.Status, "content_type": resp.Header.Get("Content-Type"), "bytes": len(resp.Body),
		"body": traceBody(resp.Body),
	})
	if err != nil {
		return nil
	}
	return out
}
