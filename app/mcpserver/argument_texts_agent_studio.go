/*
|--------------------------------------------------------------------------
| Written Argument Texts: AI Agents
|--------------------------------------------------------------------------
|
| The arguments of the tools for a workspace's AI agents: an agent's
| draft, a preview turn, its saved simulations and the settings every
| agent shares. A simulation of an agent and a test of a procedure take
| the same made-up customer, mocks and state, so the texts they share
| are written once and given to each tool.
|
*/

package mcpserver

/*
agentStudioTexts returns the texts written for these tools.
*/
func agentStudioTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range []map[string]map[string]string{
		agentDraftTexts(), agentScopeTexts(), agentPreviewTexts(), agentSimulationTexts(), sharedAITexts(),
	} {
		mergeTexts(out, part)
	}

	return out
}

/*
agentDraftTexts returns texts of update_ai_agent: how the agent talks,
when it answers and when it hands over.
*/
func agentDraftTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"update_ai_agent": {
			"tone": "How the agent sounds: friendly, professional or concise, such as \"friendly\".",
			"enabled_channels": "The kinds of channel the agent answers on, replacing the list: widget (the " +
				"messenger), email and api, at most 10, such as [\"widget\", \"email\"].",
			"schedule": "When the agent answers: always, outside_office_hours or inside_office_hours, such as " +
				"\"always\".",
			"answer_key_id": "Provider key the agent answers with, starting with aik_, such as aik_01jx2m8k4t. " +
				"Find it with dimmi_list_ai_keys. Send null to use the key every agent shares, set with " +
				"dimmi_update_ai_settings.",
			"answer_model": "Model the agent answers with, one of the models its answer key lists, at most 120 " +
				"characters, such as \"claude-haiku-5-5\". Send null to use the shared model.",
			"fallback_key_id": "Provider key used when the answer key fails, starting with aik_, such as " +
				"aik_01jx2m8k4t. Find it with dimmi_list_ai_keys. Send null to clear it.",
			"max_ai_replies": "Most replies the agent sends in one conversation before it hands over, 1 to " +
				"50, such as 10.",
			"languages": "Language tags the agent answers in, replacing the list, at most 50, such as " +
				"[\"en\", \"pt-BR\"].",
			"default_language": "Language tag the agent answers in when the customer's language is unknown, at " +
				"most 35 characters, such as \"en\".",
			"unknown_behavior": "What the agent does when it cannot answer: offer_handoff (asks whether the " +
				"customer wants a person), handoff (hands over at once) or say_unknown (says it does not " +
				"know), such as \"offer_handoff\".",
			"assigned_behavior": "What the agent does in a conversation a teammate holds: " +
				"skip_when_human_active, never_when_assigned or always, such as \"skip_when_human_active\".",
			"include_contact_context": "How much of the customer's record the agent reads: none or basic, " +
				"such as \"basic\".",
			"link_allowlist": "Hosts the agent may link to, replacing the list, at most 100, such as " +
				"[\"example.com\", \"*.example.com\"].",
			"handoff_mode": "full lets the agent answer from the knowledge; triage_only has it only sort the " +
				"conversation and hand it to a person. Such as \"full\".",
			"handoff_team_id": "Team a handed-over conversation goes to, starting with team_, such as " +
				"team_01jx2m8k4t. Find it with dimmi_list_teams. Send null to clear it.",
			"always_handoff_tag_ids": "Tags that always send a conversation to a person, replacing the list, " +
				"at most 50, each starting with tag_, such as [\"tag_01jx2m8k4t\"]. Find them with " +
				"dimmi_list_tags.",
			"transfer_agent_ids": "The other agents this one may pass a conversation to, replacing the list, at " +
				"most 20, each starting with aia_, such as [\"aia_01jx2m8k4t\"]. Find them with " +
				"dimmi_list_ai_agents. Each is offered by its description.",
		},
	}
}

