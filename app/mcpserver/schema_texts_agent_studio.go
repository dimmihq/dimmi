/*
|--------------------------------------------------------------------------
| Written Schema Texts: AI Agents
|--------------------------------------------------------------------------
|
| What each field of an AI agent's records means, by schema and field:
| the agent and its definition, its versions, a preview turn, its saved
| simulations and their runs, the templates, the places an agent can
| take conversations and the settings every agent shares.
|
*/

package mcpserver

/*
agentStudioSchemaTexts returns the texts of these records.
*/
func agentStudioSchemaTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range []map[string]map[string]string{
		agentSchemaTexts(), agentDefinitionSchemaTexts(), agentScopeSchemaTexts(), agentRunSchemaTexts(),
		agentSimulationSchemaTexts(), sharedAISchemaTexts(), mergedSchemaTexts(),
		workflowSimulationSchemaTexts(), seriesSimulationSchemaTexts(),
	} {
		mergeTexts(out, part)
	}

	return out
}

/*
agentSchemaTexts returns the texts of an agent, its versions and the
templates a new one starts from.
*/
func agentSchemaTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"AIAgent": {
			"description": "What the live agent handles. Other agents read it to know when to pass a " +
				"conversation to this one.",
			"has_unpublished_changes": "true when the draft differs from what customers meet, so a publish " +
				"would change something.",
			"is_default":   "true for the one agent that answers every conversation no other agent takes.",
			"live_version": "Number of the version customers meet now. 0 before the first publish.",
			"lock_version": "Counts the agent's saves. Send it back with an edit to refuse the edit when " +
				"someone else saved in between.",
			"published_at": "When the agent was last published. Null when it never was.",
			"role": "The kind of work the agent was made for, taken from its template, such as support, " +
				"billing or developer.",
			"status":   "Where the agent stands: draft (never published), live or paused.",
			"template": "Key of the template the agent started from, such as support. Null when it started empty.",
			"transfer_agent_ids": "Ids of the agents the live agent may pass a conversation to, each " +
				"starting with aia_.",
		},
		"AIAgentVersion": {
			"agent_id":     "Id of the agent this is a version of (starts with aia_).",
			"change_note":  "The publisher's note on what changed in this version. Null when none was given.",
			"published_at": "When this version was published.",
			"published_by_member_id": "Id of the teammate who published this version. Null when they are no " +
				"longer known.",
			"version": "The version's number, counted from 1 for each agent.",
		},
		"AIAgentTemplate": {
			"agent_name":  "The name the template suggests for the new agent, such as Billing.",
			"description": "What an agent made from the template does, in a sentence, for the picker.",
			"group":       "The group the picker lists the template under: support, growth, internal or blank.",
			"key": "The template's key, sent as template when creating an agent. Empty for starting from " +
				"nothing.",
			"name": "The template's name in the picker, such as Billing and retention.",
			"role": "The kind of work an agent made from the template does, such as support or billing.",
			"widget": "true when an agent of this kind usually answers on a page of its own, such as a docs " +
				"site, and not in the support messenger.",
		},
	}
}

