/*
|--------------------------------------------------------------------------
| Written Schema Texts: OrganizationDomain To PushSubscription
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromOrganizationDomain returns the texts from OrganizationDomain on.
*/
func schemaTextsFromOrganizationDomain() map[string]map[string]string {
	return map[string]map[string]string{
		"OrganizationDomain": {
			"domain": "The domain the company is proving it owns, such as example.com.",
			"last_checked_at": "When dimmi last looked for the proving record. Null when it has not " +
				"yet.",
			"record_name":  "Name of the DNS TXT record to create to prove ownership.",
			"record_value": "Value the DNS TXT record must hold to prove ownership.",
			"sso_connection_id": "Id of the single sign-on connection the domain is tied to (starts " +
				"with ssoc_). Null when it is tied to none.",
			"status":      "Where the proof stands: pending, verified or failed.",
			"verified_at": "When the domain was proven. Null until then.",
			"verified_via": "How it was proven: dns (the TXT record was found) or cli (an operator of " +
				"the server did it). Null until verified.",
		},
		"PasswordChange": {
			"tokens_remaining": "How many of the person's access tokens still work after the password " +
				"change, since changing it does not end them.",
		},
		"PastConversations": {
			"data": "The customer's newest other conversations, besides the one being viewed.",
			"total_estimate": "How many other conversations the customer has, counted only up to 100, " +
				"so 100 means 100 or more.",
		},
		"Permission": {
			"assignable": "True when a custom role may hold this permission.",
			"base_roles": "The built-in roles (owner, admin, member) a custom role may be based on to " +
				"hold this permission.",
			"description": "A sentence saying what the permission lets someone do.",
			"group":       "The ability group access tokens reach this permission through.",
			"key": "The permission's key, such as ai_runs:read_payloads; use it in a custom role's " +
				"permissions.",
			"label": "Short name of the permission.",
		},
		"PersonalAccessToken": {
			"abilities":    "What the token may do, as ability names. Empty when it has none.",
			"expires_at":   "When the token stops working, in UTC. Null when it never expires.",
			"last_used_at": "When the token was last used, in UTC. Null when it never was.",
		},
	}
}

/*
schemaTextsFromPersonalAccessToken returns the texts from
PersonalAccessToken on.
*/
func schemaTextsFromPersonalAccessToken() map[string]map[string]string {
	return map[string]map[string]string{
		"PersonalAccessToken": {
			"last_used_ip": "IP address the token was last used from. Null when it never was.",
			"name":         "Name its owner gave the token.",
			"plain_text_token": "The token's secret itself. Present only in the answer that created it " +
				"and never again, so store it then.",
			"prefix":     "The start of the token, shown so its owner can recognize it without the secret.",
			"revoked_at": "When the token was revoked, in UTC. Null while it is still valid.",
		},
		"PreviousSlug": {
			"slug": "A slug the workspace used before and gave up; it is still held for this " +
				"workspace.",
			"until": "Until when the old slug is held for this workspace.",
		},
		"Problem": {
			"code": "Code of what the series is missing before it can go live, such as loop, " +
				"entry_missing or step_unreachable.",
			"field": "The field of the step the problem is about. Left out when it is not about one " +
				"field.",
			"message": "A sentence explaining the problem, to show beside the step.",
			"node_id": "Id of the step the problem is about. Left out when it concerns the whole " +
				"series.",
		},
		"Procedure": {
			"definition": "The procedure's working copy as a JSON object: the draft being edited, " +
				"which may differ from what dimmi follows.",
			"has_unpublished_changes": "True when the procedure is live and its working copy differs " +
				"from the published one dimmi follows.",
			"lock_version": "Version counter of the working copy; send the value you read when saving " +
				"so a newer save is not overwritten.",
			"name":         "Name of the procedure.",
			"published_at": "When it was last published. Null when it never was.",
			"status": "draft when it was never published, live when dimmi follows its published " +
				"version.",
			"uses": "How often dimmi followed the procedure in the last 30 days.",
		},
	}
}

