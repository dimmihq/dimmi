/*
|--------------------------------------------------------------------------
| Written Argument Texts: AI
|--------------------------------------------------------------------------
|
| The arguments of the tools for the AI agent, its actions and
| connections, provider keys, procedures and their simulations, and
| Operator's routines.
|
*/

package mcpserver

/*
aiTexts returns the texts written for these tools.
*/
func aiTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range []map[string]map[string]string{
		aiTextsPart1(), aiTextsPart2(), aiTextsPart3(), aiTextsPart4(), aiTextsPart5(), aiTextsPart6(),
		aiTextsPart7(), aiTextsPart8(),
	} {
		mergeTexts(out, part)
	}

	return out
}

/*
aiTextsPart1 returns texts of create_ai_action,
create_ai_action_connection.
*/
func aiTextsPart1() map[string]map[string]string {
	return map[string]map[string]string{
		"create_ai_action": {
			"body_encoding": "How the request body is sent: \"json\" or \"form\". Left out, it is \"json\". " +
				"With \"form\" the body_template must be a flat object of plain values, such as {\"email\": " +
				"\"{contact.email}\"}.",
			"body_template": "JSON body sent with the request, only for POST, PUT and PATCH. Values may hold " +
				"placeholders {input.x}, {contact.x} or {connection.x}, such as {\"order_id\": " +
				"\"{input.order_id}\", \"reason\": \"{input.reason}\"}. Send null to clear the body.",
			"bound_arguments": "For an action on an MCP connection: arguments you fix yourself, hidden from " +
				"the model and laid over what the model sends. An object of argument name to value; values may " +
				"hold placeholders, such as {\"account_id\": \"{connection.account_id}\", \"limit\": 10}.",
			"error_message_path": "Dotted path to the error message in the other system's JSON response, read " +
				"when a call fails. Letters, digits, _ , - and # joined by dots, at most 200 characters, such " +
				"as \"error.message\".",
			"headers": "Extra HTTP headers sent with the request, as an object of header name to value, at " +
				"most 10, such as {\"X-Api-Version\": \"2024-01\"}. Not allowed: Host, Cookie, Content-Length, " +
				"Content-Type, Transfer-Encoding, Authorization, Idempotency-Key, User-Agent, Connection and " +
				"any name starting with Dimmi-.",
			"method": "HTTP method of an action on an http connection: GET, POST, PUT, PATCH or DELETE " +
				"(lowercase is accepted). Left out on create, it is GET. Not used for an action on an mcp " +
				"connection. Such as \"POST\".",
			"response_fields": "Which values of the response the model reads, at most 30; left empty, it reads " +
				"the whole response. A list of {name, path, description}: name is lowercase letters, digits " +
				"and _, path is a dotted path into the JSON, such as [{\"name\": \"status\", \"path\": " +
				"\"order.status\", \"description\": \"Order status\"}].",
			"status": "Has no effect when creating: a new action always starts as a draft and goes live " +
				"through dimmi_publish_ai_action. Leave it out.",
		},
		"create_ai_action_connection": {
			"auth_config": "Non-secret auth settings as an object with header_name, query_param and prefix, " +
				"each optional. auth_type api_key_header needs header_name, such as {\"header_name\": " +
				"\"X-Api-Key\"}; api_key_query needs query_param, such as {\"query_param\": \"api_key\"}; " +
				"bearer may set prefix to replace \"Bearer\".",
		},
	}
}