/*
agentDefinitionSchemaTexts returns the texts of what a teammate edits
on an agent.
*/
func agentDefinitionSchemaTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"AIAgentDefinition": {
			"always_handoff_tag_ids": "Ids of the tags that always send a conversation to a person, each " +
				"starting with tag_.",
			"answer_key_id": "Id of the provider key the agent answers with (starts with aik_). Null when it " +
				"uses the key every agent shares.",
			"answer_model": "The model the agent answers with. Null when it uses the shared model.",
			"assigned_behavior": "What the agent does in a conversation a teammate holds: " +
				"skip_when_human_active, never_when_assigned or always.",
			"default_language": "Language tag the agent answers in when the customer's language is unknown, " +
				"such as en.",
			"description": "What the agent handles. Other agents read it to know when to pass a conversation " +
				"to this one.",
			"enabled_channels": "The kinds of channel the agent answers on, such as widget, email and api.",
			"fallback_key_id": "Id of the provider key used when the answer key fails (starts with aik_). " +
				"Null when there is none.",
			"fallback_model": "The model used when the answer model fails. Null when there is none.",
			"guidance":       "How the agent talks and what it does, in the team's own words.",
			"handoff_mode": "full when the agent answers from the knowledge; triage_only when it only sorts " +
				"the conversation and hands it to a person.",
			"handoff_team_id": "Id of the team a handed-over conversation goes to (starts with team_). Null " +
				"when it goes to no team.",
			"include_contact_context": "How much of the customer's record the agent reads: none or basic.",
			"languages":               "Language tags the agent answers in, such as en or pt-BR.",
			"link_allowlist":          "Hosts the agent may link to, such as example.com or *.example.com.",
			"max_ai_replies":          "Most replies the agent sends in one conversation before it hands over.",
			"name":                    "What the team calls the agent.",
			"pii_redaction": "true when personal data is masked in text before it is sent to the model " +
				"provider.",
			"product_name": "The product's name as the agent says it. Null when it uses the workspace's name.",
			"raise_priority_on_handoff": "true when a conversation becomes high priority as the agent hands " +
				"it over.",
			"require_citations": "true when the agent answers only with what it can cite from the knowledge.",
			"schedule":          "When the agent answers: always, outside_office_hours or inside_office_hours.",
			"streaming":         "true when the agent's reply appears in the messenger as it is written.",
			"tone":              "How the agent sounds: friendly, professional or concise.",
			"transfer_agent_ids": "Ids of the agents this one may pass a conversation to, each starting " +
				"with aia_.",
			"unknown_behavior": "What the agent does when it cannot answer: offer_handoff, handoff or " +
				"say_unknown.",
		},
	}
}

/*
agentScopeSchemaTexts returns the texts of what an agent may use and
where it takes new conversations.
*/
func agentScopeSchemaTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"AIKnowledgeScope": {
			"help_center_ids": "With mode selected: ids of the help centers the agent may search, each " +
				"starting with hc_.",
			"mode": "all when the agent may search every help center and knowledge source; selected when " +
				"only the ones listed.",
			"source_ids": "With mode selected: ids of the knowledge sources the agent may search, each " +
				"starting with ks_.",
		},
		"AIScope": {
			"ids":  "With mode selected: ids of the procedures or actions the agent may use.",
			"mode": "all when the agent may use every one; selected when only the ones in ids.",
		},
		"AIPlacement": {
			"channel_connection_ids": "Ids of the channel connections whose new conversations the agent " +
				"takes, each starting with chn_.",
			"channel_room_ids": "Ids of the Slack channels whose new conversations the agent takes, each " +
				"starting with chr_.",
			"inbox_ids": "Ids of the inboxes whose new conversations the agent takes, each starting with " +
				"inbox_.",
		},
		"AIPlacementTarget": {
			"agent_id": "Id of the agent placed there now (starts with aia_). Null where the default agent " +
				"answers.",
			"channel": "The connection's type, such as widget or email, the room's kind, or inbox.",
			"kind":    "What the place is: connection, room or inbox.",
			"name":    "The place's name as the team sees it.",
		},
	}
}

/*
agentRunSchemaTexts returns the texts of a preview turn and a transfer
it made.
*/
func agentRunSchemaTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"AIAgentPreview": {
			"action_calls": "The actions the agent called this turn, in order, each with what it was called " +
				"with and what it answered. No call reached the workspace's own systems.",
			"outcome": "How the turn ended, such as answered, unknown, handoff, refused or error.",
			"procedure_trace": "The procedure the agent followed this turn and the steps it took, as a JSON " +
				"object. Left out when it followed none.",
			"reply":  "What the agent wrote to the customer this turn. Empty when it wrote nothing.",
			"run_id": "Id of the run that answered this turn.",
		},
		"SimTransfer": {
			"agent_id":  "Id of the agent the conversation was passed to (starts with aia_).",
			"from_name": "Name of the agent that passed the conversation on.",
			"name":      "Name of the agent the conversation was passed to.",
			"summary":   "What the first agent told the next one about the conversation so far.",
		},
	}
}

