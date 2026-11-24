/*
|--------------------------------------------------------------------------
| Written Schema Texts: AIAction To AIAgent
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromAIAction returns the texts from AIAction on.
*/
func schemaTextsFromAIAction() map[string]map[string]string {
	return map[string]map[string]string{
		"AIAction": {
			"agents": "Which agents may use the action: dimmi (answers customers), operator (works for " +
				"the team), or both.",
			"approval": "Who on the team must approve a call before it runs: none, teammate or admin. " +
				"Only a write action can need approval.",
			"approval_timeout_minutes": "Minutes a call waits for a teammate's approval before it " +
				"expires, from 1 to 10080. Defaults to 60.",
			"body_encoding": "How the request body is sent: json or form.",
			"body_template": "The request body as JSON, with placeholders such as {input.order_id}, " +
				"{contact.email} or {connection.region} filled in at call time. Null when the request has no " +
				"body.",
			"bound_arguments": "Arguments of an MCP tool that the admin fills in and the model never " +
				"sees, by argument name. Values may hold placeholders such as {contact.email}. Empty when " +
				"none.",
			"channels": "Channels on which the action is offered, such as [\"widget\"]. Defaults to " +
				"the widget alone.",
			"confirmation": "Whether the customer must confirm a call before it runs: none or " +
				"customer. Only a write action can ask for it.",
			"connection_id": "Id of the connection the action sends its requests through, such as " +
				"acn_... Send it wherever an action connection is asked for.",
			"description": "What the action does, written for the model that decides when to call it. " +
				"10 to 1024 characters.",
			"error_message_path": "Dotted path into a failed response's body where the error message " +
				"is found, such as error.message. Null when none is set.",
			"has_unpublished_changes": "True when this working copy differs from the published " +
				"version, or nothing was published yet. The agent only runs the published version.",
			"headers": "Extra request headers by name. Values may hold placeholders such as " +
				"{input.locale}. Empty when none.",
			"input_schema": "JSON Schema object of the arguments the model must give when it calls the " +
				"action.",
			"kind": "How the action runs: http sends a request to an API, mcp_tool calls a tool on an " +
				"MCP server.",
			"last_test_response": "The result of the last successful live test of the action, kept to " +
				"help map response fields. Left out when there is none.",
			"mcp_tool_name": "Name of the tool called on the MCP server. Null for an http action.",
		},
	}
}

/*
schemaTextsFromAIActionAndAIActionCall returns the texts from AIAction on.
*/
func schemaTextsFromAIActionAndAIActionCall() map[string]map[string]string {
	return map[string]map[string]string{
		"AIAction": {
			"method": "HTTP method of the request: GET, POST, PUT, PATCH or DELETE. Null for an " +
				"mcp_tool action.",
			"name": "The tool name the model calls the action by: 3 to 48 lowercase letters, digits " +
				"and underscores, such as lookup_order.",
			"path_template": "Path added to the connection's base URL, with placeholders, such as " +
				"/orders/{input.order_id}. Null for an mcp_tool action.",
			"query": "Query string parameters by name. Values may hold placeholders such as " +
				"{input.status}. Empty when none.",
			"response_fields": "Which values of the response the model gets to read, each a name and a " +
				"dotted path into the body. Empty when nothing is mapped.",
			"response_max_bytes": "Largest response body the action accepts, in bytes, from 1024 to " +
				"262144. Defaults to 65536.",
			"side_effect": "Whether a call only reads data (read) or changes something (write). A read " +
				"runs at once; a write may wait for confirmation or approval.",
			"source": "How the action was made: dashboard, api or mcp_sync (created from an MCP " +
				"server's tool list).",
			"status": "Where the action stands: draft (never published), published (the agent can use " +
				"it) or disabled (switched off).",
			"timeout_ms": "How long a call may take, in milliseconds, from 1000 to 15000. Null uses " +
				"the connection's timeout.",
			"title": "Short name people see for the action, up to 80 characters, such as Look up an " +
				"order.",
			"warnings": "Notes about the action's definition. Currently always an empty list.",
		},
		"AIActionCall": {
			"action_id": "Id of the action that was called, such as aac_... Send it wherever an action " +
				"is asked for.",
			"action_version_id": "Id of the published version of the action that ran, such as aacv_... " +
				"Null when no version is recorded.",
			"approval": "Who on the team had to approve the call: none, teammate or admin.",
			"cancel_reason": "Why the call was canceled: customer (the customer declined to confirm) " +
				"or superseded (the agent proposed a newer write). Null when not canceled.",
			"completed_at": "When the call finished or was canceled. Null while it still waits or " +
				"runs.",
			"confirmation": "Whether the customer had to confirm the call: none or customer.",
		},
	}
}

