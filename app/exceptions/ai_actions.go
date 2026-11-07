/*
|--------------------------------------------------------------------------
| Agent Action Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a workspace defines the actions its agent may
| take, and while a customer or a teammate agrees to a call. A decision
| on a call that is no longer waiting says why, so a second click or a
| late approval never runs anything twice.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Definition failures.

ErrActionTemplateInvalid is a placeholder the action cannot fill.
ErrActionSchemaUnsupported is an input schema outside the portable
subset. ErrActionNameReserved is a tool name the agent keeps for
itself. ErrActionNameTaken is a tool name another action holds.
ErrActionKindMismatch is an action whose kind does not match its
connection's. ErrActionHostBlocked is a base URL the agent may never
call. ErrActionNotPublishable is a draft that fails a publish check.
ErrNothingToPublish is a working copy identical to the live version.
ErrConnectionInUse is a connection that live actions still use.
ErrActionInvalidTransition is a status change the action cannot make.
ErrSideEffectsNotConfirmed is a live test of a write the caller did not
confirm.
*/
var (
	ErrActionTemplateInvalid   = fault.New(fault.Invalid, "action_template_invalid", "A template is not valid.")
	ErrActionSchemaUnsupported = fault.New(fault.Invalid, "action_schema_unsupported",
		"The input schema is outside the supported subset.")
	ErrActionNameReserved = fault.New(fault.Invalid, "action_name_reserved", "This tool name is reserved.")
	ErrActionNameTaken    = fault.New(fault.Invalid, "action_name_taken", "Another action already uses this name.")
	ErrActionKindMismatch = fault.New(fault.Invalid, "action_kind_mismatch",
		"The action's kind does not match its connection.")
	ErrActionHostBlocked = fault.New(fault.Invalid, "action_host_blocked",
		"The agent may not call this address.")
	ErrActionNotPublishable = fault.New(fault.Invalid, "action_not_publishable",
		"The action cannot be published yet.")
	ErrNothingToPublish = fault.New(fault.Conflict, "nothing_to_publish",
		"The working copy is the same as the live version.")
	ErrConnectionInUse = fault.New(fault.Conflict, "connection_in_use",
		"Actions still use this connection.")
	ErrActionInvalidTransition = fault.New(fault.Invalid, "invalid_transition",
		"The action cannot move to that status.")
	ErrSideEffectsNotConfirmed = fault.New(fault.Invalid, "side_effects_not_confirmed",
		"A live test of a write action needs confirm_side_effects.")
	ErrConnectionNotMCP = fault.New(fault.Invalid, "connection_not_mcp",
		"Only an MCP connection has tools to sync.")
	ErrMCPServerIncompatible = fault.New(fault.Invalid, "mcp_server_incompatible",
		"The MCP server could not be reached or does not speak the protocol.")
)

/*
Call decisions.

ErrActionCallNotPending is a decision on a call that is no longer
waiting for one. ErrActionCallExpired is a decision that came after the
call's deadline. ErrActionCallNotFound answers any call the caller may
not see.
*/
var (
	ErrActionCallNotPending = fault.New(fault.Conflict, "action_call_not_pending",
		"This request is no longer waiting for a decision.")
	ErrActionCallExpired  = fault.New(fault.Conflict, "action_call_expired", "This request has expired.")
	ErrActionCallNotFound = fault.New(fault.NotFound, "not_found", "Not Found.")
)