/*
schemaTextsFromProcedureActionFigure returns the texts from
ProcedureActionFigure on.
*/
func schemaTextsFromProcedureActionFigure() map[string]map[string]string {
	return map[string]map[string]string{
		"ProcedureActionFigure": {
			"action_id": "Id of the workspace action (starts with aac_).",
			"calls":     "How many times the action was called in runs that followed the procedure.",
			"error_rate": "Share of those calls that failed, from 0 to 1. Null when there were no " +
				"calls.",
			"failed":          "How many of those calls failed.",
			"mean_latency_ms": "How long a call took on average, in milliseconds.",
			"name":            "Name of the action.",
		},
		"ProcedureCSAT": {
			"mean": "Average customer rating of the rated conversations, on the 1 to 5 scale. Null " +
				"when none were rated.",
			"positive": "How many conversations were rated 4 or 5.",
			"positive_rate": "Share of rated conversations rated 4 or 5, from 0 to 1. Null when none " +
				"were rated.",
			"rated": "How many of the conversations the customer rated.",
		},
		"ProcedureHandoverReason": {
			"conversations": "How many conversations were handed to the team for this reason.",
			"kind": "What sort of reason it is: intentional (the procedure asked), customer (the " +
				"customer asked), guardrail (one stepped in) or failure (something broke).",
			"reason": "The handover reason as recorded, such as customer_requested, procedure_guard or " +
				"provider_error; other when none was given.",
		},
		"ProcedureMetrics": {
			"actions": "How each action fared in the procedure's runs: calls, failures and average " +
				"time.",
			"ai_assumed": "Conversations counted as resolved by the AI without the customer confirming " +
				"it: ai_resolved minus ai_confirmed.",
			"ai_confirmed": "Conversations the AI resolved where the customer confirmed it was solved.",
			"ai_involved":  "How many of the matched conversations the AI agent took part in.",
			"ai_resolution_rate": "ai_resolved divided by ai_involved, from 0 to 1. Null when the AI " +
				"was in none.",
			"ai_resolved": "Conversations the AI resolved with no teammate reply and no handover: " +
				"confirmed, closed, or left quiet after its last answer.",
		},
	}
}

/*
schemaTextsFromProcedureMetrics returns the texts from ProcedureMetrics on.
*/
func schemaTextsFromProcedureMetrics() map[string]map[string]string {
	return map[string]map[string]string{
		"ProcedureMetrics": {
			"days": "How many days back the numbers cover.",
			"guardrails": "How often each kind of guardrail stepped in, as an object of guardrail kind " +
				"to count. Empty when none did.",
			"handed_off": "How many conversations the AI handed to the team.",
			"handover_rate": "handed_off divided by ai_involved, from 0 to 1. Null when the AI was in " +
				"none.",
			"handover_reasons":      "The handovers counted by why, most common first.",
			"matched_conversations": "How many conversations the procedure ran in over the range.",
			"procedure_id":          "Id of the procedure these numbers are for (starts with prc_).",
			"runs":                  "The runs behind the numbers, newest first, for drilling into what went wrong.",
			"steps":                 "How far conversations got through each step of the procedure.",
			"version": "The published version number these numbers are narrowed to. Null when every " +
				"version is counted.",
		},
		"ProcedureReportRow": {
			"ai_involved": "How many of the matched conversations the AI agent took part in.",
			"ai_resolution_rate": "ai_resolved divided by ai_involved, from 0 to 1. Null when the AI " +
				"was in none.",
			"ai_resolved": "How many of those conversations the AI resolved with no teammate reply and " +
				"no handover.",
			"guard_events": "How many times the procedure's guardrails stepped in over the range.",
			"handed_off":   "How many conversations the AI handed to the team.",
			"handover_rate": "handed_off divided by ai_involved, from 0 to 1. Null when the AI was in " +
				"none.",
			"matched_conversations": "How many conversations the procedure ran in over the range.",
			"mean_rating": "Average customer rating of the rated conversations, on the 1 to 5 scale. " +
				"Null when none were rated.",
			"name": "Name of the procedure.",
			"procedure_id": "Id of the procedure (starts with prc_); send it wherever a procedure is " +
				"asked for.",
			"rated": "How many of the conversations the customer rated.",
		},
	}
}

/*
schemaTextsFromProcedureRunFigure returns the texts from ProcedureRunFigure on.
*/
func schemaTextsFromProcedureRunFigure() map[string]map[string]string {
	return map[string]map[string]string{
		"ProcedureRunFigure": {
			"conversation_id": "Id of the conversation the run was in (starts with conv_). Null when " +
				"it has none.",
			"conversation_number": "Number of the conversation the run was in.",
			"handoff_reason": "Why the run handed the conversation to the team, such as " +
				"customer_requested or provider_error. Null when it did not.",
			"outcome": "How the run ended, such as answered, handoff, unknown, silent, refused, " +
				"skipped or error. Null when not recorded.",
			"procedure_trace": "What the procedure did in this run, as a JSON object with its version, " +
				"the steps reached and any guardrail events. Left out when there is none.",
			"rating": "The customer's rating of the conversation, from 1 (worst) to 5 (best). Null " +
				"when not rated.",
			"reopens": "How many times the conversation was reopened.",
			"run_id":  "Id of the AI run (starts with air_); read that run for its full steps.",
		},
		"ProcedureSimulation": {
			"action_calls": "The actions the simulated turn called. They are mocked and never really " +
				"run. Empty when it called none.",
			"matched": "True when the procedure under test was the one followed in this turn.",
			"outcome": "How the simulated turn ended, such as answered, handoff, unknown, silent or " +
				"refused.",
			"procedure_trace": "What the procedure did in this turn, as a JSON object with the steps " +
				"reached and any guardrail events. Left out when there is none.",
			"reply": "The reply the AI agent would have sent the customer. Empty when it would send " +
				"none.",
			"run_id": "Id of the AI run behind the simulated turn (starts with air_).",
		},
		"ProcedureStepFigure": {
			"completion_rate": "done divided by reached, from 0 to 1. Null when no conversation " +
				"reached the step.",
			"done":      "How many conversations finished the step.",
			"reached":   "How many conversations reached the step.",
			"step_id":   "Id of the step inside the procedure's definition.",
			"took_else": "How many conversations took the else side of this step's condition.",
			"took_then": "How many conversations took the then side of this step's condition.",
		},
	}
}

