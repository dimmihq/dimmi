/*
|--------------------------------------------------------------------------
| Definition Fields
|--------------------------------------------------------------------------
|
| How an admin's input becomes a definition and a row. Defaults lean
| safe: a request that is not a GET writes, an MCP tool writes until the
| admin says it only reads, and a write asks the customer to confirm.
| Turning an action into a read clears who must agree, since reads run
| inline.
|
*/

package agentactions

import (
	"encoding/json"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/models"
)

/*
newSpec starts a definition on the connection with conservative
defaults, then applies the input.
*/
func newSpec(conn models.AiActionConnection, in DefinitionInput) actionkit.Spec {
	s := actionkit.Spec{
		ConnectionID: conn.ID, Kind: actionkit.KindHTTP, Method: "GET",
		Confirmation: actionkit.ConfirmationNone, Approval: actionkit.ApprovalNone,
	}
	if conn.Kind == "mcp" {
		s.Kind, s.Method = actionkit.KindMCPTool, ""
	}
	if in.Method != nil {
		s.Method = *in.Method
	}
	s.SideEffect = actionkit.SideEffectWrite
	if s.Kind == actionkit.KindHTTP && strings.EqualFold(s.Method, "get") {
		s.SideEffect = actionkit.SideEffectRead
	}
	if in.SideEffect != nil {
		s.SideEffect = *in.SideEffect
	}
	if s.SideEffect == actionkit.SideEffectWrite {
		s.Confirmation = actionkit.ConfirmationCustomer
	}
	return apply(s, in)
}

/*
apply writes the input's fields onto a definition.
*/
func apply(s actionkit.Spec, in DefinitionInput) actionkit.Spec {
	if in.SideEffect != nil && *in.SideEffect != s.SideEffect {
		s.SideEffect = *in.SideEffect
		s.Confirmation, s.Approval = actionkit.ConfirmationNone, actionkit.ApprovalNone
		if s.SideEffect == actionkit.SideEffectWrite {
			s.Confirmation = actionkit.ConfirmationCustomer
		}
	}
	s = applyText(s, in)
	return applyValues(s, in)
}

/*
applyText writes the input's text and number fields.
*/
func applyText(s actionkit.Spec, in DefinitionInput) actionkit.Spec {
	setString(&s.Name, in.Name)
	setString(&s.Title, in.Title)
	setString(&s.Description, in.Description)
	setString(&s.Confirmation, in.Confirmation)
	setString(&s.Approval, in.Approval)
	setString(&s.Method, in.Method)
	setString(&s.PathTemplate, in.PathTemplate)
	setString(&s.BodyEncoding, in.BodyEncoding)
	setString(&s.MCPToolName, in.MCPToolName)
	setString(&s.ErrorMessagePath, in.ErrorMessagePath)
	setInt(&s.ApprovalTimeoutMinutes, in.ApprovalTimeoutMinutes)
	setInt(&s.ResponseMaxBytes, in.ResponseMaxBytes)
	setInt(&s.TimeoutMS, in.TimeoutMS)
	return s
}

/*
applyValues writes the input's maps, lists and JSON fields.
*/
func applyValues(s actionkit.Spec, in DefinitionInput) actionkit.Spec {
	if in.Query != nil {
		s.Query = *in.Query
	}
	if in.Headers != nil {
		s.Headers = *in.Headers
	}
	if in.BoundArguments != nil {
		s.BoundArguments = *in.BoundArguments
	}
	if in.InputSchema != nil {
		s.InputSchema = *in.InputSchema
	}
	if in.ResponseFields != nil {
		s.ResponseFields = *in.ResponseFields
	}
	if in.Channels != nil {
		s.Channels = *in.Channels
	}
	if in.BodyTemplate != nil {
		s.BodyTemplate = *in.BodyTemplate
		if string(s.BodyTemplate) == "null" {
			s.BodyTemplate = nil
		}
	}
	return s
}

