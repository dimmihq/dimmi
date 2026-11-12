/*
|--------------------------------------------------------------------------
| Tool Catalog: AI Actions
|--------------------------------------------------------------------------
|
| The entries for the AI agent's actions: the connections to the
| workspace's own systems, the actions drafted, tested and published on
| them, and the calls the agents made with their approvals.
|
*/

package mcpserver

import "maps"

/*
aiActionDocs returns the entries for AI actions, their connections
and their calls.
*/
func aiActionDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		connectionDocs(), connectionCheckDocs(), actionDocs(), actionEditDocs(), actionCallDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
connectionDocs returns the entries for listing and changing action
connections.
*/
func connectionDocs() map[string]Doc {
	return map[string]Doc{
		"ai.action-connections.index": {
			Title: "List action connections",
			Use: "Lists the connections to the workspace's own systems that AI actions call: an HTTP API or an " +
				"MCP server, each with its address and credential. Secrets never come back.",
			Returns: "every connection with its kind, base url, auth type, status, last error and action count.",
		},
		"ai.action-connections.show": {
			Title:   "Get an action connection",
			Use:     "Reads one connection to a workspace system. The secret never comes back, only a hint.",
			Returns: "the connection with its status, last success and last error.",
		},
		"ai.action-connections.store": {
			Title: "Add an action connection",
			Use: "Connects one of the workspace's own systems so actions can call it. Nothing runs until an " +
				"action on it is published. Only store a credential the person gave you for this.",
			Returns: "the connection, the secret hidden.",
			Next: []string{
				"dimmi_ai_action_connections_tests_create: check dimmi can reach it",
				"dimmi_ai_action_connections_tool_syncs_create: turn an MCP server's tools into draft actions",
				"dimmi_ai_actions_create: draft an action on it",
			},
			Params: map[string]string{
				"kind":      "http for an HTTP API, mcp for an MCP server. Fixed once created.",
				"base_url":  "The https address every action's path is added to; requests can never leave it.",
				"auth_type": "none, bearer, api_key_header, api_key_query or basic.",
				"token":     "The bearer token or API key. Written only, never returned.",
				"username":  "Username for basic auth, with password.",
				"variables": "Named values templates can use as {connection.name}.",
			},
		},
		"ai.action-connections.update": {
			Title: "Change an action connection",
			Use: "Changes a connection's address, credential, headers or status. Every published action on it " +
				"uses the change at once. A new token, username or password replaces the stored secret.",
			Returns: "the connection as it now is.",
			Risk:    new(RiskDanger),
			Params:  map[string]string{"status": "active, or disabled to stop every action on it."},
		},
		"ai.action-connections.destroy": {
			Title: "Remove an action connection",
			Use: "Deletes a connection and its secret. Refused while actions use it unless force is set, which " +
				"also deletes those actions and cancels their calls still waiting.",
			Params: map[string]string{"force": "true also deletes the actions on this connection."},
			Hint: "connection_in_use means actions still use it: delete them, or pass force=true if the " +
				"person agrees.",
		},
	}
}

/*
connectionCheckDocs returns the entries for probing a connection and
syncing an MCP server's tools.
*/
func connectionCheckDocs() map[string]Doc {
	return map[string]Doc{
		"ai.action-connections.tests.store": {
			Title: "Check an action connection",
			Use: "Sends one request to the connection to see whether dimmi can reach it and sign in. For an " +
				"MCP server it lists the tools.",
			Returns: "how the check went: status, reason, HTTP status, latency and the MCP tool count.",
			Params:  map[string]string{"path": "Optional path under the base url to probe, such as /health."},
		},
		"ai.action-connections.tool-syncs.store": {
			Title: "Import MCP tools as actions",
			Use: "Reads an MCP server connection's tools and creates a draft action for each one that has none. " +
				"Drafts start as writes that ask the customer to confirm; nothing is live until published.",
			Returns: "the draft actions created, how many tools already had one, and the tools skipped with why.",
			Next: []string{
				"dimmi_ai_actions_list: review the new drafts",
				"dimmi_ai_actions_update: mark a tool that only reads as side_effect=read",
			},
			Hint: "connection_not_mcp means this only works on an mcp connection.",
		},
	}
}

