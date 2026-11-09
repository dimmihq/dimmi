/*
|--------------------------------------------------------------------------
| Definition Rules
|--------------------------------------------------------------------------
|
| An action is checked when it is saved and again when it is published,
| so a broken template or an unbounded schema never reaches the agent.
| Reads run inline and need nobody's agreement; a write's agreement is
| the admin's choice. Every placeholder must name something that can
| fill it, and a header an action sets can never override the ones the
| agent controls, such as the credential or the idempotency key.
|
*/

package actionkit

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionresult"
	"github.com/dimmihq/dimmi/app/support/actionschema"
	"github.com/dimmihq/dimmi/app/support/actiontemplate"
)

/*
Definition limits.
*/
const (
	maxTitle          = 80
	minDescription    = 10
	maxDescription    = 1024
	maxPathTemplate   = 512
	maxQueryParams    = 20
	maxHeaders        = 10
	defaultMaxBytes   = 65536
	defaultApprovalTO = 60
)

/*
namePattern matches a tool name.
*/
func namePattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z][a-z0-9_]{2,47}$`)
}

/*
headerPattern matches a header name an action may set.
*/
func headerPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
}

/*
channelPattern matches a channel type name.
*/
func channelPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z][a-z0-9_]{1,30}$`)
}

/*
ReservedName reports whether a tool name belongs to the agent itself.
*/
func ReservedName(name string) bool {
	return slices.Contains([]string{"search_knowledge", "handoff", "triage_conversation"}, name) ||
		strings.HasPrefix(name, "dimmi_")
}

/*
reservedHeader reports whether a header is the agent's to set.
*/
func reservedHeader(name string) bool {
	lower := strings.ToLower(name)
	return slices.Contains([]string{
		"host", "cookie", "content-length", "content-type", "transfer-encoding", "authorization",
		"idempotency-key", "user-agent", "connection",
	}, lower) || strings.HasPrefix(lower, "dimmi-")
}

/*
Normalize checks a definition against its connection and returns it with
its defaults filled in and its schema in normal form.
*/
func Normalize(s Spec, conn models.AiActionConnection) (Spec, error) {
	s = withDefaults(s)
	checks := []func(Spec, models.AiActionConnection) error{checkIdentity, checkAgreement, checkShape, checkRequest}
	for _, check := range checks {
		if err := check(s, conn); err != nil {
			return s, err
		}
	}

	schema, err := actionschema.Normalize(s.InputSchema)
	if err != nil {
		return s, fault.New(fault.Invalid, exceptions.ErrActionSchemaUnsupported.Code(), err.Error()).
			OnField("input_schema")
	}
	s.InputSchema = schema

	return s, checkRefs(s, conn)
}

/*
withDefaults fills what the admin left out.
*/
func withDefaults(s Spec) Spec {
	if s.Query == nil {
		s.Query = map[string]string{}
	}
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}
	if s.BoundArguments == nil {
		s.BoundArguments = map[string]any{}
	}
	if s.ResponseFields == nil {
		s.ResponseFields = []actionresult.ResponseField{}
	}
	if s.BodyEncoding == "" {
		s.BodyEncoding = "json"
	}
	if s.ResponseMaxBytes == 0 {
		s.ResponseMaxBytes = defaultMaxBytes
	}
	if s.ApprovalTimeoutMinutes == 0 {
		s.ApprovalTimeoutMinutes = defaultApprovalTO
	}
	if len(s.Channels) == 0 {
		s.Channels = []string{"widget"}
	}
	s.Agents = compactAgents(s.Agents)
	s.Method = strings.ToUpper(s.Method)
	return s
}

/*
checkIdentity checks the name, title and description.
*/
func checkIdentity(s Spec, _ models.AiActionConnection) error {
	switch {
	case !namePattern().MatchString(s.Name):
		return invalid("name_invalid", "name", "use 3 to 48 lowercase letters, digits and underscores")
	case ReservedName(s.Name):
		return exceptions.ErrActionNameReserved.OnField("name")
	case s.Title == "" || len([]rune(s.Title)) > maxTitle:
		return invalid("title_invalid", "title", "the title needs 1 to 80 characters")
	case len([]rune(s.Description)) < minDescription || len([]rune(s.Description)) > maxDescription:
		return invalid("description_invalid", "description", "the description needs 10 to 1024 characters")
	}
	for _, c := range s.Channels {
		if !channelPattern().MatchString(c) {
			return invalid("channel_invalid", "channels", "unknown channel "+c)
		}
	}
	for _, a := range s.Agents {
		if a != AgentDimmi && a != AgentOperator {
			return invalid("agent_invalid", "agents", "agents are dimmi and operator")
		}
	}
	return nil
}

/*
checkAgreement checks the side effect and who must agree to a call.
*/
func checkAgreement(s Spec, _ models.AiActionConnection) error {
	switch {
	case s.SideEffect != SideEffectRead && s.SideEffect != SideEffectWrite:
		return invalid("side_effect_invalid", "side_effect", "side_effect is read or write")
	case s.Confirmation != ConfirmationNone && s.Confirmation != ConfirmationCustomer:
		return invalid("confirmation_invalid", "confirmation", "confirmation is none or customer")
	case !slices.Contains([]string{ApprovalNone, ApprovalTeammate, ApprovalAdmin}, s.Approval):
		return invalid("approval_invalid", "approval", "approval is none, teammate or admin")
	case s.SideEffect == SideEffectRead && s.NeedsAgreement():
		return invalid("read_needs_no_agreement", "side_effect",
			"a read runs inline; only a write may wait for a confirmation or an approval")
	case s.ApprovalTimeoutMinutes < 1 || s.ApprovalTimeoutMinutes > 10080:
		return invalid("approval_timeout_invalid", "approval_timeout_minutes", "use 1 to 10080 minutes")
	}
	return nil
}

