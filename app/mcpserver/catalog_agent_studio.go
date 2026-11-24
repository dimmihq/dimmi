/*
|--------------------------------------------------------------------------
| Tool Catalog: AI Agents
|--------------------------------------------------------------------------
|
| The entries for a workspace's AI agents: creating and editing them,
| the draft a teammate works on and the version customers meet, where
| each agent takes new conversations, a preview turn, the saved
| simulations that say whether a change is safe to publish, and the
| settings every agent shares.
|
*/

package mcpserver

import "maps"

/*
agentStudioDocs returns the entries for AI agents, their versions,
their simulations and the shared AI settings.
*/
func agentStudioDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		agentListDocs(), agentDraftDocs(), agentVersionDocs(), agentPastVersionDocs(), agentPreviewDocs(),
		agentSimulationDocs(), agentSimulationEditDocs(), agentSimulationRunDocs(), sharedAIDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
agentListDocs returns the entries for listing and creating agents.
*/
func agentListDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.index": {
			Title:   "List AI agents",
			Use:     "Lists the workspace's AI agents: what each handles, whether it is live, and which is default.",
			Returns: "the agents with their name, description, status and whether a draft waits to be published.",
			Next: []string{
				"dimmi_get_ai_agent_by_id: read one agent with its draft",
				"dimmi_create_ai_agent: add an agent for another kind of work",
			},
		},
		"ai.agent-templates.index": {
			Title:   "List AI agent templates",
			Use:     "Lists what a new agent can start from, such as support or sales, with its starting guidance.",
			Returns: "the templates with their key, name and the guidance a new agent starts with.",
			Next:    []string{"dimmi_create_ai_agent: start an agent from a template's key"},
		},
		"ai.agents.store": {
			Title: "Create an AI agent",
			Use: "Creates an agent, from a template or empty. It starts switched off, as a draft: customers meet " +
				"it only once it is published and placed somewhere.",
			Returns: "the new agent with its draft.",
			Next: []string{
				"dimmi_update_ai_agent: write its guidance and say where it takes conversations",
				"dimmi_publish_ai_agent: make it live once its simulations pass",
			},
			Params: map[string]string{
				"name": "What the team calls the agent, 1 to 60 characters, such as Billing.",
				"template": "A template's key from dimmi_list_ai_agent_templates, such as support; leave out to " +
					"start empty.",
			},
		},
	}
}

/*
agentDraftDocs returns the entries for reading, changing and deleting
one agent.
*/
func agentDraftDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.show": {
			Title: "Get an AI agent",
			Use: "Reads one agent: its guidance, tone, channels, models, handover rules, where it is placed, and " +
				"the draft a teammate is working on beside the version that is live.",
			Returns: "the agent with its live definition and its draft, when it has one.",
			Next: []string{
				"dimmi_update_ai_agent: change the agent's draft",
				"dimmi_list_ai_agent_simulations: see whether its simulations pass",
				"dimmi_list_ai_agent_versions: read what was published before",
			},
		},
		"ai.agents.update": {
			Title: "Edit an AI agent's draft",
			Use: "Changes the agent's draft: guidance, tone, models, knowledge, handover, transfers or placement. " +
				"Customers see no change until the draft is published.",
			Returns: "the agent with the draft as saved.",
			Next: []string{
				"dimmi_preview_ai_agent: try a customer message on the draft",
				"dimmi_run_ai_agent_simulations: check the draft against the saved simulations",
				"dimmi_publish_ai_agent: make the draft live",
			},
			Risk: new(RiskWrite),
			Params: map[string]string{
				"name":     "What the team calls the agent, 1 to 60 characters.",
				"guidance": "How the agent talks and what it does, in the team's own words, up to 20000 characters.",
				"description": "What the agent handles, up to 400 characters. Other agents read it to know when " +
					"to pass a conversation.",
				"placement": "Where it takes new conversations: an object with channel_connection_ids, " +
					"channel_room_ids and inbox_ids. Each list sent replaces the one saved. Find the places " +
					"with dimmi_list_ai_placements.",
			},
			Hint: "A field error names the value refused; read the agent with dimmi_get_ai_agent_by_id for " +
				"what is set now.",
		},
		"ai.agents.destroy": {
			Title: "Delete an AI agent",
			Use: "Deletes an agent that is not the default, with its versions and simulations. Conversations it " +
				"would have taken go to the default agent.",
			Returns: "nothing.",
			Hint:    "The default agent cannot be deleted; switch it off with dimmi_unpublish_ai_agent instead.",
		},
	}
}