/*
editsDefinition reports whether the input changes the working copy, not
just the status.
*/
func (in DefinitionInput) editsDefinition() bool {
	strs := []*string{
		in.Name, in.Title, in.Description, in.SideEffect, in.Confirmation, in.Approval, in.Method,
		in.PathTemplate, in.BodyEncoding, in.MCPToolName, in.ErrorMessagePath,
	}
	for _, p := range strs {
		if p != nil {
			return true
		}
	}
	return in.ApprovalTimeoutMinutes != nil || in.ResponseMaxBytes != nil || in.TimeoutMS != nil ||
		in.Query != nil || in.Headers != nil || in.BodyTemplate != nil || in.BoundArguments != nil ||
		in.InputSchema != nil || in.ResponseFields != nil || in.Channels != nil
}

/*
columns are a definition's values as the action row stores them.
*/
type columns struct {
	query, headers, bound, fields json.RawMessage
	body                          json.RawMessage
	method, path, tool, errPath   *string
	timeout                       *int32
}

/*
columnsOf encodes a definition for the row.
*/
func columnsOf(s actionkit.Spec) columns {
	c := columns{
		method: nilText(s.Method), path: nilText(s.PathTemplate), tool: nilText(s.MCPToolName),
		errPath: nilText(s.ErrorMessagePath), body: s.BodyTemplate,
	}
	c.query, _ = json.Marshal(s.Query)
	c.headers, _ = json.Marshal(s.Headers)
	c.bound, _ = json.Marshal(s.BoundArguments)
	c.fields, _ = json.Marshal(s.ResponseFields)
	if s.TimeoutMS != 0 {
		c.timeout = &s.TimeoutMS
	}
	return c
}

/*
createParams are the row of a new action.
*/
func createParams(ws ids.UUID, s actionkit.Spec, in DefinitionInput, c clock.Clock) models.CreateActionParams {
	col, member := columnsOf(s), in.Membership.Member.ID
	source := in.Source
	if source == "" {
		source = "api"
	}
	return models.CreateActionParams{
		WorkspaceID: ws, ConnectionID: s.ConnectionID, Name: s.Name, Title: s.Title, Description: s.Description,
		Kind: s.Kind, SideEffect: s.SideEffect, Confirmation: s.Confirmation, Approval: s.Approval,
		ApprovalTimeoutMinutes: s.ApprovalTimeoutMinutes, Method: col.method, PathTemplate: col.path,
		Query: col.query, Headers: col.headers, BodyTemplate: col.body, BodyEncoding: s.BodyEncoding,
		McpToolName: col.tool, BoundArguments: col.bound, InputSchema: s.InputSchema, ResponseFields: col.fields,
		ErrorMessagePath: col.errPath, ResponseMaxBytes: s.ResponseMaxBytes, TimeoutMs: col.timeout,
		Channels: s.Channels, SpecHash: s.Hash(), Source: source, MemberID: &member, Now: c.Now(),
	}
}

/*
updateParams are the row of an edited working copy.
*/
func updateParams(a models.AiAction, s actionkit.Spec, in DefinitionInput, c clock.Clock) models.UpdateActionParams {
	col, member := columnsOf(s), in.Membership.Member.ID
	return models.UpdateActionParams{
		WorkspaceID: a.WorkspaceID, ID: a.ID, Name: s.Name, Title: s.Title, Description: s.Description,
		SideEffect: s.SideEffect, Confirmation: s.Confirmation, Approval: s.Approval,
		ApprovalTimeoutMinutes: s.ApprovalTimeoutMinutes, Method: col.method, PathTemplate: col.path,
		Query: col.query, Headers: col.headers, BodyTemplate: col.body, BodyEncoding: s.BodyEncoding,
		McpToolName: col.tool, BoundArguments: col.bound, InputSchema: s.InputSchema, ResponseFields: col.fields,
		ErrorMessagePath: col.errPath, ResponseMaxBytes: s.ResponseMaxBytes, TimeoutMs: col.timeout,
		Channels: s.Channels, SpecHash: s.Hash(), MemberID: &member, Now: c.Now(),
	}
}

/*
setString applies an optional string.
*/
func setString(dst, v *string) {
	if v != nil {
		*dst = *v
	}
}

/*
setInt applies an optional number.
*/
func setInt(dst, v *int32) {
	if v != nil {
		*dst = *v
	}
}

/*
nilText maps an empty string to NULL.
*/
func nilText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

/*
invalidField is a 422 on a field.
*/
func invalidField(code, field, message string) error {
	return fault.New(fault.Invalid, code, message).OnField(field)
}