/*
schemaTextsFromProcedureTestCase returns the texts from ProcedureTestCase on.
*/
func schemaTextsFromProcedureTestCase() map[string]map[string]string {
	return map[string]map[string]string{
		"ProcedureTestCase": {
			"action_mocks": "What each mocked action answers, as a JSON object keyed by action, each " +
				"with an HTTP status and a body.",
			"assertions": "What must hold after the simulated conversation, as a JSON array of checks, " +
				"such as a procedure match, an action call, a branch taken, a handover or reply text.",
			"last_result": "How its last run went, as a JSON object with passed, ran_at, turns, " +
				"assertion_results and, on a broken run, error. Null when it never ran.",
			"messages": "The scripted customer messages of the test, in order, as a JSON array; each " +
				"has text and an optional confirm.",
			"name": "Name of the test, up to 120 characters.",
			"persona": "The made-up customer of the test, as a JSON object with fields such as name, " +
				"email, verified, channel, attributes and company.",
			"procedure_id": "Id of the procedure the test belongs to (starts with prc_).",
			"simulated_at": "The moment the test pretends it is, so date rules such as a refund window " +
				"give the same result every run. Null to run at the real time.",
			"source_conversation_id": "Id of the real conversation the test was made from (starts with " +
				"conv_). Null when it was written by hand.",
		},
		"ProcedureVersion": {
			"change_note": "The publisher's note on what changed in this version. Null when none was " +
				"given.",
			"conversations": "How many conversations this version ran in over the last 90 days.",
			"definition": "The procedure's definition exactly as published in this version, as a JSON " +
				"object.",
			"handed_off": "How many of those conversations were handed to the team over the last 90 " +
				"days.",
			"name":         "The procedure's name as it was when this version was published.",
			"procedure_id": "Id of the procedure this is a version of (starts with prc_).",
			"published_at": "When this version was published.",
			"published_by_member_id": "Id of the teammate who published it (starts with mem_). Null " +
				"when they are gone.",
			"version": "The version's number within its procedure, counting up with each publish.",
		},
	}
}

/*
schemaTextsFromProvider returns the texts from Provider on.
*/
func schemaTextsFromProvider() map[string]map[string]string {
	return map[string]map[string]string{
		"Provider": {
			"key": "The sign-in provider's key, such as google; it is the name used in its sign-in " +
				"address.",
			"name":     "The provider's name to show on the sign-in button.",
			"protocol": "How it signs people in: oauth2 or oidc.",
			"redirect_url": "Path on this server where sign-in with the provider starts, such as " +
				"/auth/google/redirect.",
		},
		"ProviderDescriptor": {
			"auth_type": "How the AI provider is connected: managed (built in), api_key or oauth.",
			"available": "True when this server can actually serve the provider; false when it is not " +
				"set up here.",
			"docs_url":     "Where to get a key for the provider. Empty when there is no such page.",
			"key_required": "True when connecting the provider needs an API key.",
			"name":         "The provider's name for the picker, such as Anthropic or OpenAI.",
		},
		"PushConfig": {
			"key_id": "Id naming the server's push key now in use, the one new subscriptions are made " +
				"with.",
			"vapid_public_key": "The server's public VAPID key a browser subscribes to push with.",
		},
		"PushKeysBody": {
			"auth": "The subscription's auth secret as the browser gives it, base64url, at most 100 " +
				"characters.",
			"p256dh": "The subscription's P-256 public key as the browser gives it, base64url, at most " +
				"200 characters.",
		},
		"PushSubscription": {
			"device_label": "Name shown for the device, such as its browser and system. Null when " +
				"unknown.",
			"endpoint":        "The push service address the browser gave for this device.",
			"last_success_at": "When a push last reached this device. Null when none has yet.",
			"platform": "Which push service the device uses: apple, google, mozilla, microsoft or " +
				"other.",
		},
	}
}