/*
agentSimulationSchemaTexts returns the texts of a saved simulation, its
history and its runs.
*/
func agentSimulationSchemaTexts() map[string]map[string]string {
	result := func(where string) string {
		return "How its last run on the " + where + " went, as a JSON object with passed, ran_at, turns, " +
			"assertion_results and, on a broken run, error. Null when it never ran there."
	}

	return map[string]map[string]string{
		"AIAgentSimulation": {
			"action_mocks": "What each mocked action answers, as a JSON object keyed by action, each with " +
				"an HTTP status and a body.",
			"agent_id": "Id of the agent the simulation belongs to (starts with aia_).",
			"assertions": "What must hold after the simulated conversation, as a JSON array of checks, such " +
				"as an action call, a handover, reply text or a check written in words.",
			"draft_result": result("draft"),
			"live_result":  result("live version"),
			"messages": "The customer's messages, in order, as a JSON array; each has text and an optional " +
				"confirm.",
			"name": "Name of the simulation, up to 120 characters.",
			"persona": "The made-up customer, as a JSON object with fields such as name, email, verified, " +
				"channel, attributes and company.",
			"scenario": "Who the customer is and what they came for. The simulated customer answers from it.",
			"simulated_at": "The moment the simulation pretends it is, so date rules such as a refund window " +
				"give the same result every run. Null to run at the real time.",
			"source_conversation_id": "Id of the real conversation the simulation was made from (starts with " +
				"conv_). Null when it was written by hand.",
		},
		"AIAgentSimulationHistory": {
			"passed": "How many of the counted runs passed.",
			"runs":   "The counted runs, newest first.",
			"state":  "passing when every counted run passed, failing when none did, flaky in between.",
			"total":  "How many runs are counted.",
			"window": "The most runs a history counts: only the latest this many.",
		},
		"AIAgentSimulationHistoryRun": {
			"passed": "true when every check of the run held.",
			"ran_at": "When the run happened.",
		},
		"AIAgentSimulationRun": {
			"counted": "true when the run counts toward the simulation's history: it ran on the agent, the " +
				"procedures and the simulation as they stand now.",
			"errored": "true when the run broke before its checks could be judged, which is not a pass.",
			"passed":  "true when every check of the run held.",
			"ran_at":  "When the run happened.",
			"result": "How the run went, as a JSON object with the turns, the result of each check and, on " +
				"a broken run, the error.",
			"simulation_id": "Id of the simulation that ran (starts with aias_).",
			"version":       "The copy of the agent the run was on: draft or live.",
		},
	}
}

/*
sharedAISchemaTexts returns the texts of the settings every agent
shares.
*/
func sharedAISchemaTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"AISettings": {
			"answer_key_id": "Id of the provider key every agent answers with until it picks its own " +
				"(starts with aik_). Null when none is set.",
			"answer_model": "The model every agent answers with until it picks its own. Null when none is set.",
			"classifier_key_id": "Id of the provider key that sorts each customer message before an agent " +
				"answers (starts with aik_). Null when none is set.",
			"classifier_model": "The model that sorts each customer message. Null when none is set.",
			"fallback_key_id": "Id of the provider key used when an agent's answer key fails (starts with " +
				"aik_). Null when there is none.",
			"fallback_model": "The model used when an agent's answer model fails. Null when there is none.",
			"monthly_spend_cap_micros": "Most the workspace's AI may spend a month, in millionths of a US " +
				"dollar. Null when there is no cap.",
		},
	}
}

/*
mergedSchemaTexts returns the texts of fields that reached records
described elsewhere after their texts were written.
*/
func mergedSchemaTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorRoutineRun": {
			"suggestions_waiting": "How many of the run's suggestions nobody has applied or declined yet.",
		},
		"OperatorRoutineRunRead": {
			"articles":      "How many help articles the run opened, each counted once.",
			"conversations": "How many conversations the run opened, each counted once.",
			"procedures":    "How many procedures the run opened, each counted once.",
		},
		"ProcedureTestCase": {
			"scenario": "Who the customer is and what they came for. The simulated customer answers from it.",
		},
		"WorkspaceSummary": {
			"logo_url": "Address of the workspace's logo on this server. Null when it has none.",
		},
	}
}