/*
aiTextsPart2 returns texts of create_ai_action_connection, create_ai_key
and the tools after them.
*/
func aiTextsPart2() map[string]map[string]string {
	return map[string]map[string]string{
		"create_ai_action_connection": {
			"default_headers": "Headers sent with every request through this connection, as an object of " +
				"header name to value, at most 10, values at most 256 characters, such as {\"X-Api-Version\": " +
				"\"2024-01\"}. Not allowed: Authorization, Content-Type, Cookie, Host, User-Agent and names " +
				"starting with Dimmi-.",
			"status": "Must be \"active\" or \"disabled\" if sent, but has no effect when creating: a new " +
				"connection always starts active. Leave it out; use dimmi_update_ai_action_connection to " +
				"disable one.",
		},
		"create_ai_key": {
			"config": "Optional provider settings, as an object. Keys dimmi reads: anthropic_version, " +
				"max_tokens_field (text); supports_tools, supports_stream_usage, json_schema, prompt_cache " +
				"(true or false); context_tokens, max_output_tokens (numbers). Such as {\"supports_tools\": " +
				"false, \"context_tokens\": 32000}.",
		},
		"create_operator_routine": {
			"cadence": "How often the routine runs: \"daily\", \"weekly\", \"monthly\", \"cron\" or \"once\". " +
				"\"once\" also needs run_at; the exact time otherwise comes from cron_expression. A routine " +
				"runs at most once an hour. Such as \"weekly\".",
			"delivery": "Who is told about the routine's runs, as an object of id lists: in_app_member_ids and " +
				"email_member_ids (active teammates, at most 100 each) and slack_destination_ids (Slack " +
				"notification destinations, at most 20), such as {\"in_app_member_ids\": [\"mem_abc123\"], " +
				"\"slack_destination_ids\": [\"ndst_abc123\"]}.",
			"notify_policy": "When the routine tells people about a run: \"on_findings\" (only when it has " +
				"suggestions), \"always\" (after every run that was not skipped) or \"on_error_only\" (only " +
				"when a run fails). A pause is always told. Such as \"on_findings\".",
			"run_at": "The date and time a one-off routine runs, used only when cadence is \"once\". An ISO " +
				"8601 time still to come, such as \"2026-12-01T09:00:00Z\".",
			"scope": "What the routine works on beyond its inboxes, as an object: help_center_ids (at most 20 " +
				"help center ids) and github with installation_id, repositories (1 to 20, as owner/repo), " +
				"base_branch and ignore_labels, such as {\"help_center_ids\": [\"hc_abc123\"], \"github\": " +
				"{\"installation_id\": \"itg_abc123\", \"repositories\": [\"acme/app\"]}}.",
		},
	}
}

/*
aiTextsPart3 returns texts of create_operator_routine,
create_procedure_test_case and the tools after them.
*/
func aiTextsPart3() map[string]map[string]string {
	return map[string]map[string]string{
		"create_operator_routine": {
			"template_key": "The template the new routine starts from; fields you leave out take its values. " +
				"One of tpl_knowledge_gap_finder, tpl_github_pr_docs_sync, tpl_knowledge_staleness_audit, " +
				"tpl_qa_procedure_digest or tpl_custom. Left out, it is tpl_custom. Such as " +
				"\"tpl_knowledge_gap_finder\".",
		},
		"create_procedure_test_case": {
			"action_mocks": "What each action answers instead of really being called, as an object keyed by " +
				"action id; each value has status (HTTP status, 100 to 599) and body (any JSON), such as " +
				"{\"aac_abc123\": {\"status\": 200, \"body\": {\"order\": {\"status\": \"shipped\"}}}}. An " +
				"action with no mock answers with the result of its last live test.",
			"persona": "The made-up customer, an object with optional name, email, external_id, verified (true " +
				"when signed in), channel (widget, email, slack, whatsapp, instagram, facebook or api), " +
				"inbox_id, attributes and company (objects), such as {\"email\": \"ada@example.com\", " +
				"\"verified\": true, \"attributes\": {\"plan\": \"pro\"}}.",
			"simulated_at": "The date and time the test pretends it is when it runs, as an ISO 8601 time, such " +
				"as \"2026-12-24T18:00:00Z\". Left out, the test runs at the real current time.",
		},
		"publish_ai_action": {
			"acknowledge_warnings": "A list of text values that is accepted but not used: publishing an action " +
				"reads only its note. Leave it out.",
		},
		"publish_procedure": {
			"change_note": "An optional short note on what changed, kept with the version this publish " +
				"creates. At most 500 characters, such as \"Added the refund limit check before the refund " +
				"step\".",
		},
		"run_procedure_test_case": {
			"stream_id": "An id you make up, at most 64 characters, so the run reports each turn live on your " +
				"own realtime channel while it happens. Only useful to a client listening on that channel; " +
				"leave it out otherwise. Such as \"sim-7f3a2c\".",
		},
	}
}

