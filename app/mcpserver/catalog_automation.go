/*
|--------------------------------------------------------------------------
| Tool Catalog: Automation
|--------------------------------------------------------------------------
|
| The entries for the AI agent, its actions and keys, Operator, procedures and workflows.
|
*/

package mcpserver

import "maps"

/*
automationDocs returns the entries for the AI agent, its actions and
keys, Operator, procedures and workflows.
*/
func automationDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		agentDocs(), agentSettingsDocs(), keyDocs(), aiActionDocs(), operatorProcedureDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
agentDocs returns the entries for reading the AI agent, its usage and
its playground.
*/
func agentDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agent.show": {
			Title: "Get the AI agent",
			Use: "Reads the workspace's AI agent, the one that answers customers: whether it is on, its tone, " +
				"guidance, channels, schedule, models, spending cap and handoff rules. Read it before changing " +
				"the agent, to see what is set today.",
			Returns: "the agent's settings with the provider keys and models it answers with.",
			Next: []string{
				"dimmi_ai_agent_update: change how the agent answers",
				"dimmi_ai_usage_get: see what the agent costs this month",
			},
		},
		"ai.usage.show": {
			Title: "Get AI agent usage",
			Use: "Reads one month of the AI agent's work: what it spent, tokens, how many conversations it " +
				"answered, did not know or handed over, day by day and by model. Months are counted in UTC. " +
				"Operator's own spend is in dimmi_operator_usage_get.",
			Returns: "the month's cost in micro-dollars, counts by outcome, daily series, spend by model and the cap.",
			Params: map[string]string{
				"period": "The month as YYYY-MM, such as 2026-11. Leave out for the current month.",
			},
		},
		"ai.playground.store": {
			Title: "Test the AI agent",
			Use: "Asks the AI agent a test question and shows what it would answer, without writing any message " +
				"or changing a conversation. Each question runs the real model and spends real tokens from the " +
				"workspace's AI budget, so ask only what the person wants tested. Works while the agent is off.",
			Returns: "the answer with its outcome, how the question was read, the knowledge found, tokens, cost " +
				"and timing.",
			Next: []string{
				"dimmi_ai_agent_update: adjust guidance or tone if the answer was off",
				"dimmi_ai_usage_get: check the month's spend",
			},
			Params: map[string]string{
				"message": "The test question, as a customer would write it, up to 4000 characters.",
				"conversation": "Optional conversation (conv_...) whose history the question reads, to test a " +
					"follow-up. The conversation is not changed.",
			},
			Hint: "A member may ask at most 30 test questions a minute; wait and try again.",
		},
	}
}

/*
agentSettingsDocs returns the entry for changing the AI agent.
*/
func agentSettingsDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agent.update": {
			Title: "Change the AI agent",
			Use: "Changes how the AI agent answers customers: switching it on or off, its tone, guidance, " +
				"channels, schedule, models, spending cap and handoff. Customers get the change on their next " +
				"message. Only the fields sent change; null clears a field that may be empty.",
			Returns: "the agent's settings as they now are.",
			Next:    []string{"dimmi_ai_playground_create: try a question with the new settings"},
			Risk:    new(RiskReach),
			Params: map[string]string{
				"enabled": "true lets the agent answer customers. Switching on needs a usable answer model and " +
					"a channel.",
				"tone":     "friendly, professional or concise.",
				"guidance": "Instructions for the agent in plain words, up to 2000 characters.",
				"enabled_channels": "Where the agent answers, replacing the list: widget (the messenger), email " +
					"and api.",
				"schedule": "When it answers: always, outside_office_hours or inside_office_hours.",
				"answer_key_id": "Provider key (aik_...) the agent answers with. Find it with " +
					"dimmi_ai_keys_list.",
				"answer_model":    "Model id on that key, from the models the key lists.",
				"fallback_key_id": "Provider key (aik_...) used when the answer key fails; null clears it.",
				"monthly_cap_usd": "Most the agent may spend a month, in US dollars; 0 or null means no cap. " +
					"Raising it lifts a pause the old cap caused.",
				"max_ai_replies":   "Most replies the agent sends in one conversation, 1 to 50.",
				"unknown_behavior": "When it cannot answer: offer_handoff, handoff or say_unknown.",
				"assigned_behavior": "In a conversation a teammate holds: skip_when_human_active, " +
					"never_when_assigned or always.",
				"handoff_mode": "full answers from the knowledge; triage_only only sorts the conversation and " +
					"hands it to a person.",
				"handoff_team_id":         "Team (team_...) a handed-over conversation goes to; null clears it.",
				"include_contact_context": "none or basic: how much of the customer's record the agent reads.",
				"link_allowlist": "Hosts the agent may link to, such as example.com or *.example.com, " +
					"replacing the list.",
				"always_handoff_tag_ids": "Tags (tag_...) that always send a conversation to a person, " +
					"replacing the list.",
				"languages":        "Language tags the agent answers in, such as en or pt-BR, replacing the list.",
				"default_language": "Language tag used when the customer's language is unknown.",
			},
			Hint: "A field error names the value refused; read the agent with dimmi_ai_agent_get for what is " +
				"set now.",
		},
	}
}

/*
keyDocs returns the entries for the AI agent's provider keys.
*/
func keyDocs() map[string]Doc {
	return map[string]Doc{
		"ai.keys.index": {
			Title:   "List AI provider keys",
			Use:     "Lists the AI provider connections the agents answer with. Secrets never come back, only a hint.",
			Returns: "every key with its provider, status, the models it offers and when it was last checked.",
		},
		"ai.keys.store": {
			Title: "Add an AI provider key",
			Use: "Connects an AI provider with the workspace's own API key. dimmi checks the key with the " +
				"provider as it saves; a key the provider refuses is not kept. Only add a key the person gave " +
				"you for this.",
			Returns: "the key with its status and the models it offers, the secret hidden.",
			Next:    []string{"dimmi_ai_agent_update: point the agent at the new key with answer_key_id"},
			Params: map[string]string{
				"provider": "anthropic, openai, engy or openai_compatible. ChatGPT plans connect in the browser " +
					"instead.",
				"api_key":  "The provider's secret key. Required except for an openai_compatible server without one.",
				"base_url": "Endpoint of an openai_compatible server, such as https://llm.example.com/v1.",
			},
			Hint: "A workspace holds at most 20 keys; remove an unused one first.",
		},
		"ai.keys.update": {
			Title: "Change an AI provider key",
			Use: "Renames a provider key, moves its endpoint or rotates its secret. A new secret or endpoint " +
				"is checked again at once, and the agents switch to it on their next answer.",
			Returns: "the key with its fresh status.",
			Params:  map[string]string{"api_key": "A new secret to replace the stored one; leave out to keep it."},
		},
		"ai.keys.destroy": {
			Title: "Remove an AI provider key",
			Use: "Deletes a provider key and its secret. Refused while the agent still answers with it: point " +
				"the agent at another key with dimmi_ai_agent_update first.",
		},
		"ai.keys.test.store": {
			Title:   "Check an AI provider key",
			Use:     "Checks a provider key with the provider again and stores the fresh status.",
			Returns: "the key with its status, a reason when it fails, and the models it offers.",
			Risk:    new(RiskWrite),
		},
	}
}
