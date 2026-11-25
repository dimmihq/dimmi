/*
|--------------------------------------------------------------------------
| Written Schema Texts: AIAgent To AllowedAddress
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromAIAgentAndAIAgentView returns the texts from AIAgent on.
*/
func schemaTextsFromAIAgentAndAIAgentView() map[string]map[string]string {
	return map[string]map[string]string{
		"AIAgent": {
			"require_citations": "Whether the agent's answers must cite their sources.",
			"schedule": "When the agent may answer: always, outside_office_hours or " +
				"inside_office_hours.",
			"streaming": "Whether answers are streamed to the customer as they are written.",
			"tone":      "How the agent sounds: friendly, professional or concise.",
			"unknown_behavior": "What the agent does when it cannot answer: offer_handoff, handoff or " +
				"say_unknown.",
		},
		"AIAgentView": {
			"catalog": "The models that can be picked, as lists keyed by provider such as dimmi or " +
				"chatgpt. Each entry has the model id, display name and prices.",
			"keys":           "The workspace's provider connections, without their secrets.",
			"managed":        "True when the workspace has dimmi's built-in provider connection.",
			"prompt_version": "The version of the prompts the agent's answers use.",
			"providers": "The model providers the workspace could connect, each with its id, name and " +
				"how it signs in.",
			"readiness_errors": "Why the agent cannot go live, in plain English: \"answer model not " +
				"configured\" or \"no channels enabled\". Empty when ready.",
			"ready": "True when the agent could go live: it has a usable answer model and at least one " +
				"channel.",
		},
		"AIChatGPTConnection": {
			"account_id": "The ChatGPT plan account that signed in. Left out while the sign-in is " +
				"pending.",
			"interval_seconds": "How many seconds to wait before asking again while the sign-in is " +
				"pending. Left out otherwise.",
			"key_id": "Id of the new provider connection, such as aik_... Present only once status is " +
				"connected.",
			"status": "Where the ChatGPT sign-in stands: pending or connected.",
		},
		"AIProviderKey": {
			"auth_type": "How the connection signs in to the provider: api_key, aws_iam, " +
				"bedrock_api_key, none, managed or oauth.",
		},
	}
}

/*
schemaTextsFromAIProviderKey returns the texts from AIProviderKey on.
*/
func schemaTextsFromAIProviderKey() map[string]map[string]string {
	return map[string]map[string]string{
		"AIProviderKey": {
			"base_url": "The provider's address for this connection, for providers that need one. Null " +
				"when the provider's default is used.",
			"key_hint": "Dots and the last four characters of the stored key, such as ••••a1b2, to " +
				"tell keys apart. Null when no key is stored.",
			"last_used_at": "When the connection was last used for a model call. Null when never.",
			"last_validated_at": "When the connection was last checked against the provider. Null when " +
				"never.",
			"models": "Ids of the models the provider offered when the connection last validated. " +
				"Empty when it listed none.",
			"models_count": "How many models are in the models list.",
			"name":         "The name the team gave this provider connection.",
			"provider": "Which model provider this connects to, such as dimmi (built in), anthropic, " +
				"openai, chatgpt or openai_compatible.",
			"region": "The provider region this connection uses, for providers that have regions. Null " +
				"otherwise.",
			"source": "Where the connection came from: dashboard, api, env or system.",
			"status": "Whether the connection works: unverified (never checked), valid, invalid or " +
				"error.",
			"status_reason": "A short safe explanation of the status, such as \"never validated\" or " +
				"\"connected\". Null when there is none.",
		},
		"APILimits": {
			"token_per_minute": "Requests allowed per minute for one access token. 0 means no limit.",
			"user_per_minute": "Requests allowed per minute for one signed-in person. 0 means no " +
				"limit.",
			"workspace_per_minute": "Requests allowed per minute for all of the workspace's tokens " +
				"together. 0 means no limit.",
		},
		"AccessPolicy": {
			"allowed_sign_in_methods": "The ways of signing in the workspace accepts, such as " +
				"password, magic_link, google or sso. Empty means every way is accepted.",
			"api_ip_allowlist": "Address ranges access tokens may call the API from. Empty means from " +
				"anywhere.",
			"available_sign_in_methods": "The ways of signing in this install offers, such as " +
				"password, magic_link, google or sso. allowed_sign_in_methods must be picked from these.",
		},
	}
}