/*
actionDocs returns the entries for reading AI actions and their
versions.
*/
func actionDocs() map[string]Doc {
	return map[string]Doc{
		"ai.actions.index": {
			Title: "List AI actions",
			Use: "Lists the actions the agents can take in the workspace's own systems, such as looking up an " +
				"order or issuing a refund, by name.",
			Returns: "every action with its status, side effect, who must agree, and its live version.",
			Params: map[string]string{
				"status":        "draft, published or disabled.",
				"connection_id": "Only actions on this connection (acn_...).",
			},
		},
		"ai.actions.show": {
			Title:   "Get an AI action",
			Use:     "Reads one action's working copy: its request, input schema, response fields and agreement rules.",
			Returns: "the action with has_unpublished_changes and any warnings.",
		},
		"ai.actions.versions.index": {
			Title:   "List an action's versions",
			Use:     "Lists the published versions of an action, newest first. Each is frozen once published.",
			Returns: "every version with its number, definition, note and who published it.",
			Next:    []string{"dimmi_ai_actions_update: restore a version with restore_version_id, then publish it"},
		},
		"ai.actions.destroy": {
			Title: "Delete an AI action",
			Use: "Deletes an action so no agent can take it any more. Its past calls stay. To stop it for now, " +
				"set status=disabled with dimmi_ai_actions_update instead.",
		},
		"ai.actions.publications.store": {
			Title: "Publish an AI action",
			Use: "Freezes the action's working copy as a new version and makes it live: the agents can take it " +
				"in customer conversations at once. Test it first and publish only when the person agrees.",
			Returns: "the new version with its number.",
			Risk:    new(RiskReach),
			Params:  map[string]string{"note": "What changed, for the version history, up to 500 characters."},
			Hint:    "nothing_to_publish means the live version already matches the working copy.",
		},
	}
}

/*
actionEditDocs returns the entries for drafting, editing and testing
AI actions.
*/
func actionEditDocs() map[string]Doc {
	fields := map[string]string{
		"name":         "Tool name the model sees: 3 to 48 lowercase letters, digits and underscores.",
		"description":  "When to use the action, 10 to 1024 characters; the model reads it to decide.",
		"side_effect":  "read runs at once; write changes something and may wait for agreement. Default write.",
		"confirmation": "none, or customer to have the customer confirm a write first (the default).",
		"approval":     "none, teammate or admin: who on the team must approve a write before it runs.",
		"path_template": "Path under the connection's base url; placeholders are {input.x}, {contact.email}, " +
			"{contact.external_id}, {contact.attr.key} and {connection.x}.",
		"input_schema":  "JSON Schema of what the model fills in.",
		"channels":      "Where dimmi may take it, such as widget. Default widget.",
		"agents":        "dimmi (answers customers) and operator (works for the team). Default dimmi only.",
		"mcp_tool_name": "The MCP server's tool this action calls, on an mcp connection.",
	}

	return map[string]Doc{
		"ai.actions.store": {
			Title: "Draft an AI action",
			Use: "Drafts an action on a connection: the request it sends, what the model fills in and who must " +
				"agree. A draft never runs for customers until published.",
			Returns: "the draft action with any warnings.",
			Next:    []string{"dimmi_ai_actions_tests_create: try it before publishing"},
			Params:  fields,
		},
		"ai.actions.update": {
			Title: "Change an AI action",
			Use: "Edits the working copy, restores a version with restore_version_id, or switches a published " +
				"action with status=disabled or published, which changes what customers get at once.",
			Returns: "the action as it now is.",
			Risk:    new(RiskReach),
			Params:  fields,
		},
		"ai.actions.tests.store": {
			Title: "Test an AI action",
			Use: "Tries the working copy with sample input and a stand-in customer. mode=dry_run (default) " +
				"renders the request and sends nothing. mode=live sends it to the real system; a live write " +
				"changes a real account, so run it only when the person asks.",
			Returns: "the result: rendered request, response, what the model would read and any validation errors.",
			Risk:    new(RiskDanger),
			Params: map[string]string{
				"input":                "Sample input matching the action's input_schema.",
				"contact":              "Stand-in customer, treated as verified: email, external_id and attributes.",
				"mode":                 "dry_run (default) or live.",
				"confirm_side_effects": "Must be true for a live test of a write.",
			},
			Hint: "side_effects_not_confirmed means a live write needs confirm_side_effects=true and the person's yes.",
		},
	}
}

/*
actionCallDocs returns the entries for the calls the agents made.
*/
func actionCallDocs() map[string]Doc {
	return map[string]Doc{
		"ai.action-calls.index": {
			Title:   "List AI action calls",
			Use:     "Lists the action calls the agents made, newest first, including those waiting for a teammate.",
			Returns: "a page of calls with their state, action, conversation and summary, with next_cursor for more.",
			Params: map[string]string{
				"state": "awaiting_confirmation (the customer), pending_approval (the team), approved, declined, " +
					"expired, canceled, running, succeeded or failed.",
				"conversation_id": "Only calls in this conversation (conv_...).",
				"action_id":       "Only calls of this action (aac_...).",
			},
		},
		"ai.action-calls.show": {
			Title:   "Get an AI action call",
			Use:     "Reads one call: what the agent asked to do, who agreed, and how it went.",
			Returns: "the call with its state, timings and, for those allowed, its input.",
		},
		"ai.action-calls.update": {
			Title: "Approve or decline an AI action call",
			Use: "Decides a call waiting for the team. Approving runs it in the workspace's system at once; " +
				"declining has the agent tell the customer. Show the person what the call does and decide only " +
				"as they say.",
			Returns: "the call in its new state.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"status":           "approved or declined.",
				"reason":           "Why, for the team only.",
				"customer_message": "After a decline, what the agent passes on to the customer.",
			},
			Hint: "action_call_not_pending means the call was already decided or expired.",
		},
	}
}
