/*
|--------------------------------------------------------------------------
| Checking Reply Times, Categories and Calls
|--------------------------------------------------------------------------
|
| A reply time message may use one more variable, the reply time itself.
| Sorting by topic needs named categories the model can tell apart and
| a place to keep the answer that a customer's words cannot reach into.
| Running another workflow needs the one it runs. A request to the
| customer's own system needs a method, a URL whose host no variable can
| change, headers that carry no credentials (those live on a connection,
| sealed), a timeout of ten seconds at most and response paths in the
| plain dotted form.
|
*/

package workflowspec

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
Limits of these steps.
*/
const (
	maxDescription = 300
	maxHeaders     = 10
	maxSaves       = 10
	maxBody        = 8000
	maxURL         = 2000
	maxRequestWait = 10 * time.Second
)

/*
VariableReplyTime is the variable a reply time step fills.
*/
const VariableReplyTime = "reply_time"

/*
DefaultReplyTimeText is what a reply time step says when it has no text.
*/
const DefaultReplyTimeText = "We typically reply {{reply_time}}."

/*
checkReplyTime checks a reply time step's text.
*/
func (c *checker) checkReplyTime(s Step) {
	r, ok := decode[ReplyTimeSettings](c, s)
	if !ok {
		return
	}

	if utf8.RuneCountInString(r.Text) > maxMessage {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "text", Message: "This text is too long."})
	}

	for _, name := range Variables(r.Text) {
		if name != VariableReplyTime && !KnownVariable(name) {
			c.fail(Issue{
				Code: CodeUnknownVariable, StepID: s.ID, Field: "text", Message: "No variable is called " + name + ".",
			})
		}
	}
}

/*
checkClassify checks the categories and where the answer is kept.
*/
func (c *checker) checkClassify(s Step) {
	cl, ok := decode[ClassifySettings](c, s)
	if !ok {
		return
	}

	seen := map[string]bool{}
	for _, p := range userPaths(s) {
		label := strings.ToLower(strings.TrimSpace(p.Label))
		if n := utf8.RuneCountInString(label); n == 0 || n > maxLabel || seen[label] {
			c.fail(Issue{
				Code: CodeMissingSetting, StepID: s.ID, PathID: p.ID,
				Message: "Name each category differently, 40 characters at most.",
			})
		}

		seen[label] = true
		if utf8.RuneCountInString(p.Description) > maxDescription {
			c.fail(Issue{
				Code: CodeLimitExceeded, StepID: s.ID, PathID: p.ID, Message: "Describe it in 300 characters.",
			})
		}
	}

	if utf8.RuneCountInString(cl.Question) > maxNote {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "question", Message: "This text is too long."})
	}

	if cl.SaveTo != "" && !saveTarget(cl.SaveTo) {
		c.fail(Issue{
			Code: CodeInvalidSetting, StepID: s.ID, Field: "save_to",
			Message: "Save to run.vars.<key> or contact.custom.<key>.",
		})
	}

	if cl.Source != "" {
		c.oneOf(s, "source", cl.Source, "last_message", "conversation")
	}

	if cl.MinConfidence != "" {
		c.oneOf(s, "min_confidence", cl.MinConfidence, "low", "medium", "high")
	}
}

/*
saveTarget reports whether a value may be saved to the field: a run
variable or a contact's custom attribute, never an identifying field.
*/
func saveTarget(field string) bool {
	for _, prefix := range []string{"contact.custom.", "run.vars."} {
		if rest, ok := strings.CutPrefix(field, prefix); ok && keyPattern().MatchString(rest) {
			return true
		}
	}

	return false
}

/*
checkRunWorkflow checks the workflow a step runs is named.
*/
func (c *checker) checkRunWorkflow(s Step) {
	if r, ok := decode[RunWorkflowSettings](c, s); ok {
		c.required(s, "workflow_id", r.WorkflowID, "Pick the workflow to run.")
	}
}