/*
schemaTextsFromAIActionCall returns the texts from AIActionCall on.
*/
func schemaTextsFromAIActionCall() map[string]map[string]string {
	return map[string]map[string]string{
		"AIActionCall": {
			"confirmed_at": "When the customer confirmed the call. Null when no confirmation was asked " +
				"or it has not been given.",
			"connection_id": "Id of the connection the call went through, such as acn_...",
			"conversation_id": "Id of the conversation the call was made in, such as conv_... Null for " +
				"a call made outside a conversation, such as a test.",
			"customer_message": "The message the agent passes on to the customer after a teammate " +
				"declined the call. Null when none was written.",
			"decided_at": "When a teammate approved or declined the call. Null when no approval was " +
				"needed or none has been given.",
			"decided_by_member_id": "Id of the teammate who approved or declined the call, such as " +
				"mem_... Null when nobody decided.",
			"decision_reason": "The teammate's internal reason for approving or declining, never shown " +
				"to the customer. Null when none was given.",
			"error_code": "Why the call failed, such as auth_failed, not_found, upstream_rejected, " +
				"rate_limited, unavailable, host_blocked or response_too_large. Null when it did not fail.",
			"expires_at": "When a call waiting for verification, confirmation or approval lapses. Null " +
				"once it no longer waits.",
			"http_status": "HTTP status the workspace's API answered with, such as 200 or 404. Null " +
				"when no answer was received.",
			"input": "The arguments the model gave, frozen when the call was proposed. Left out unless " +
				"the caller may read payloads.",
			"latency_ms": "How long the request took, in milliseconds. Null when nothing was sent.",
			"message_id": "Id of the agent's message that told the customer about the call, such as " +
				"msg_... Null when there is none.",
			"mode": "Whether the call was real or a trial: live for a call in a conversation, test for " +
				"a test of the action.",
			"reported_at": "When the customer was told how the call ended, or it was settled that " +
				"nobody needs to be told. Null until then.",
			"request": "The request that was sent, as JSON. Left out unless the caller may read " +
				"payloads.",
			"requested_at": "When the agent asked for the call.",
			"response": "What the workspace's API or MCP server answered, as JSON. Left out unless the " +
				"caller may read payloads.",
		},
	}
}

