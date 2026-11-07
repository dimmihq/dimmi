/*
|--------------------------------------------------------------------------
| Request Rendering
|--------------------------------------------------------------------------
|
| A call becomes one HTTP request: the path template expanded under the
| connection's base URL, the query, the headers and the body filled from
| the frozen input and bindings. The final URL is parsed again and must
| still point at the connection's host and under its base path, so no
| value can steer the request elsewhere. The credential is added last,
| and the copy kept for the trace shows it as [secret].
|
*/

package actionkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actiontemplate"
)

/*
maxTraceBody caps the request or response body kept on a call.
*/
const maxTraceBody = 16 << 10

/*
userAgent names the agent to the workspace's API.
*/
const userAgent = "Dimmi-Actions/1 (+https://trydimmi.com/docs/actions)"

/*
Target is everything one execution needs: the connection and its
opened secret, the definition, the frozen input and bindings, and the
call's id, which doubles as the idempotency key of a write.
*/
type Target struct {
	Conn        models.AiActionConnection
	Creds       Credentials
	Spec        Spec
	Input       map[string]any
	Bindings    Bindings
	CallID      ids.UUID
	WorkspaceID ids.UUID
}

/*
vars are the values the target's templates are filled with.
*/
func (t Target) vars() actiontemplate.Vars {
	conn := map[string]string{}
	_ = json.Unmarshal(t.Conn.Variables, &conn)
	return actiontemplate.Vars{Input: t.Input, Contact: t.Bindings.Contact, Connection: conn}
}

/*
outbound is a rendered request, with the copy kept for the trace.
*/
type outbound struct {
	method   string
	url      string
	shownURL string
	headers  fhttp.Header
	shown    map[string]string
	body     []byte
}

/*
errURLEscape marks a rendered URL that left the connection's base.
*/
var errURLEscape = errors.New("rendered url leaves the connection")

/*
render turns the target into a request.
*/
func render(t Target) (outbound, error) {
	v := t.vars()
	path, err := actiontemplate.ExpandPath(t.Spec.PathTemplate, v)
	if err != nil {
		return outbound{}, err
	}
	u, err := targetURL(t.Conn.BaseUrl, path)
	if err != nil {
		return outbound{}, err
	}

	q, err := renderQuery(u.Query(), t, v)
	if err != nil {
		return outbound{}, err
	}

	out := outbound{method: t.Spec.Method, headers: fhttp.Header{}, shown: map[string]string{}}
	if err := renderHeaders(&out, t, v); err != nil {
		return outbound{}, err
	}
	if err := renderBody(&out, t, v); err != nil {
		return outbound{}, err
	}

	auth := applyAuth(t.Conn, t.Creds)
	shownQuery := cloneValues(q)
	for name := range auth.query {
		q.Set(name, auth.query.Get(name))
		shownQuery.Set(name, secretMask)
	}
	for name := range auth.headers {
		out.headers.Set(name, auth.headers.Get(name))
		out.shown[name] = secretMask
	}

	u.RawQuery = q.Encode()
	out.url = u.String()
	u.RawQuery = shownQuery.Encode()
	out.shownURL = u.String()
	return out, nil
}

/*
renderQuery fills the query parameters; one with no value is left out.
*/
func renderQuery(q url.Values, t Target, v actiontemplate.Vars) (url.Values, error) {
	for _, name := range sortedKeys(t.Spec.Query) {
		value, ok, err := actiontemplate.ExpandString(t.Spec.Query[name], v)
		if err != nil {
			return nil, err
		}
		if ok {
			q.Set(name, value)
		}
	}
	return q, nil
}

/*
targetURL joins the base URL and the expanded path and checks the
result stays on the base's scheme, host and path.
*/
func targetURL(base, path string) (*url.URL, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errURLEscape, err)
	}
	basePath := strings.TrimRight(b.EscapedPath(), "/") + "/"
	if u.Scheme != b.Scheme || u.Host != b.Host || !strings.HasPrefix(u.EscapedPath()+"/", basePath) {
		return nil, errURLEscape
	}
	return u, nil
}

/*
renderHeaders sets the connection's static headers, the action's
headers and the agent's own, which nothing may override.
*/
func renderHeaders(out *outbound, t Target, v actiontemplate.Vars) error {
	for name, value := range defaultHeaders(t.Conn) {
		out.headers.Set(name, value)
	}
	for _, name := range sortedKeys(t.Spec.Headers) {
		value, ok, err := actiontemplate.ExpandHeader(t.Spec.Headers[name], v)
		if err != nil {
			return err
		}
		if ok {
			out.headers.Set(name, value)
		}
	}
	callID := models.PublicID(models.PrefixAiActionCall, t.CallID)
	out.headers.Set("User-Agent", userAgent)
	out.headers.Set("Dimmi-Action", t.Spec.Name)
	out.headers.Set("Dimmi-Action-Call", callID)
	out.headers.Set("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, t.WorkspaceID))
	if t.Spec.SideEffect == SideEffectWrite {
		out.headers.Set("Idempotency-Key", callID)
	}
	for name := range out.headers {
		out.shown[name] = out.headers.Get(name)
	}
	return nil
}

/*
renderBody fills the body template as JSON or as a form.
*/
func renderBody(out *outbound, t Target, v actiontemplate.Vars) error {
	if len(t.Spec.BodyTemplate) == 0 {
		return nil
	}
	var tpl any
	if err := json.Unmarshal(t.Spec.BodyTemplate, &tpl); err != nil {
		return fmt.Errorf("%w: %w", actiontemplate.ErrInvalidTemplate, err)
	}
	if t.Spec.BodyEncoding == "form" {
		fields, _ := tpl.(map[string]any)
		form, err := actiontemplate.ExpandForm(fields, v)
		if err != nil {
			return err
		}
		out.body = []byte(form.Encode())
		out.headers.Set("Content-Type", "application/x-www-form-urlencoded")
		return nil
	}
	body, _, err := actiontemplate.ExpandJSON(tpl, v)
	if err != nil {
		return err
	}
	if out.body, err = json.Marshal(body); err != nil {
		return fmt.Errorf("encode body: %w", err)
	}
	out.headers.Set("Content-Type", "application/json")
	return nil
}

/*
failureCode maps a rendering failure to the call's error code.
*/
func failureCode(err error) string {
	switch {
	case errors.Is(err, errURLEscape):
		return "url_escape"
	case errors.Is(err, actiontemplate.ErrInvalidPathValue):
		return "invalid_path_value"
	case errors.Is(err, actiontemplate.ErrInvalidHeader):
		return "invalid_header_value"
	case errors.Is(err, actiontemplate.ErrTypeMismatch):
		return "template_type_mismatch"
	}
	return "invalid_arguments"
}

/*
trace is the copy of a request kept on the call, secrets masked.
*/
func (o outbound) trace() json.RawMessage {
	out, err := json.Marshal(map[string]any{
		"method": o.method, "url": o.shownURL, "headers": o.shown, "body": traceBody(o.body),
	})
	if err != nil {
		return nil
	}
	return out
}

/*
traceBody keeps a body for a trace: JSON as JSON, anything else as
text, cut to the trace cap.
*/
func traceBody(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	if len(body) <= maxTraceBody && json.Valid(body) {
		return json.RawMessage(body)
	}
	if len(body) > maxTraceBody {
		body = body[:maxTraceBody]
	}
	return string(body)
}

/*
cloneValues copies query values.
*/
func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}