/*
checkShape checks the response mapping and limits.
*/
func checkShape(s Spec, _ models.AiActionConnection) error {
	if err := actionresult.CheckFields(s.ResponseFields); err != nil {
		return invalid("response_fields_invalid", "response_fields", err.Error())
	}
	if s.ErrorMessagePath != "" {
		if err := actionresult.CheckPath(s.ErrorMessagePath); err != nil {
			return invalid("error_message_path_invalid", "error_message_path", err.Error())
		}
	}
	if s.ResponseMaxBytes < 1024 || s.ResponseMaxBytes > 262144 {
		return invalid("response_max_bytes_invalid", "response_max_bytes", "use 1024 to 262144 bytes")
	}
	if s.TimeoutMS != 0 && (s.TimeoutMS < 1000 || s.TimeoutMS > 15000) {
		return invalid("timeout_invalid", "timeout_ms", "use 1000 to 15000 milliseconds")
	}
	return nil
}

/*
checkRequest checks how the request is described, by kind.
*/
func checkRequest(s Spec, conn models.AiActionConnection) error {
	want := KindHTTP
	if conn.Kind == "mcp" {
		want = KindMCPTool
	}
	if s.Kind != want {
		return exceptions.ErrActionKindMismatch.OnField("kind")
	}
	if s.Kind == KindMCPTool {
		if strings.TrimSpace(s.MCPToolName) == "" {
			return invalid("mcp_tool_name_required", "mcp_tool_name", "name the tool on the server")
		}
		return nil
	}
	return checkHTTP(s)
}

/*
checkHTTP checks an HTTP action's method, path, query, headers and body.
*/
func checkHTTP(s Spec) error {
	if !slices.Contains([]string{"GET", "POST", "PUT", "PATCH", "DELETE"}, s.Method) {
		return invalid("method_invalid", "method", "method is GET, POST, PUT, PATCH or DELETE")
	}
	if !strings.HasPrefix(s.PathTemplate, "/") || len(s.PathTemplate) > maxPathTemplate {
		return invalid("path_template_invalid", "path_template",
			"the path starts with / and has at most 512 characters")
	}
	if err := actiontemplate.CheckBraces(s.PathTemplate); err != nil {
		return templateError("path_template", err)
	}
	if len(s.Query) > maxQueryParams || len(s.Headers) > maxHeaders {
		return invalid("too_many_parameters", "query", "at most 20 query parameters and 10 headers")
	}
	for name := range s.Headers {
		if !headerPattern().MatchString(name) || reservedHeader(name) {
			return invalid("header_invalid", "headers", "header "+name+" may not be set by an action")
		}
	}
	return checkBody(s)
}

/*
checkBody checks the body template against the method and encoding.
*/
func checkBody(s Spec) error {
	if len(s.BodyTemplate) == 0 {
		return nil
	}
	if !slices.Contains([]string{"POST", "PUT", "PATCH"}, s.Method) {
		return invalid("body_not_allowed", "body_template", "only POST, PUT and PATCH carry a body")
	}
	var body any
	if err := json.Unmarshal(s.BodyTemplate, &body); err != nil {
		return invalid("body_template_invalid", "body_template", "the body template must be JSON")
	}
	switch s.BodyEncoding {
	case "json":
		return nil
	case "form":
		fields, ok := body.(map[string]any)
		if !ok {
			return invalid("body_template_invalid", "body_template", "a form body is a flat object")
		}
		for k, v := range fields {
			if _, nested := v.(map[string]any); nested {
				return invalid("body_template_invalid", "body_template", "form field "+k+" must be a plain value")
			}
		}
		return nil
	}
	return invalid("body_encoding_invalid", "body_encoding", "body_encoding is json or form")
}

/*
checkRefs checks that every placeholder names a declared input, a
contact value or a connection variable.
*/
func checkRefs(s Spec, conn models.AiActionConnection) error {
	inputs := actionschema.Properties(s.InputSchema)
	variables := connectionVariables(conn)
	for _, r := range s.Refs() {
		if err := actiontemplate.Check(r, inputs, variables); err != nil {
			return templateError("templates", err)
		}
	}
	return nil
}

/*
connectionVariables lists the variable names a connection holds.
*/
func connectionVariables(conn models.AiActionConnection) []string {
	vars := map[string]string{}
	_ = json.Unmarshal(conn.Variables, &vars)
	return sortedKeys(vars)
}

/*
invalid is a 422 on a field.
*/
func invalid(code, field, message string) error {
	return fault.New(fault.Invalid, code, message).OnField(field)
}

/*
templateError is a template failure on a field.
*/
func templateError(field string, err error) error {
	return fault.New(fault.Invalid, exceptions.ErrActionTemplateInvalid.Code(), fmt.Sprint(err)).OnField(field)
}
