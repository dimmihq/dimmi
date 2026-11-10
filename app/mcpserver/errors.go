/*
|--------------------------------------------------------------------------
| Errors That Say What To Do
|--------------------------------------------------------------------------
|
| A failed call is the moment an agent most needs help. Every error says
| what failed, why in the API's own words, which arguments were wrong
| and how, and the one thing to do next: fix an argument, find the right
| id, ask the person for access, wait, or stop and tell the person. The
| same detail goes to programs as structured content. Nothing about the
| server's internals is ever shown.
|
*/

package mcpserver

import (
	"fmt"
	"slices"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
apiError is a call the API refused or failed.
*/
type apiError struct {
	tool       string
	doc        Doc
	op         mcp.Operation
	status     int
	statusText string
	code       string
	message    string
	fields     map[string][]string
	retryWait  string
	requestID  string
}

/*
refusal reads the API's error answer.
*/
func refusal(tool string, doc Doc, op mcp.Operation, a mcp.Response) *apiError {
	e := &apiError{
		tool: tool, doc: doc, op: op, status: a.Status, statusText: a.StatusText(),
		retryWait: a.Header.Get("Retry-After"), requestID: a.Header.Get("X-Request-Id"),
	}

	body := asMap(a.Decoded())
	e.code, _ = body["code"].(string)
	e.message, _ = body["message"].(string)

	if fields := asMap(body["errors"]); fields != nil {
		e.fields = map[string][]string{}
		for name, list := range fields {
			for _, item := range asList(list) {
				e.fields[name] = append(e.fields[name], fmt.Sprint(item))
			}
		}
	}

	return e
}

/*
Error describes the failure in one line.
*/
func (e *apiError) Error() string {
	return fmt.Sprintf("%s failed with %d %s", e.tool, e.status, e.code)
}

/*
ToolResult writes the error for the agent: what failed, why, which
arguments were wrong, and what to do now.
*/
func (e *apiError) ToolResult() (string, any) {
	var b strings.Builder

	fmt.Fprintf(&b, "%s failed (%d %s).\n", title(e.doc, e.tool), e.status, e.statusText)
	if e.message != "" {
		fmt.Fprintf(&b, "Reason: %s\n", e.message)
	}

	if e.code != "" {
		fmt.Fprintf(&b, "Code: %s\n", e.code)
	}

	if len(e.fields) > 0 {
		b.WriteString("Arguments to fix:\n")

		names := make([]string, 0, len(e.fields))
		for name := range e.fields {
			names = append(names, name)
		}

		slices.Sort(names)

		for _, name := range names {
			fmt.Fprintf(&b, "- %s: %s\n", name, strings.Join(e.fields[name], " "))
		}
	}

	fmt.Fprintf(&b, "What to do: %s\n", e.advice())
	if e.doc.Hint != "" {
		fmt.Fprintf(&b, "Tip: %s\n", e.doc.Hint)
	}

	detail := map[string]any{
		"status": e.status, "code": e.code, "message": e.message, "errors": e.fields, "advice": e.advice(),
	}
	if e.requestID != "" {
		detail["request_id"] = e.requestID
	}

	return b.String(), map[string]any{"error": detail}
}

/*
advice is the one thing the agent should do after this failure.
*/
func (e *apiError) advice() string {
	switch {
	case e.status == fhttp.StatusUnauthorized:
		return "The connection's token is missing, expired or revoked. Stop and ask the person to reconnect dimmi " +
			"with a valid token."
	case e.status == fhttp.StatusForbidden:
		return e.forbidden()
	case e.status == fhttp.StatusNotFound:
		return e.notFound()
	case e.status == fhttp.StatusConflict:
		return "The record changed or is in a state that does not allow this. Read it again with the matching get " +
			"tool, then decide whether to retry with updated arguments."
	case e.status == fhttp.StatusUnprocessableEntity || e.status == fhttp.StatusBadRequest:
		return "Fix the arguments listed above, keeping the others as they were, and call " + e.tool +
			" again. Check the tool's input schema for allowed values and limits."
	case e.status == fhttp.StatusTooManyRequests:
		return e.slowDown()
	case e.status == fhttp.StatusRequestEntityTooLarge:
		return "The request is too large. Send less, such as a shorter text or fewer items."
	case e.status >= fhttp.StatusInternalServerError:
		return "dimmi could not finish this. Try once more in a little while; if it fails again, stop and tell the " +
			"person, mentioning the request id if there is one."
	default:
		return "Read the reason above, adjust the call, and try once more; if it still fails, tell the person."
	}
}

/*
forbidden explains a refusal for lack of access.
*/
func (e *apiError) forbidden() string {
	if e.code == "missing_ability" || strings.Contains(e.code, "ability") || strings.Contains(e.code, "scope") {
		return "The token this connection uses does not allow " + e.tool + ". Do not retry. Tell the person " +
			"which access is missing (" + strings.Join(e.op.Abilities, ", ") + ") so they can connect with a " +
			"token that has it."
	}

	return "The person this connection acts for is not allowed to do this, by their role or the workspace's " +
		"rules. Do not retry or look for a way around it. Tell the person, who can ask a workspace admin."
}

/*
notFound explains a record that does not exist or cannot be seen.
*/
func (e *apiError) notFound() string {
	var finders []string

	guide := argumentGuide()
	for _, p := range pathArguments(e.op.Path) {
		if text, ok := guide[p]; ok {
			finders = append(finders, p+": "+text)
		}
	}

	advice := "Nothing with that id exists in this workspace, or the person cannot see it. Do not guess ids."
	if len(finders) == 0 {
		return advice + " Find the right one with a list or search tool, then call again."
	}

	return advice + " Check each id: " + strings.Join(finders, " ")
}

/*
slowDown explains a rate limit.
*/
func (e *apiError) slowDown() string {
	if e.retryWait != "" {
		return "Too many calls in a short time. Wait " + e.retryWait + " seconds, then call again. Prefer one " +
			"list call with filters over many single reads."
	}

	return "Too many calls in a short time. Wait a minute, then call again. Prefer one list call with filters " +
		"over many single reads."
}

/*
pathArguments returns the names of a path template's parameters.
*/
func pathArguments(path string) []string {
	var names []string

	for part := range strings.SplitSeq(path, "/") {
		if name, ok := strings.CutPrefix(part, "{"); ok {
			names = append(names, strings.TrimSuffix(name, "}"))
		}
	}

	return names
}

/*
asList reads a JSON array, nil for anything else.
*/
func asList(v any) []any {
	l, _ := v.([]any)

	return l
}