/*
schemaTextsFromAIActionCallAndAIActionConnection returns the texts from
AIActionCall on.
*/
func schemaTextsFromAIActionCallAndAIActionConnection() map[string]map[string]string {
	return map[string]map[string]string{
		"AIActionCall": {
			"result": "The shaped result the model read: only the mapped response fields, or an error " +
				"and message. Left out unless the caller may read payloads.",
			"run_id": "Id of the agent run that asked for the call, such as air_... Null when there is " +
				"none.",
			"sent_at":     "When the request was sent. Null when nothing has been sent yet.",
			"side_effect": "Whether the call only reads data (read) or changes something (write).",
			"state": "Where the call stands: awaiting_verification, awaiting_confirmation, " +
				"pending_approval, approved, declined, expired, canceled, running, succeeded or failed.",
			"summary": "Plain sentence of what the call will do, built from the checked arguments. " +
				"This is what a customer confirms or a teammate approves.",
			"title": "The action's short name at the time of the call, such as Refund an order.",
		},
		"AIActionConnection": {
			"actions_count": "How many actions use this connection. Counted in lists; 0 when the " +
				"connection is read alone.",
			"auth_type": "How requests prove who they are: none, bearer, api_key_header, api_key_query " +
				"or basic.",
			"base_url": "The address every request of this connection starts from, such as " +
				"https://api.example.com, or the MCP server's URL.",
			"default_headers": "Headers sent with every request of this connection, by name. Empty " +
				"when none.",
			"has_secret": "True when a token or password is stored for the connection. The secret " +
				"itself is never returned.",
			"kind": "What the connection reaches: http for an HTTP API, mcp for a remote MCP server.",
			"last_error": "Error code of the last failed call or test, such as auth_failed or " +
				"unavailable. Null when none has failed.",
			"last_error_at": "When a call or test through this connection last failed. Null when none " +
				"has.",
			"last_success_at": "When a call or test last reached the connection. Null when none has.",
			"name":            "The name the team gave the connection, such as Orders API. Up to 60 characters.",
		},
	}
}

/*
schemaTextsFromAIActionConnection returns the texts from AIActionConnection on.
*/
func schemaTextsFromAIActionConnection() map[string]map[string]string {
	return map[string]map[string]string{
		"AIActionConnection": {
			"secret_hint": "The last four characters of the stored secret after an ellipsis, such as " +
				"…9f2c, to tell keys apart. Null when there is no secret or it is under 8 characters.",
			"status": "Whether the connection can be used: active, error (its actions are not offered " +
				"until a test passes) or disabled.",
			"status_reason": "Why the connection is in error: host_blocked, or auth after three " +
				"refused credentials in a row. Null otherwise.",
			"timeout_ms": "How long a request may take, in milliseconds, from 1000 to 15000. Defaults " +
				"to 8000.",
			"tools_synced_at": "When the MCP server's tools were last synced into actions. Null when " +
				"never synced.",
			"variables": "Plain named values that request templates read as {connection.<name>}, such " +
				"as a region or account id. Not for secrets. Empty when none.",
		},
		"AIActionConnectionTest": {
			"http_status": "HTTP status the server answered the test with, such as 200. Null when no " +
				"answer was received.",
			"latency_ms": "How long the test took, in milliseconds.",
			"status":     "How the test went: ok or error.",
			"status_reason": "Why the test failed, such as credentials, url_escape, auth or " +
				"unreachable. Null when it passed.",
			"tools_count": "How many tools the MCP server listed. Null for an http connection.",
		},
		"AIActionTest": {
			"call_id": "Id of the call recorded for a live test, such as aax_... Null for a dry run or " +
				"a test that was stopped before sending.",
			"confirmation_text": "The sentence a customer would be asked to confirm. Same text as " +
				"summary.",
			"error_code": "Why the test failed or was blocked, such as invalid_arguments, " +
				"identity_unavailable, auth_failed or not_found. Null when it went through.",
			"latency_ms": "How long the request took, in milliseconds. 0 when nothing was sent.",
			"request": "The request as it was or would be sent, with placeholders filled in. Null when " +
				"it could not be built.",
			"response": "What the API or MCP server answered in a live test. Null for a dry run.",
		},
	}
}