/*
schemaTextsFromAccessPolicy returns the texts from AccessPolicy on.
*/
func schemaTextsFromAccessPolicy() map[string]map[string]string {
	return map[string]map[string]string{
		"AccessPolicy": {
			"dashboard_ip_allowlist": "Address ranges browsers and apps may use the dashboard from. " +
				"Empty means from anywhere.",
			"google_allowed_domains": "Google hosted domains a Google sign-in may come from, such as " +
				"example.com. Empty means any.",
			"require_two_factor": "Whether members must use two-factor authentication to use the " +
				"workspace.",
			"session_idle_timeout_minutes": "Minutes a session may sit unused before it ends, from 15 " +
				"to 20160. Null means no limit.",
			"session_max_age_minutes": "Minutes a session may last in total before signing in again, " +
				"from 60 to 43200. Null means no limit.",
			"sso_enforced": "True when the organization enforces a single sign-on connection for this " +
				"workspace.",
			"tokens_outside": "How many live tokens were last used from an address outside " +
				"api_ip_allowlist, so the list would stop them. 0 when the list is empty.",
			"your_ip": "The IP address the caller is seen at, such as 203.0.113.7. Empty when it is " +
				"not known.",
		},
		"ActionMockInput": {
			"body": "The JSON body the mocked action answers with.",
			"status": "The HTTP status the mocked action answers with, from 100 to 599, such as 200 or " +
				"404.",
		},
		"AiPlaygroundAnswer": {
			"outcome": "How the test run ended: answered, unknown or handoff.",
			"retrieved": "The passages the agent found for the question, numbered the way the answer " +
				"cites them. Empty when it found none.",
			"run_id": "Id of the recorded playground run, such as air_... Null when no run was kept.",
		},
		"AiPlaygroundCitation": {
			"n":     "The citation's number, the same number as the retrieved passage it points to.",
			"title": "Title of the cited source.",
			"url":   "Link to the cited source.",
		},
	}
}

/*
schemaTextsFromAiPlaygroundClass returns the texts from AiPlaygroundClass on.
*/
func schemaTextsFromAiPlaygroundClass() map[string]map[string]string {
	//nolint:gosec // words about secrets, never a secret
	return map[string]map[string]string{
		"AiPlaygroundClass": {
			"intent": "What the agent took the question to be asking for.",
			"language": "The language the agent detected in the question, as a language tag such as " +
				"en.",
			"query": "The standalone search query the agent rewrote the question into and searched " +
				"with.",
		},
		"AiPlaygroundReply": {
			"body":      "The reply the agent would send. Empty when it would send none.",
			"citations": "The sources the reply cites. Empty when it cites none.",
		},
		"AiPlaygroundSource": {
			"n": "The passage's number, the one the answer uses to cite it.",
			"similarity": "How closely the passage matched the question, as a number where higher is " +
				"closer.",
			"text":  "The passage's text.",
			"title": "Title of the source the passage comes from.",
			"url":   "Link to the source the passage comes from.",
			"used":  "True when the answer used this passage.",
		},
		"AiPlaygroundTiming": {
			"first_token_ms": "Milliseconds until the first piece of the answer arrived.",
			"latency_ms":     "Milliseconds the whole answer took.",
		},
		"AiPlaygroundUsage": {
			"cached_input_tokens": "How many of the input tokens were read from the provider's cache.",
			"cost_micros":         "What the question cost, in millionths of a US dollar (1000000 is $1).",
			"input_tokens":        "Tokens sent to the models for this question.",
			"output_tokens":       "Tokens the models wrote for this question.",
		},
		"AiRun": {
			"answer_message_id": "Id of the message the agent sent as its answer, such as msg_... Null " +
				"when the run sent none.",
			"completed_at": "When the run finished. Null while it is still running.",
		},
	}
}

/*
schemaTextsFromAiRun returns the texts from AiRun on.
*/
func schemaTextsFromAiRun() map[string]map[string]string {
	//nolint:gosec // words about secrets, never a secret
	return map[string]map[string]string{
		"AiRun": {
			"conversation_id": "Id of the conversation the run worked on, such as conv_... Null for a " +
				"run outside a conversation, such as a playground question.",
			"cost_micros": "What the run cost in model calls, in millionths of a US dollar (1000000 is " +
				"$1).",
			"error": "Why the run failed, as an object with code and message. Null when it did not " +
				"fail.",
			"feedback": "How the answer was rated: helped, not_helped, solved, handoff, discarded, " +
				"good or bad. Null when nobody rated it.",
			"first_token_ms": "Milliseconds until the first piece of the answer arrived. Null when not " +
				"measured.",
			"flags": "Things that stepped in during the run, such as link_removed, reply_regenerated, " +
				"reply_withheld, implied_handoff or enforced_handoff. Empty when nothing did.",
			"handoff_reason": "Why the agent handed the conversation to the team, such as " +
				"customer_requested, unknown, max_replies, frustration, spend_cap or provider_error. Null when " +
				"it did not hand off.",
			"input_tokens": "Tokens sent to the models during the run.",
			"kind": "What the run was for: conversation, playground, probe, help_center, task, " +
				"classify, judge, ticket_fill, ticket_match, ticket_summary or simulation.",
			"language": "The language detected for the run, as a language tag such as en. Null when " +
				"none was detected.",
			"latency_ms": "Milliseconds the whole run took. Null when not measured.",
			"outcome": "How the run ended: answered, unknown, handoff, silent, greeted, refused, " +
				"skipped, superseded, drafted or error. Null while it runs.",
			"output_tokens": "Tokens the models wrote during the run.",
			"payloads_visible": "True when the caller may read the steps' request and response " +
				"payloads. Present only when one run is read with its steps.",
			"procedure_trace": "What the run did with the procedure it followed: the path taken, the " +
				"branches decided and where a guardrail stepped in. Left out when it followed none.",
			"prompt_version": "The version of the prompts the run used. Empty when not recorded.",
			"skip_reason": "Why the run was skipped without answering, such as human_replied_in_delay " +
				"or agent_missing. Null when it was not skipped.",
			"status": "Where the run stands: running, completed, failed or skipped.",
		},
	}
}