/*
agentScopeTexts returns texts of update_ai_agent: what the agent may
use and where it takes new conversations.
*/
func agentScopeTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"update_ai_agent": {
			"knowledge_scope": "What the agent may search: an object with mode, \"all\" or \"selected\", and " +
				"with \"selected\" the help_center_ids and source_ids it may read, such as {\"mode\": " +
				"\"selected\", \"help_center_ids\": [\"hc_01jx2m8k4t\"]}.",
			"knowledge_scope.mode": "\"all\" lets the agent search every help center and knowledge source; " +
				"\"selected\" only the ones named beside it.",
			"knowledge_scope.help_center_ids": "With mode \"selected\": the help centers the agent may " +
				"search, at most 50, each starting with hc_, such as [\"hc_01jx2m8k4t\"]. Find them with " +
				"dimmi_list_help_centers.",
			"knowledge_scope.source_ids": "With mode \"selected\": the knowledge sources the agent may " +
				"search, at most 200, each starting with ks_, such as [\"ks_01jx2m8k4t\"]. Find them with " +
				"dimmi_list_knowledge_sources.",
			"procedure_scope": "Which procedures the agent may follow: an object with mode, \"all\" or " +
				"\"selected\", and with \"selected\" their ids, such as {\"mode\": \"selected\", \"ids\": " +
				"[\"prc_01jx2m8k4t\"]}.",
			"procedure_scope.mode": "\"all\" lets the agent follow every live procedure; \"selected\" only " +
				"the ones in ids.",
			"procedure_scope.ids": "With mode \"selected\": the procedures the agent may follow, at most 200, " +
				"each starting with prc_, such as [\"prc_01jx2m8k4t\"]. Find them with dimmi_list_procedures.",
			"action_scope": "Which actions the agent may call: an object with mode, \"all\" or \"selected\", " +
				"and with \"selected\" their ids, such as {\"mode\": \"selected\", \"ids\": " +
				"[\"aac_01jx2m8k4t\"]}.",
			"action_scope.mode": "\"all\" lets the agent call every live action; \"selected\" only the ones " +
				"in ids.",
			"action_scope.ids": "With mode \"selected\": the actions the agent may call, at most 200, each " +
				"starting with aac_, such as [\"aac_01jx2m8k4t\"]. Find them with dimmi_list_ai_actions.",
			"placement.channel_connection_ids": "The channel connections (a messenger, an email address, a " +
				"number) whose new conversations this agent takes, replacing the list, at most 100, each " +
				"starting with chn_, such as [\"chn_01jx2m8k4t\"]. Find them with dimmi_list_ai_placements.",
			"placement.channel_room_ids": "The Slack channels whose new conversations this agent takes, " +
				"replacing the list, at most 200, each starting with chr_, such as [\"chr_01jx2m8k4t\"]. " +
				"Find them with dimmi_list_ai_placements.",
			"placement.inbox_ids": "The inboxes whose new conversations this agent takes, replacing the list, " +
				"at most 100, each starting with inbox_, such as [\"inbox_01jx2m8k4t\"]. Find them with " +
				"dimmi_list_ai_placements.",
		},
	}
}

/*
agentPreviewTexts returns texts of preview_ai_agent.
*/
func agentPreviewTexts() map[string]map[string]string {
	state := "the state the last preview turn handed back"

	return map[string]map[string]string{
		"preview_ai_agent": {
			"action_mocks": actionMocksText(),
			"persona":      personaText(),
			"confirmed_action_ids": "Ids of the waiting writes you confirm or approve this turn, taken from " +
				"action_id in the pending list of " + state + " (send that state as active_state). At most " +
				"10, such as [\"aac_01jx2m8k4t\"]. You may send these without a message.",
			"stream_id": streamText("the turn reports its progress"),
			"active_state": "The state the last preview turn handed back, sent as it came, so the agent picks " +
				"up where it was: the procedure it is in, the steps done and the writes still waiting. Leave " +
				"out on the first turn.",
			"active_state.branches": "Copy it unchanged from " + state + ": an object of step id to the side " +
				"that step took.",
			"active_state.called_action_ids": "Copy it unchanged from " + state + ": the list of ids of the " +
				"actions already called, such as [\"aac_01jx2m8k4t\"].",
			"active_state.completed_step_ids": "Copy it unchanged from " + state + ": the list of ids of the " +
				"procedure's steps already done.",
			"active_state.pending": "Copy it unchanged from " + state + ": the list of writes still waiting " +
				"to be confirmed or approved, each with action_id, tool, args, state and summary.",
			"active_state.pending.action_id": "Id of the action whose write is waiting, starting with aac_, " +
				"such as aac_01jx2m8k4t, copied from the last turn's state. Put it in confirmed_action_ids to " +
				"confirm it.",
			"active_state.pending.args": "The arguments the waiting write would be called with, as an object, " +
				"copied unchanged from the last turn's state, such as {\"order_id\": \"1042\"}.",
			"active_state.pending.state": "Why the write waits, copied from the last turn's state: " +
				"awaiting_confirmation (the customer must confirm) or pending_approval (a teammate must " +
				"approve).",
			"active_state.pending.summary": "The short text that says what the waiting write would do, copied " +
				"unchanged from the last turn's state.",
			"active_state.pending.tool": "The tool name of the action whose write is waiting, copied unchanged " +
				"from the last turn's state.",
			"active_state.procedure_id": "Id of the procedure the conversation is in, starting with prc_, such " +
				"as prc_01jx2m8k4t, copied from " + state + ".",
			"active_state.status": "Where the procedure stands, copied from " + state + ": active, completed, " +
				"handed_off or exited. Only active carries the procedure on.",
			"active_state.step_outputs": "Copy it unchanged from " + state + ": an object of what the " +
				"procedure's steps have produced so far.",
			"history.role": "Who wrote this earlier message: customer or agent, such as \"customer\".",
			"history.text": "What that earlier message said, at most 4000 characters, such as Hi, where is my " +
				"order?",
		},
	}
}