/*
aiTextsPart4 returns texts of simulate_procedure, test_ai_action and the
tools after them.
*/
func aiTextsPart4() map[string]map[string]string {
	return map[string]map[string]string{
		"simulate_procedure": {
			"action_mocks": "What each action answers instead of really being called, as an object keyed by " +
				"action id; each value has status (HTTP status, 100 to 599) and body (any JSON), such as " +
				"{\"aac_abc123\": {\"status\": 200, \"body\": {\"order\": {\"status\": \"shipped\"}}}}. An " +
				"action with no mock answers with the result of its last live test.",
			"confirmed_action_ids": "Ids of the waiting writes you confirm or approve this turn, taken from " +
				"action_id in the pending list of the state the last simulated turn handed back (send that " +
				"state as active_state). At most 10, such as [\"aac_abc123\"]. You may send these without a " +
				"message.",
			"persona": "The made-up customer, an object with optional name, email, external_id, verified (true " +
				"when signed in), channel (widget, email, slack, whatsapp, instagram, facebook or api), " +
				"inbox_id, attributes and company (objects), such as {\"email\": \"ada@example.com\", " +
				"\"verified\": true, \"attributes\": {\"plan\": \"pro\"}}.",
			"stream_id": "An id you make up, at most 64 characters, so the run reports its progress live on " +
				"your own realtime channel while it happens. Only useful to a client listening on that " +
				"channel; leave it out otherwise. Such as \"sim-7f3a2c\".",
		},
		"test_ai_action": {
			"version": "An optional text of at most 16 characters that is accepted but not used: a test always " +
				"runs the action's working copy. Leave it out.",
		},
		"update_ai_action": {
			"body_encoding": "How the request body is sent: \"json\" or \"form\". Left out, it is \"json\". " +
				"With \"form\" the body_template must be a flat object of plain values, such as {\"email\": " +
				"\"{contact.email}\"}.",
			"body_template": "JSON body sent with the request, only for POST, PUT and PATCH. Values may hold " +
				"placeholders {input.x}, {contact.x} or {connection.x}, such as {\"order_id\": " +
				"\"{input.order_id}\", \"reason\": \"{input.reason}\"}. Send null to clear the body.",
			"bound_arguments": "For an action on an MCP connection: arguments you fix yourself, hidden from " +
				"the model and laid over what the model sends. An object of argument name to value; values may " +
				"hold placeholders, such as {\"account_id\": \"{connection.account_id}\", \"limit\": 10}.",
		},
	}
}