/*
checkHTTPRequest checks a request to the customer's system.
*/
func (c *checker) checkHTTPRequest(s Step) {
	h, ok := decode[HTTPRequestSettings](c, s)
	if !ok {
		return
	}

	c.oneOf(s, "method", h.Method, "GET", "POST", "PUT", "PATCH", "DELETE")
	c.requestURL(s, h)
	c.duration(s, "timeout", h.Timeout, span{time.Second, maxRequestWait})
	c.requestHeaders(s, h.Headers)

	if h.Body != "" && (h.Method == fhttp.MethodGet || h.Method == fhttp.MethodDelete) {
		c.fail(Issue{
			Code: CodeInvalidSetting, StepID: s.ID, Field: "body", Message: "Only POST, PUT and PATCH send a body.",
		})
	}

	if utf8.RuneCountInString(h.Body) > maxBody {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "body", Message: "This body is too long."})
	}

	c.variables(s, "body", h.Body)
	c.responseSaves(s, h.Save)

	if h.ConnectionID == "" {
		c.warn(Issue{
			Code: WarnHTTPRawURL, StepID: s.ID, Field: "url",
			Message: "This request goes out without a connection, so it carries no credentials.",
		})
	}
}

/*
requestURL checks the URL: absolute https without a connection, a path
with one, and never a variable before the path, where it could pick
the host.
*/
func (c *checker) requestURL(s Step, h HTTPRequestSettings) {
	raw := strings.TrimSpace(h.URL)
	at := Issue{Code: CodeInvalidSetting, StepID: s.ID, Field: "url"}

	if raw == "" || len(raw) > maxURL {
		at.Code, at.Message = CodeMissingSetting, "Write the URL to call."
		c.fail(at)

		return
	}

	c.variables(s, "url", raw)

	if h.ConnectionID != "" {
		if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
			at.Message = "With a connection, write the path under its base URL, starting with /."
			c.fail(at)
		}

		return
	}

	head, _, templated := strings.Cut(raw, "{{")
	rest, _ := strings.CutPrefix(head, "https://")
	hostDone := !templated || strings.ContainsAny(rest, "/?#")

	u, err := url.Parse(head)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || !hostDone {
		at.Message = "Use an absolute https URL; variables may only follow its host."
		c.fail(at)
	}
}

/*
headerName is the shape of a header name.
*/
func headerName() *regexp.Regexp {
	return regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,63}$`)
}

/*
requestHeaders refuses malformed headers and the ones that carry
credentials or belong to the transport.
*/
func (c *checker) requestHeaders(s Step, headers []HTTPHeader) {
	if len(headers) > maxHeaders {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "headers", Message: "Send 10 headers at most."})
	}

	for _, h := range headers {
		lower := strings.ToLower(h.Name)
		reserved := lower == "authorization" || lower == "cookie" || lower == "host" || lower == "content-length" ||
			lower == "proxy-authorization" || strings.HasPrefix(lower, "x-api-key")
		if !headerName().MatchString(h.Name) || reserved || strings.ContainsAny(h.Value, "\r\n\x00") ||
			len(h.Value) > 1000 {
			c.fail(Issue{
				Code: CodeInvalidSetting, StepID: s.ID, Field: "headers",
				Message: "Header " + h.Name + " cannot be set here; credentials belong on a connection.",
			})
		}

		c.variables(s, "headers", h.Value)
	}
}

/*
responsePath is the plain dotted form of a response path.
*/
func responsePath() *regexp.Regexp {
	return regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)
}

/*
responseSaves checks where response values are saved.
*/
func (c *checker) responseSaves(s Step, saves []ResponseSave) {
	if len(saves) > maxSaves {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "save", Message: "Save 10 values at most."})
	}

	for _, sv := range saves {
		if len(sv.Path) > 200 || !responsePath().MatchString(sv.Path) || !saveTarget(sv.Field) {
			c.fail(Issue{
				Code: CodeInvalidSetting, StepID: s.ID, Field: "save",
				Message: "Save a dotted path like data.plan to run.vars.<key> or contact.custom.<key>.",
			})
		}
	}
}
