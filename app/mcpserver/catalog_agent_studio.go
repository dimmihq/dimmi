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
		agentListDocs(), agentVersionDocs(), agentPreviewDocs(), agentSimulationDocs(), agentSimulationRunDocs(),
		sharedAIDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
agentListDocs returns the entries for creating, reading and changing
agents.
*/
func agentListDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.index": {
			Title:   "List AI agents",
			Use:     "Lists the workspace's AI agents: what each handles, whether it is live, and which is default.",
			Returns: "the agents with their name, description, status and whether a draft waits to be published.",
		},
		"ai.agent-templates.index": {
			Title:   "List AI agent templates",
			Use:     "Lists what a new agent can start from, such as support or sales, with its starting guidance.",
			Returns: "the templates with their key, name and the guidance a new agent starts with.",
		},
		"ai.agents.store": {
			Title: "Create an AI agent",
			Use: "Creates an agent, from a template or empty. It starts switched off, as a draft: customers meet " +
				"it only once it is published and placed somewhere.",
			Returns: "the new agent with its draft.",
			Params: map[string]string{
				"template":    "A template key from the templates list; leave out to start empty.",
				"description": "What the agent handles. Other agents read it to know when to pass a conversation.",
			},
		},
		"ai.agents.show": {
			Title: "Get an AI agent",
			Use: "Reads one agent: its guidance, tone, channels, models, handover rules, where it is placed, and " +
				"the draft a teammate is working on beside the version that is live.",
			Returns: "the agent with its live definition and its draft, when it has one.",
		},
		"ai.agents.update": {
			Title: "Edit an AI agent's draft",
			Use: "Changes the agent's draft: guidance, tone, models, knowledge, handover, transfers or placement. " +
				"Customers see no change until the draft is published.",
			Returns: "the agent with the draft as saved.",
			Risk:    new(RiskWrite),
			Params: map[string]string{
				"guidance":  "How the agent talks and what it does, in the team's own words.",
				"placement": "Where it takes new conversations: channel connections, Slack channels and inboxes.",
			},
		},
		"ai.agents.destroy": {
			Title: "Delete an AI agent",
			Use: "Deletes an agent that is not the default, with its versions and simulations. Conversations it " +
				"would have taken go to the default agent.",
			Returns: "nothing.",
		},
	}
}

/*
agentVersionDocs returns the entries for publishing an agent and its
past versions.
*/
func agentVersionDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.publications.store": {
			Title: "Publish an AI agent",
			Use: "Makes the agent's draft live: from now on customers meet the agent as the draft describes it. " +
				"Run its simulations first.",
			Returns: "the agent as published, with its new version number.",
			Risk:    new(RiskReach),
			Params:  map[string]string{"change_note": "What changed and why, for the version history."},
		},
		"ai.agents.publications.destroy": {
			Title: "Switch an AI agent off",
			Use: "Stops the agent taking conversations. Its draft, versions and simulations are kept, and it can " +
				"be published again.",
			Returns: "the agent, now off.",
			Risk:    new(RiskReach),
		},
		"ai.agents.versions.index": {
			Title:   "List an AI agent's versions",
			Use:     "Lists every published version of an agent, newest first, with who published it and why.",
			Returns: "the versions with their number, note and publish time.",
		},
		"ai.agents.versions.show": {
			Title:   "Get an AI agent version",
			Use:     "Reads one published version of an agent as it was.",
			Returns: "the version with its full definition.",
		},
		"ai.agents.versions.restore.store": {
			Title: "Restore an AI agent version",
			Use: "Copies a published version into the agent's draft. Customers see no change until the draft is " +
				"published again.",
			Returns: "the agent with the restored draft.",
		},
	}
}

/*
agentPreviewDocs returns the entry for a preview turn.
*/
func agentPreviewDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.preview.store": {
			Title: "Preview an AI agent turn",
			Use: "Sends one customer message to the agent's draft or live version and returns its reply. Nothing " +
				"reaches a customer, and actions answer with their saved test result, never your systems. Every " +
				"turn runs real models and spends the workspace's AI budget.",
			Returns: "the agent's reply, what it did on the way, and whether it handed over.",
			Params: map[string]string{
				"message": "What the customer writes.",
				"history": "The turns so far, oldest first, as returned by earlier calls.",
				"version": "draft tests the working copy, live the published one.",
			},
		},
	}
}

/*
agentSimulationDocs returns the entries for saved simulations.
*/
func agentSimulationDocs() map[string]Doc {
	return map[string]Doc{
		"ai.agents.simulations.index": {
			Title: "List an AI agent's simulations",
			Use: "Lists the agent's saved simulations and how each has gone lately: passing, flaky or failing over " +
				"its recent runs.",
			Returns: "the simulations with their messages, checks, last result and recent history.",
		},
		"ai.agents.simulations.show": {
			Title:   "Get an AI agent simulation",
			Use:     "Reads one saved simulation of an agent, with its last result turn by turn.",
			Returns: "the simulation with its scenario, messages, checks and last result.",
		},
		"ai.agents.simulations.store": {
			Title: "Save an AI agent simulation",
			Use: "Saves a conversation the agent must keep getting right: who the customer is, what they write, " +
				"and the checks that say whether the agent did what it should.",
			Returns: "the saved simulation.",
			Params: map[string]string{
				"scenario":   "Who the customer is and what they came for; the simulated customer answers from it.",
				"messages":   "The customer's messages, oldest first, at most 12.",
				"assertions": "Checks such as action_called, handed_off, or llm_rubric for one written in words.",
			},
		},
		"ai.agents.simulations.from-conversation.store": {
			Title: "Save a conversation as a simulation",
			Use: "Turns a real conversation into a simulation of the agent, with a scenario written from what " +
				"the customer said.",
			Returns: "the saved simulation.",
		},
		"ai.agents.simulations.update": {
			Title:   "Edit an AI agent simulation",
			Use:     "Changes a saved simulation's name, scenario, messages, customer or checks.",
			Returns: "the simulation as saved.",
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
			Title: "Run an AI agent's simulations",
			Use: "Replays the agent's saved simulations, all of them or the ones named, against its draft or its " +
				"live version. A simulated customer plays each one and a judge reads the result. Every run uses " +
				"real models and is tallied as simulation spend.",
			Returns: "each simulation with its verdict, the reason for every written check, and the conversation.",
			Params: map[string]string{
				"simulation_ids": "The simulations to run; leave out to run every one.",
				"version":        "draft runs the working copy, live the published one.",
			},
		},
		"ai.agents.simulations.runs.index": {
			Title: "List a simulation's past runs",
			Use: "Lists the latest runs of one simulation, newest first, to see whether it passes steadily or " +
				"only sometimes.",
			Returns: "the runs with their verdict, when they ran and what was said.",
			Params:  map[string]string{"version": "draft lists runs on the working copy, live on the published one."},
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
		},
		"ai.settings.update": {
			Title:   "Change shared AI settings",
			Use:     "Changes the models every agent falls back on, or the workspace's monthly AI spend cap.",
			Returns: "the settings as saved.",
		},
		"ai.placements.index": {
			Title: "List where AI agents can be placed",
			Use: "Lists everywhere an agent can take new conversations, channel connections, Slack channels and " +
				"inboxes, with the agent placed at each now.",
			Returns: "the places with their kind and the agent there, null where the default agent answers.",
		},
	}
}