/*
aiTextsPart5 returns texts of update_ai_action,
update_ai_action_connection.
*/
func aiTextsPart5() map[string]map[string]string {
	return map[string]map[string]string{
		"update_ai_action": {
			"error_message_path": "Dotted path to the error message in the other system's JSON response, read " +
				"when a call fails. Letters, digits, _ , - and # joined by dots, at most 200 characters, such " +
				"as \"error.message\".",
			"headers": "Extra HTTP headers sent with the request, as an object of header name to value, at " +
				"most 10, such as {\"X-Api-Version\": \"2024-01\"}. Not allowed: Host, Cookie, Content-Length, " +
				"Content-Type, Transfer-Encoding, Authorization, Idempotency-Key, User-Agent, Connection and " +
				"any name starting with Dimmi-.",
			"method": "HTTP method of an action on an http connection: GET, POST, PUT, PATCH or DELETE " +
				"(lowercase is accepted). Left out on create, it is GET. Not used for an action on an mcp " +
				"connection. Such as \"POST\".",
			"response_fields": "Which values of the response the model reads, at most 30; left empty, it reads " +
				"the whole response. A list of {name, path, description}: name is lowercase letters, digits " +
				"and _, path is a dotted path into the JSON, such as [{\"name\": \"status\", \"path\": " +
				"\"order.status\", \"description\": \"Order status\"}].",
			"status": "Switches an action that was published before off or back on: \"disabled\" or " +
				"\"published\". Any other value is refused, and \"published\" is refused for an action never " +
				"published; use dimmi_publish_ai_action for a draft. Such as \"disabled\".",
		},
		"update_ai_action_connection": {
			"auth_config": "Non-secret auth settings as an object with header_name, query_param and prefix, " +
				"each optional. auth_type api_key_header needs header_name, such as {\"header_name\": " +
				"\"X-Api-Key\"}; api_key_query needs query_param, such as {\"query_param\": \"api_key\"}; " +
				"bearer may set prefix to replace \"Bearer\".",
			"auth_type": "How the connection signs its requests: \"none\", \"bearer\", \"api_key_header\", " +
				"\"api_key_query\" or \"basic\". api_key_header also needs auth_config.header_name and " +
				"api_key_query needs auth_config.query_param; \"none\" clears the stored secret. Such as " +
				"\"bearer\".",
			"default_headers": "Headers sent with every request through this connection, as an object of " +
				"header name to value, at most 10, values at most 256 characters, such as {\"X-Api-Version\": " +
				"\"2024-01\"}. Not allowed: Authorization, Content-Type, Cookie, Host, User-Agent and names " +
				"starting with Dimmi-.",
			"kind": "The connection's kind, \"http\" or \"mcp\". It is set when the connection is created and " +
				"an update does not change it, so leave it out.",
		},
	}
}

/*
aiTextsPart6 returns texts of update_ai_action_connection,
update_ai_agent and the tools after them.
*/
func aiTextsPart6() map[string]map[string]string {
	return map[string]map[string]string{
		"update_ai_action_connection": {
			"variables": "Named values the connection's actions can use in their templates as " +
				"{connection.name}, as an object of name to text. At most 20; names are lowercase letters, " +
				"digits and _, up to 40 characters; values up to 256 characters. Such as {\"account_id\": " +
				"\"acct_42\", \"region\": \"eu\"}.",
		},
		"update_ai_agent": {
			"fallback_model": "The name of the backup model, saved alongside fallback_key_id (the backup " +
				"provider connection). At most 120 characters; send null or empty to clear it. Use a model " +
				"name that connection offers, such as \"gpt-4o-mini\".",
			"pii_redaction": "true to mask personal data in text before it is sent to the model provider " +
				"(emails, phone numbers, card and bank account numbers, US social security numbers, API keys); " +
				"false to send text as it is. Such as true.",
			"product_name": "The name of your product, which the AI agent uses for it in its answers. At most " +
				"80 characters; when empty, the workspace name is used. Such as \"Acme Billing\".",
			"raise_priority_on_handoff": "true to make a conversation high priority whenever the AI agent " +
				"hands it over to the team; false to leave the priority as it is (a handover for a customer's " +
				"safety is still raised). Such as true.",
			"require_citations": "true or false for the \"Cite sources\" setting: when on, each answer links " +
				"the articles it came from. Such as true.",
			"streaming": "true or false; saved as the AI agent's streaming setting. Such as true.",
		},
		"update_ai_key": {
			"config": "Provider settings, as an object that replaces the saved one. Keys dimmi reads: " +
				"anthropic_version, max_tokens_field (text); supports_tools, supports_stream_usage, " +
				"json_schema, prompt_cache (true or false); context_tokens, max_output_tokens (numbers). Such " +
				"as {\"supports_tools\": false, \"context_tokens\": 32000}.",
		},
		"update_operator_routine": {
			"cadence": "How often the routine runs: \"daily\", \"weekly\", \"monthly\", \"cron\" or \"once\". " +
				"\"once\" also needs run_at; the exact time otherwise comes from cron_expression. A routine " +
				"runs at most once an hour. Such as \"weekly\".",
		},
	}
}