/*
agentVersionDocs returns the entries for publishing an agent and
switching it off.
*/
func agentVersionDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.publications.store": {
			Title: "Publish an AI agent",
			Use: "Makes the agent's draft live: from now on customers meet the agent as the draft describes it. " +
				"Run its simulations first.",
			Returns: "the agent as published, with its new version number.",
			Next: []string{
				"dimmi_list_ai_agent_versions: read the version this made",
				"dimmi_unpublish_ai_agent: switch the agent off again",
			},
			Risk: new(RiskReach),
			Params: map[string]string{
				"change_note": "What changed and why, for the version history, up to 500 characters.",
			},
			Hint: "Run dimmi_run_ai_agent_simulations on the draft first; publish only when they pass.",
		},
		"ai.agents.publications.destroy": {
			Title: "Switch an AI agent off",
			Use: "Stops the agent taking conversations. Its draft, versions and simulations are kept, and it can " +
				"be published again.",
			Returns: "the agent, now off.",
			Next:    []string{"dimmi_publish_ai_agent: switch the agent on again"},
			Risk:    new(RiskReach),
		},
	}
}

/*
agentPastVersionDocs returns the entries for an agent's past versions.
*/
func agentPastVersionDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.versions.index": {
			Title:   "List an AI agent's versions",
			Use:     "Lists every published version of an agent, newest first, with who published it and why.",
			Returns: "the versions with their number, note and publish time.",
			Next: []string{
				"dimmi_get_ai_agent_version_by_id: read one version in full",
				"dimmi_restore_ai_agent_version: copy a version back into the draft",
			},
		},
		"ai.agents.versions.show": {
			Title:   "Get an AI agent version",
			Use:     "Reads one published version of an agent as it was.",
			Returns: "the version with its full definition.",
			Next:    []string{"dimmi_restore_ai_agent_version: copy this version back into the draft"},
			Params:  map[string]string{"version": agentVersionText()},
		},
		"ai.agents.versions.restore.store": {
			Title: "Restore an AI agent version",
			Use: "Copies a published version into the agent's draft. Customers see no change until the draft is " +
				"published again.",
			Returns: "the agent with the restored draft.",
			Next: []string{
				"dimmi_run_ai_agent_simulations: check the restored draft",
				"dimmi_publish_ai_agent: make the restored draft live",
			},
			Params: map[string]string{"version": agentVersionText()},
		},
	}
}

/*
agentPreviewDocs returns the entry for a preview turn.
*/
func agentPreviewDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.preview.store": {
			Name:  "preview_ai_agent",
			Title: "Preview an AI agent turn",
			Use: "Sends one customer message to the agent's draft or live version and returns its reply. Nothing " +
				"reaches a customer, and actions answer with their saved test result, never your systems. Every " +
				"turn runs real models and spends the workspace's AI budget.",
			Returns: "the agent's reply, what it did on the way, and whether it handed over.",
			Next: []string{
				"dimmi_update_ai_agent: adjust the draft's guidance if the reply was off",
				"dimmi_create_ai_agent_simulation: save the conversation so it keeps being checked",
			},
			Params: map[string]string{
				"message": "What the customer writes, up to 4000 characters.",
				"history": "The turns so far, oldest first, at most 40, as returned by earlier calls.",
				"version": "draft tests the working copy, live the published one. Left out, it is draft.",
				"source_conversation_id": "A real conversation (conv_...) whose messages the preview replays " +
					"as the turns so far. The conversation is not changed.",
			},
		},
	}
}

/*
agentSimulationDocs returns the entries for reading and saving
simulations.
*/
func agentSimulationDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.simulations.index": {
			Title: "List an AI agent's simulations",
			Use: "Lists the agent's saved simulations and how each has gone lately: passing, flaky or failing over " +
				"its recent runs.",
			Returns: "the simulations with their messages, checks, last result and recent history.",
			Next: []string{
				"dimmi_run_ai_agent_simulations: replay them against the draft or the live version",
				"dimmi_get_ai_agent_simulation_by_id: read one with its last result turn by turn",
			},
		},
		"ai.agents.simulations.show": {
			Title:   "Get an AI agent simulation",
			Use:     "Reads one saved simulation of an agent, with its last result turn by turn.",
			Returns: "the simulation with its scenario, messages, checks and last result.",
			Next: []string{
				"dimmi_list_ai_agent_simulation_runs: see whether it passes steadily",
				"dimmi_update_ai_agent_simulation: change its messages or checks",
			},
		},
		"ai.agents.simulations.store": {
			Title: "Save an AI agent simulation",
			Use: "Saves a conversation the agent must keep getting right: who the customer is, what they write, " +
				"and the checks that say whether the agent did what it should.",
			Returns: "the saved simulation.",
			Next:    []string{"dimmi_run_ai_agent_simulations: run it against the draft"},
			Params:  agentSimulationParams(),
		},
	}
}

