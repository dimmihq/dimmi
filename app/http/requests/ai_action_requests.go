/*
|--------------------------------------------------------------------------
| Agent Action Requests
|--------------------------------------------------------------------------
|
| The inputs of the action endpoints: actions edited field by field. What
| each value may be is checked by the actions.
|
*/

package requests

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/app/support/actionresult"
)

/*
AIActionRequest names an action in the path.
*/
type AIActionRequest struct {
	Action string `path:"action"`
}

/*
AIActionsRequest filters the action list.
*/
type AIActionsRequest struct {
	Status     string `query:"status"        validate:"omitempty,oneof=draft published disabled"`
	Connection string `query:"connection_id"`
}

/*
AIActionBody is an action's fields. BodyTemplate sent as null clears
the body; RawBody tells it from a body left out. RestoreVersionID makes
the working copy that version's definition again.
*/
type AIActionBody struct {
	ConnectionID           *string                       `json:"connection_id,omitempty"`
	Name                   *string                       `json:"name,omitempty"`
	Title                  *string                       `json:"title,omitempty"`
	Description            *string                       `json:"description,omitempty"`
	SideEffect             *string                       `json:"side_effect,omitempty"`
	Confirmation           *string                       `json:"confirmation,omitempty"`
	Approval               *string                       `json:"approval,omitempty"`
	ApprovalTimeoutMinutes *int32                        `json:"approval_timeout_minutes,omitempty"`
	Method                 *string                       `json:"method,omitempty"`
	PathTemplate           *string                       `json:"path_template,omitempty"`
	Query                  *map[string]string            `json:"query,omitempty"`
	Headers                *map[string]string            `json:"headers,omitempty"`
	BodyTemplate           json.RawMessage               `json:"body_template,omitempty"`
	BodyEncoding           *string                       `json:"body_encoding,omitempty"`
	MCPToolName            *string                       `json:"mcp_tool_name,omitempty"`
	BoundArguments         *map[string]any               `json:"bound_arguments,omitempty"`
	InputSchema            json.RawMessage               `json:"input_schema,omitempty"`
	ResponseFields         *[]actionresult.ResponseField `json:"response_fields,omitempty"`
	ErrorMessagePath       *string                       `json:"error_message_path,omitempty"`
	ResponseMaxBytes       *int32                        `json:"response_max_bytes,omitempty"`
	TimeoutMS              *int32                        `json:"timeout_ms,omitempty"`
	Channels               *[]string                     `json:"channels,omitempty"`
	Status                 *string                       `json:"status,omitempty"`
	RestoreVersionID       *string                       `json:"restore_version_id,omitempty"`
}

/*
AIStoreActionRequest is the input of POST /ai/actions.
*/
type AIStoreActionRequest struct {
	Body    AIActionBody
	RawBody []byte
}

/*
AIUpdateActionRequest is the input of PATCH /ai/actions/{action}.
*/
type AIUpdateActionRequest struct {
	Action  string `path:"action"`
	Body    AIActionBody
	RawBody []byte
}