/*
aiTextsPart7 returns texts of update_operator_routine,
update_procedure_test_case.
*/
func aiTextsPart7() map[string]map[string]string {
	return map[string]map[string]string{
		"update_operator_routine": {
			"delivery": "Who is told about the routine's runs, as an object of id lists: in_app_member_ids and " +
				"email_member_ids (active teammates, at most 100 each) and slack_destination_ids (Slack " +
				"notification destinations, at most 20), such as {\"in_app_member_ids\": [\"mem_abc123\"], " +
				"\"slack_destination_ids\": [\"ndst_abc123\"]}.",
			"notify_policy": "When the routine tells people about a run: \"on_findings\" (only when it has " +
				"suggestions), \"always\" (after every run that was not skipped) or \"on_error_only\" (only " +
				"when a run fails). A pause is always told. Such as \"on_findings\".",
			"reset_state": "Send true to forget what the routine kept between its runs, so the next run starts " +
				"fresh. Left out or false, the kept state stays. Such as true.",
			"run_at": "The date and time a one-off routine runs, used only when cadence is \"once\". An ISO " +
				"8601 time still to come, such as \"2026-12-01T09:00:00Z\".",
			"scope": "What the routine works on beyond its inboxes, as an object: help_center_ids (at most 20 " +
				"help center ids) and github with installation_id, repositories (1 to 20, as owner/repo), " +
				"base_branch and ignore_labels, such as {\"help_center_ids\": [\"hc_abc123\"], \"github\": " +
				"{\"installation_id\": \"itg_abc123\", \"repositories\": [\"acme/app\"]}}.",
			"status": "Pauses or resumes the routine: \"paused\" or \"active\". Resuming forgets past failures " +
				"and works out the next run from now; a one-off routine with no run left also needs a new " +
				"run_at. Such as \"paused\".",
		},
		"update_procedure_test_case": {
			"action_mocks": "What each action answers instead of really being called, as an object keyed by " +
				"action id; each value has status (HTTP status, 100 to 599) and body (any JSON), such as " +
				"{\"aac_abc123\": {\"status\": 200, \"body\": {\"order\": {\"status\": \"shipped\"}}}}. An " +
				"action with no mock answers with the result of its last live test.",
			"assertions": "Checks after the test, at most 30 objects with a type: procedure_matched, " +
				"handed_off, action_called or action_not_called (with action), action_order (action, before), " +
				"branch_taken (step_id, branch then or else), reply_contains or reply_not_contains (text), " +
				"llm_rubric (rubric). Such as [{\"type\": \"action_called\", \"action\": \"aac_abc123\"}].",
		},
	}
}

/*
aiTextsPart8 returns texts of update_procedure_test_case.
*/
func aiTextsPart8() map[string]map[string]string {
	return map[string]map[string]string{
		"update_procedure_test_case": {
			"messages": "The customer's scripted messages in order, 1 to 12, each an object with text (1 to " +
				"4,000 characters) and optional confirm (true first confirms the writes waiting from the turn " +
				"before; text may then be empty). Such as [{\"text\": \"I want a refund for order 1042\"}, " +
				"{\"text\": \"\", \"confirm\": true}].",
			"persona": "The made-up customer, an object with optional name, email, external_id, verified (true " +
				"when signed in), channel (widget, email, slack, whatsapp, instagram, facebook or api), " +
				"inbox_id, attributes and company (objects), such as {\"email\": \"ada@example.com\", " +
				"\"verified\": true, \"attributes\": {\"plan\": \"pro\"}}.",
			"simulated_at": "The date and time the test pretends it is when it runs, as an ISO 8601 time, such " +
				"as \"2026-12-24T18:00:00Z\". Left out, the test runs at the real current time.",
		},
	}
}