/*
schemaTextsFromAIActionTest returns the texts from AIActionTest on.
*/
func schemaTextsFromAIActionTest() map[string]map[string]string {
	return map[string]map[string]string{
		"AIActionTest": {
			"result": "The shaped result the model would read from the response. Null for a dry run.",
			"status": "How the test went: rendered (dry run built the request), succeeded, failed, or " +
				"blocked (the request could not be built).",
			"summary": "Plain sentence of what the call would do, built from the sample arguments.",
			"validation_errors": "What was wrong with the sample arguments when they did not fit the " +
				"action's input schema. Empty when they fit.",
			"warnings": "Notes about the test. Currently always an empty list.",
		},
		"AIActionToolSync": {
			"created": "The draft actions the sync made, one for each new tool of the MCP server. " +
				"Empty when there were no new tools.",
			"disabled": "Always an empty list: a sync never disables an action it made before.",
			"existing": "How many of the server's tools already had an action and were left alone.",
			"skipped": "Tools the sync could not turn into an action, each with the tool's name and " +
				"the reason. Empty when none.",
			"updated": "Always an empty list: a sync never changes an action it made before.",
		},
		"AIActionVersion": {
			"action_id": "Id of the action this version belongs to, such as aac_...",
			"note":      "What the publisher wrote about this version. Null when they left no note.",
			"number":    "The version's number within its action, starting at 1.",
			"spec": "The action's full definition as it was frozen when published: name, request, " +
				"input schema, response fields and who must agree.",
		},
		"AIActionVersionRef": {
			"number": "The published version's number within its action, starting at 1.",
		},
		"AIAgent": {
			"actions_enabled": "Whether the agent may use the workspace's actions at all.",
			"always_handoff_tag_ids": "Ids of the tags that always send a conversation to the team, " +
				"such as tag_... Empty when none.",
			"answer_key_id": "Id of the provider connection the answer model runs on, such as aik_... " +
				"Null when none is chosen.",
		},
	}
}

/*
schemaTextsFromAIAgent returns the texts from AIAgent on.
*/
func schemaTextsFromAIAgent() map[string]map[string]string {
	return map[string]map[string]string{
		"AIAgent": {
			"answer_model": "Id of the model that writes the agent's answers. Null when none is " +
				"chosen, and then the agent cannot go live.",
			"assigned_behavior": "What the agent does in a conversation a teammate holds: " +
				"skip_when_human_active, never_when_assigned or always.",
			"default_language": "Language tag the agent falls back to, such as en.",
			"enabled":          "Whether the agent is switched on and answers customers.",
			"enabled_channels": "Channels the agent answers on: any of widget, email, api and slack. " +
				"With none the agent is not ready.",
			"fallback_key_id": "Id of the provider connection the fallback model runs on, such as " +
				"aik_... Null when no fallback is set.",
			"fallback_model": "Id of the model used as a fallback for answers. Null when none is set.",
			"grounding_mode": "How answers are kept to the sources: citations or verify.",
			"guidance": "The team's own instructions to the agent, in plain text up to 2000 " +
				"characters. Empty when none.",
			"handoff_mode": "The agent's role in a conversation: full (it answers) or triage_only (it " +
				"only sorts the conversation and hands it to a person).",
			"handoff_team_id": "Id of the team a conversation goes to when the agent hands off, such " +
				"as team_... Null when no team is set.",
			"include_contact_context": "How much the agent reads about the contact: none, basic or " +
				"allowlist.",
			"languages": "Language tags set for the agent, such as [\"en\",\"de\"]. Empty when none " +
				"are set.",
			"link_allowlist": "Host names set as allowed for links in the agent's answers, such as " +
				"docs.example.com or *.example.com. Empty when none.",
			"max_ai_replies": "The most replies the agent gives in one conversation before it hands " +
				"off to the team, from 1 to 50. Defaults to 5.",
			"max_output_tokens": "The most tokens the agent may write in one answer. Defaults to 600.",
			"monthly_spend_cap_micros": "The most the agent may spend on models in a month, in " +
				"millionths of a US dollar (25000000 is $25). Null when there is no cap.",
			"name":          "The name the agent goes by with customers, 1 to 60 characters.",
			"pii_redaction": "Whether redaction of personal data is switched on for the agent.",
			"product_name": "The name of the product the agent supports, up to 80 characters. Null " +
				"when not set.",
			"raise_priority_on_handoff": "Whether a conversation's priority is raised when the agent " +
				"hands it to the team.",
		},
	}
}