/*
schemaTextsFromAiRunAndAiRunStep returns the texts from AiRun on.
*/
func schemaTextsFromAiRunAndAiRunStep() map[string]map[string]string {
	//nolint:gosec // words about secrets, never a secret
	return map[string]map[string]string{
		"AiRun": {
			"steps": "The run's traced steps in order. Present only when one run is read, not in " +
				"lists.",
		},
		"AiRunStep": {
			"cost_micros": "What this step cost, in millionths of a US dollar (1000000 is $1).",
			"error_kind": "The kind of failure when the step failed, such as auth, quota, " +
				"rate_limited, overloaded, timeout or network. Null when it did not fail.",
			"first_token_ms": "Milliseconds until the model's first output arrived in this step. Null " +
				"when not measured.",
			"input_tokens": "Tokens sent to the model in this step.",
			"kind": "What the step did: classify, retrieve, generate, tool_call, verify, summarize, " +
				"translate or probe.",
			"latency_ms":    "Milliseconds the step took. Null when not measured.",
			"model":         "Id of the model the step called. Null for a step that called none.",
			"output_tokens": "Tokens the model wrote in this step.",
			"provider": "The model provider the step called, such as dimmi, anthropic or openai. Null " +
				"for a step that called none.",
			"request": "The redacted record of what the step sent. Null when the caller may not read " +
				"payloads or the record's retention has passed.",
			"response": "The redacted record of what came back. Null when the caller may not read " +
				"payloads or the record's retention has passed.",
			"seq":    "The step's position within its run; steps run in rising order.",
			"status": "How the step ended: ok, error or canceled.",
			"upstream": "The provider the model's calls actually reached, when that differs from the " +
				"connection's name. Null when not recorded.",
		},
		"AiUsage": {
			"answered_count": "Runs this month that ended with an answer.",
			"by_model": "The month's spend and call count for each model. Empty when there were no " +
				"calls.",
			"by_outcome": "How many of the month's runs ended each way. Empty when there were no runs.",
			"cached_input_tokens": "How many of the month's input tokens were read from the provider's " +
				"cache.",
			"capped_at": "When the month's spending cap was reached. Null when it has not been.",
		},
	}
}

/*
schemaTextsFromAiUsage returns the texts from AiUsage on.
*/
func schemaTextsFromAiUsage() map[string]map[string]string {
	//nolint:gosec // words about secrets, never a secret
	return map[string]map[string]string{
		"AiUsage": {
			"cost_micros": "What the agent cost this month, in millionths of a US dollar (1000000 is " +
				"$1).",
			"daily":         "Cost and runs for each day of the month, days in UTC.",
			"handoff_count": "Runs this month that ended with a handoff to the team.",
			"input_tokens":  "Tokens sent to the models this month.",
			"monthly_spend_cap_micros": "The agent's monthly spending cap, in millionths of a US " +
				"dollar. Null when there is no cap.",
			"output_tokens": "Tokens the models wrote this month.",
			"period_start": "First day of the month the numbers cover, as YYYY-MM-DD, such as " +
				"2026-11-01.",
			"runs_count":    "How many times the agent ran this month.",
			"silent_count":  "Runs this month where the agent chose to say nothing.",
			"unknown_count": "Runs this month where the agent did not know the answer.",
		},
		"AiUsageDay": {
			"cost_micros": "What the agent cost that day, in millionths of a US dollar.",
			"day":         "The day in UTC, as YYYY-MM-DD, such as 2026-11-23.",
			"runs_count":  "How many times the agent ran that day.",
		},
		"AiUsageModel": {
			"calls":       "How many calls were made to this model in the month.",
			"cost_micros": "What this model cost in the month, in millionths of a US dollar.",
			"model":       "Id of the model.",
		},
		"AiUsageOutcome": {
			"outcome":    "How the runs ended, such as answered, unknown, handoff, silent or skipped.",
			"runs_count": "How many of the month's runs ended this way.",
		},
		"AllowedAddress": {
			"cidr":  "An address range in CIDR form, or a single IP address, such as 203.0.113.0/24.",
			"label": "What the workspace calls this range, such as Office.",
		},
	}
}