/*
agentSimulationTexts returns texts of the tools for an agent's saved
simulations and its versions.
*/
func agentSimulationTexts() map[string]map[string]string {
	saved := map[string]string{
		"action_mocks": actionMocksText(),
		"persona":      personaText(),
		"simulated_at": "The date and time the simulation pretends it is when it runs, as an ISO 8601 time, " +
			"such as \"2026-12-24T18:00:00Z\". Left out, it runs at the real current time.",
	}

	return map[string]map[string]string{
		"create_ai_agent_simulation": saved,
		"update_ai_agent_simulation": saved,
		"run_ai_agent_simulations": {
			"stream_id": streamText("the run reports each simulation as it finishes"),
		},
		"list_ai_agent_versions": {
			"before": "Version number to page from: only versions older than it are listed. Leave out for " +
				"the newest, such as 12.",
		},
	}
}

/*
sharedAITexts returns texts of update_ai_settings.
*/
func sharedAITexts() map[string]map[string]string {
	key := func(use string) string {
		return "Provider key " + use + ", starting with aik_, such as aik_01jx2m8k4t. Find it with " +
			"dimmi_list_ai_keys. Send null to clear it."
	}
	model := func(use, of string) string {
		return "Model " + use + ", one of the models the " + of + " key lists, at most 120 characters, such " +
			"as \"claude-haiku-5-5\". Send null to clear it."
	}

	return map[string]map[string]string{
		"update_ai_settings": {
			"answer_key_id":     key("every agent answers with until it picks its own"),
			"answer_model":      model("every agent answers with until it picks its own", "answer"),
			"classifier_key_id": key("that sorts each customer message before an agent answers"),
			"classifier_model":  model("that sorts each customer message", "classifier"),
			"fallback_key_id":   key("used when an agent's answer key fails"),
			"fallback_model":    model("used when an agent's answer model fails", "fallback"),
			"monthly_cap_usd": "Most the workspace's agents may spend a month, in US dollars, 0 to 100000, " +
				"such as 250. 0 or null means no cap. Raising it lifts a pause the old cap caused.",
		},
	}
}

/*
actionMocksText explains the mocks a preview or a simulation takes.
*/
func actionMocksText() string {
	return "What each action answers instead of really being called, as an object keyed by action id; each " +
		"value has status (HTTP status, 100 to 599) and body (any JSON), such as {\"aac_abc123\": " +
		"{\"status\": 200, \"body\": {\"order\": {\"status\": \"shipped\"}}}}. An action with no mock " +
		"answers with the result of its last live test."
}

/*
personaText explains the made-up customer a preview or a simulation
talks with.
*/
func personaText() string {
	return "The made-up customer, an object with optional name, email, external_id, verified (true when " +
		"signed in), channel (widget, email, slack, whatsapp, instagram, facebook or api), inbox_id, " +
		"attributes and company (objects), such as {\"email\": \"ada@example.com\", \"verified\": true, " +
		"\"attributes\": {\"plan\": \"pro\"}}."
}

/*
streamText explains the id a client listens on while models run.
*/
func streamText(what string) string {
	return "An id you make up, at most 64 characters, so " + what + " live on your own realtime channel " +
		"while it happens. Only useful to a client listening on that channel; leave it out otherwise. Such " +
		"as \"sim-7f3a2c\"."
}