/*
agentSimulationEditDocs returns the entries for saving a conversation
as a simulation and for changing or deleting one.
*/
func agentSimulationEditDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.simulations.from-conversation.store": {
			Title: "Save a conversation as a simulation",
			Use: "Turns a real conversation into a simulation of the agent, with a scenario written from what " +
				"the customer said.",
			Returns: "the saved simulation.",
			Next: []string{
				"dimmi_update_ai_agent_simulation: add the checks that say what the agent must do",
				"dimmi_run_ai_agent_simulations: run it against the draft",
			},
			Params: map[string]string{
				"conversation_id": "The conversation to save (conv_...). Find it with dimmi_list_conversations. " +
					"The conversation is not changed.",
			},
		},
		"ai.agents.simulations.update": {
			Title:   "Edit an AI agent simulation",
			Use:     "Changes a saved simulation's name, scenario, messages, customer or checks.",
			Returns: "the simulation as saved.",
			Next:    []string{"dimmi_run_ai_agent_simulations: run it with the change"},
			Params:  agentSimulationParams(),
		},
		"ai.agents.simulations.destroy": {
			Title:   "Delete an AI agent simulation",
			Use:     "Deletes a saved simulation and its past runs.",
			Returns: "nothing.",
			Risk:    new(RiskWrite),
		},
	}
}

/*
agentSimulationRunDocs returns the entries for running simulations and
reading their past runs.
*/
func agentSimulationRunDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.simulations.run.store": {
			Name:  "run_ai_agent_simulations",
			Title: "Run an AI agent's simulations",
			Use: "Replays the agent's saved simulations, all of them or the ones named, against its draft or its " +
				"live version. A simulated customer plays each one and a judge reads the result. Every run uses " +
				"real models and is tallied as simulation spend.",
			Returns: "each simulation with its verdict, the reason for every written check, and the conversation.",
			Next: []string{
				"dimmi_update_ai_agent: fix the draft where a simulation failed",
				"dimmi_publish_ai_agent: make the draft live once they pass",
			},
			Params: map[string]string{
				"simulation_ids": "The simulations to run (aias_...), at most 20; leave out to run every one.",
				"version":        "draft runs the working copy, live the published one. Left out, it is draft.",
			},
		},
		"ai.agents.simulations.runs.index": {
			Title: "List a simulation's past runs",
			Use: "Lists the latest runs of one simulation, newest first, to see whether it passes steadily or " +
				"only sometimes.",
			Returns: "the runs with their verdict, when they ran and what was said.",
			Next:    []string{"dimmi_run_ai_agent_simulations: run it again"},
			Params: map[string]string{
				"version": "draft lists runs on the working copy, live on the published one. Left out, it is draft.",
			},
		},
	}
}

/*
sharedAIDocs returns the entries for what every agent shares.
*/
func sharedAIDocs() map[string]Doc {
	return map[string]Doc{
		"ai.settings.show": {
			Title: "Get shared AI settings",
			Use: "Reads what every agent shares: the answer and fallback models an agent uses until it picks " +
				"its own, the classifier, and the monthly spend cap.",
			Returns: "the shared models and the spend cap, null when there is none.",
			Next: []string{
				"dimmi_update_ai_settings: change the shared models or the cap",
				"dimmi_get_ai_usage: see what the agents spent this month",
			},
		},
		"ai.settings.update": {
			Title:   "Change shared AI settings",
			Use:     "Changes the models every agent falls back on, or the workspace's monthly AI spend cap.",
			Returns: "the settings as saved.",
			Next:    []string{"dimmi_get_ai_usage: see the month's spend against the cap"},
			Risk:    new(RiskReach),
			Hint:    "Only the fields sent change. Find provider keys with dimmi_list_ai_keys.",
		},
		"ai.placements.index": {
			Title: "List where AI agents can be placed",
			Use: "Lists everywhere an agent can take new conversations, channel connections, Slack channels and " +
				"inboxes, with the agent placed at each now.",
			Returns: "the places with their kind and the agent there, null where the default agent answers.",
			Next:    []string{"dimmi_update_ai_agent: place an agent with its placement argument"},
		},
	}
}

/*
agentSimulationParams explains what a saved simulation is made of, the
same when one is saved and when one is changed.
*/
func agentSimulationParams() map[string]string {
	return map[string]string{
		"name": "What the team calls the simulation, up to 120 characters, such as Refund past the window.",
		"scenario": "Who the customer is and what they came for, up to 1000 characters; the simulated " +
			"customer answers from it.",
		"messages": "The customer's messages, oldest first, at most 12, each an object with text, such as " +
			"[{\"text\": \"I want a refund for order 1042\"}].",
		"assertions": "Checks after the run, at most 30 objects with a type, such as action_called (with " +
			"action), handed_off, reply_contains (with text), or llm_rubric (with rubric) for one written in " +
			"words. Such as [{\"type\": \"handed_off\"}].",
		"source_conversation_id": "The real conversation (conv_...) the simulation was made from, when it " +
			"was. Leave out otherwise.",
	}
}

/*
agentVersionText explains the argument that names one published
version of an agent.
*/
func agentVersionText() string {
	return "The version's number, such as 3. Find it with dimmi_list_ai_agent_versions."
}
