/*
|--------------------------------------------------------------------------
| Action Definitions
|--------------------------------------------------------------------------
|
| An action's definition is one value: what the working copy holds and
| what a published version freezes. Publishing snapshots it, and the
| agent only ever runs a snapshot, so an edit in progress never changes
| what a live conversation does. The hash of the definition tells
| whether the working copy differs from what is live.
|
*/

package actionkit

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionresult"
	"github.com/dimmihq/dimmi/app/support/actiontemplate"
)

/*
The kinds, side effects and agreement settings an action may have.
*/
const (
	KindHTTP    = "http"
	KindMCPTool = "mcp_tool"

	SideEffectRead  = "read"
	SideEffectWrite = "write"

	ConfirmationNone     = "none"
	ConfirmationCustomer = "customer"

	ApprovalNone     = "none"
	ApprovalTeammate = "teammate"
	ApprovalAdmin    = "admin"
)

/*
Spec is an action's definition. Templates may hold placeholders;
BoundArguments are MCP arguments the admin fills, hidden from the
model; TimeoutMS of zero takes the connection's.
*/
type Spec struct {
	ConnectionID           ids.UUID             `json:"connection_id"`
	Name                   string               `json:"name"`
	Title                  string               `json:"title"`
	Description            string               `json:"description"`
	Kind                   string               `json:"kind"`
	SideEffect             string               `json:"side_effect"`
	Confirmation           string               `json:"confirmation"`
	Approval               string               `json:"approval"`
	ApprovalTimeoutMinutes int32                `json:"approval_timeout_minutes"`
	Method                 string               `json:"method,omitempty"`
	PathTemplate           string               `json:"path_template,omitempty"`
	Query                  map[string]string    `json:"query"`
	Headers                map[string]string    `json:"headers"`
	BodyTemplate           json.RawMessage      `json:"body_template,omitempty"`
	BodyEncoding           string               `json:"body_encoding"`
	MCPToolName            string               `json:"mcp_tool_name,omitempty"`
	BoundArguments         map[string]any       `json:"bound_arguments"`
	InputSchema            json.RawMessage      `json:"input_schema"`
	ResponseFields         []actionresult.Field `json:"response_fields"`
	ErrorMessagePath       string               `json:"error_message_path,omitempty"`
	ResponseMaxBytes       int32                `json:"response_max_bytes"`
	TimeoutMS              int32                `json:"timeout_ms,omitempty"`
	Channels               []string             `json:"channels"`
}

/*
FromAction reads an action's working copy as a definition.
*/
func FromAction(a models.AiAction) Spec {
	s := Spec{
		ConnectionID: a.ConnectionID, Name: a.Name, Title: a.Title, Description: a.Description,
		Kind: a.Kind, SideEffect: a.SideEffect, Confirmation: a.Confirmation, Approval: a.Approval,
		ApprovalTimeoutMinutes: a.ApprovalTimeoutMinutes, Method: deref(a.Method),
		PathTemplate: deref(a.PathTemplate), BodyTemplate: a.BodyTemplate, BodyEncoding: a.BodyEncoding,
		MCPToolName: deref(a.McpToolName), InputSchema: a.InputSchema, ErrorMessagePath: deref(a.ErrorMessagePath),
		ResponseMaxBytes: a.ResponseMaxBytes, Channels: a.Channels,
		Query: map[string]string{}, Headers: map[string]string{}, BoundArguments: map[string]any{},
	}
	if a.TimeoutMs != nil {
		s.TimeoutMS = *a.TimeoutMs
	}
	_ = json.Unmarshal(a.Query, &s.Query)
	_ = json.Unmarshal(a.Headers, &s.Headers)
	_ = json.Unmarshal(a.BoundArguments, &s.BoundArguments)
	_ = json.Unmarshal(a.ResponseFields, &s.ResponseFields)
	if len(s.BodyTemplate) == 0 || string(s.BodyTemplate) == "null" {
		s.BodyTemplate = nil
	}
	return s
}

/*
FromVersion reads a published version's frozen definition.
*/
func FromVersion(v models.AiActionVersion) (Spec, error) {
	var s Spec
	if err := json.Unmarshal(v.Spec, &s); err != nil {
		return s, fmt.Errorf("decode action version %s: %w", v.ID, err)
	}
	return s, nil
}

/*
Hash fingerprints the definition, so an unchanged working copy is never
published twice.
*/
func (s Spec) Hash() string {
	data, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return hashing.SHA256Hex(data)
}

/*
Refs lists every placeholder of the definition: path, query, headers,
body and bound arguments.
*/
func (s Spec) Refs() []actiontemplate.Ref {
	refs := actiontemplate.Parse(s.PathTemplate)
	for _, k := range sortedKeys(s.Query) {
		refs = append(refs, actiontemplate.Parse(s.Query[k])...)
	}
	for _, k := range sortedKeys(s.Headers) {
		refs = append(refs, actiontemplate.Parse(s.Headers[k])...)
	}
	if len(s.BodyTemplate) > 0 {
		var body any
		if json.Unmarshal(s.BodyTemplate, &body) == nil {
			refs = append(refs, actiontemplate.Refs(body)...)
		}
	}
	return append(refs, actiontemplate.Refs(s.BoundArguments)...)
}

/*
UsesContact reports whether the action binds the customer's identity,
which makes it available only where that identity is proven.
*/
func (s Spec) UsesContact() bool {
	return slices.ContainsFunc(s.Refs(), actiontemplate.Ref.UsesContact)
}

/*
NeedsAgreement reports whether a call waits for anyone before it runs.
*/
func (s Spec) NeedsAgreement() bool {
	return s.Confirmation == ConfirmationCustomer || s.Approval != ApprovalNone
}

/*
boundNames lists the arguments the admin bound.
*/
func (s Spec) boundNames() []string {
	names := make([]string, 0, len(s.BoundArguments))
	for k := range s.BoundArguments {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

/*
deref reads a nullable string.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

/*
sortedKeys lists a map's keys in order.
*/
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
