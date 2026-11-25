/*
|--------------------------------------------------------------------------
| Tool Catalog: Workflow And Series Simulations
|--------------------------------------------------------------------------
|
| The entries for simulating a workflow with a made-up customer, and a
| series with one person walked through it, and the texts of what they
| answer.
|
*/

package mcpserver

/*
workflowSimulationParams returns the texts of a saved simulation's
fields.
*/
func workflowSimulationParams() map[string]string {
	return map[string]string{
		"name":     "What the team calls the simulation, up to 120 characters.",
		"scenario": "Who the customer is and what they came for; the simulated customer answers from it.",
		"messages": "The customer's messages, oldest first, each an object with text.",
		"assertions": "Checks after the run, such as workflow_started, step_reached (with step_id), " +
			"path_taken (with branch, a path id), handed_off, or llm_rubric for one written in words.",
		"source_conversation_id": "The real conversation (conv_...) the simulation was made from, when it was.",
		"persona": "The made-up customer, an object with fields such as name, email, channel and " +
			"attributes, such as {\"name\": \"Ada\", \"channel\": \"widget\"}.",
		"action_mocks": "What steps answer instead of real calls, keyed by step id, each with status and body: " +
			"an HTTP request or run action step (also keyed by action id, aac_...) its response, where a " +
			"status outside 2xx takes the failure path; an extract step the values by key, such as " +
			"{\"s_extract\": {\"body\": {\"order_number\": \"A-1\"}}}; a classify step {\"path_id\": ...}.",
	}
}

/*
workflowSimulationDocs returns the entries for workflow and series
simulations.
*/
func workflowSimulationDocs() map[string]Doc {
	params := workflowSimulationParams()

	return map[string]Doc{
		"workflows.simulations.index": {
			Title:   "List a workflow's simulations",
			Use:     "Lists the workflow's saved simulations and how each has gone lately.",
			Returns: "the simulations with their messages, checks, last result and recent history.",
		},
		"workflows.simulations.show": {
			Title:   "Get a workflow simulation",
			Use:     "Reads one saved simulation of a workflow, with its last result step by step.",
			Returns: "the simulation with its scenario, messages, checks and last result.",
		},
		"workflows.simulations.store": {
			Title: "Save a workflow simulation",
			Use: "Saves a conversation the workflow must keep handling right: who the customer is, what they " +
				"write, and the checks on the steps and paths it takes.",
			Returns: "the saved simulation.",
			Params:  params,
		},
		"workflows.simulations.from-conversation.store": {
			Title:   "Save a conversation as a workflow simulation",
			Use:     "Turns a real conversation into a simulation of the workflow.",
			Returns: "the saved simulation.",
			Params:  map[string]string{"conversation_id": "The conversation to save (conv_...). It is not changed."},
		},
		"workflows.simulations.update": {
			Title:   "Edit a workflow simulation",
			Use:     "Changes a saved simulation's name, scenario, messages, customer or checks.",
			Returns: "the simulation as saved.",
			Params:  params,
		},
		"workflows.simulations.destroy": {
			Title:   "Delete a workflow simulation",
			Use:     "Deletes a saved simulation and its past runs.",
			Returns: "nothing.",
			Risk:    new(RiskWrite),
		},
	}
}

/*
workflowSimulationRunDocs returns the entries for running simulations,
reading their past runs and walking a person through a series.
*/
func workflowSimulationRunDocs() map[string]Doc {
	return map[string]Doc{
		"workflows.simulations.run.store": {
			Name:  "run_workflow_simulations",
			Title: "Run a workflow's simulations",
			Use: "Replays the workflow's saved simulations against its draft or live version, with nothing sent " +
				"and no action run. A simulated customer taps buttons and answers questions, agent steps answer " +
				"as the agent would, and a judge reads the result. Every run uses real models.",
			Returns: "each simulation with its verdict, the trigger verdict and every step and path taken.",
			Params: map[string]string{
				"simulation_ids": "The simulations to run (wfs_...); leave out to run every one.",
				"version":        "draft or live. Left out, it is draft.",
				"stream_id": "An id you make up, at most 64 characters, so the run reports its progress live " +
					"while it goes.",
			},
		},
		"workflows.simulations.runs.index": {
			Title:   "List a workflow simulation's past runs",
			Use:     "Lists the latest runs of one simulation, newest first.",
			Returns: "the runs with their verdict and when they ran.",
			Params:  map[string]string{"version": "draft or live. Left out, it is draft."},
		},
		"series.simulations.run.store": {
			Name:  "simulate_series",
			Title: "Walk one person through a series",
			Use: "Walks a contact, or a made-up person, through a series' draft or live version without sending " +
				"anything, skipping every wait. AI paths and extract steps still ask the AI unless mocked; run " +
				"action steps never run the action, and answer with their mock.",
			Returns: "each step with the path it took, the messages that would go out, the changes, whether " +
				"the person reached the goal, and with a scenario, what they did each time the series asked.",
			Params: map[string]string{
				"version":    "draft or live. Left out, it is draft.",
				"contact_id": "The contact to walk through (cnt_...); or leave out and describe a person.",
				"person":     "A made-up person, with name, email and attributes.",
				"events":     "Events the person does on the way, by name.",
				"engaged":    "Whether they open, click or reply to the messages: opened, clicked or replied.",
				"scenario": "How the person behaves, in words, such as opens every email but never clicks. The AI " +
					"reads it to say whether they open, click, reply or do an event each time the series asks, " +
					"in place of events and engaged.",
				"action_mocks": "What extract and run action steps answer instead of the AI or the action, keyed " +
					"by step id or action id (aac_...): a status and a body. An extract step's body holds the " +
					"values by key; a run action step's body is the action's result, and a status outside 2xx " +
					"takes the failed path.",
			},
		},
	}
}

/*
workflowSimulationSchemaTexts returns the texts of workflow and series
simulations.
*/
func workflowSimulationSchemaTexts() map[string]map[string]string {
	result := func(where string) string {
		return "How its last run on the " + where + " went, as a JSON object with passed, ran_at, the trace " +
			"of steps, the result of each check and, on a broken run, error. Null when it never ran there."
	}

	return map[string]map[string]string{
		"WorkflowSimulation": {
			"action_mocks": "What each mocked step answers instead of a real call, by step id, each with a " +
				"status and a body: an HTTP request or run action step (also keyed by action id, aac_...) its " +
				"response, an extract step the values by key, a classify step the path as path_id.",
			"assertions":   "What must hold after the run, as a JSON array of checks.",
			"draft_result": result("draft"),
			"live_result":  result("live version"),
			"messages":     "The customer's messages, in order, as a JSON array.",
			"name":         "Name of the simulation, up to 120 characters.",
			"persona":      "The made-up customer, as a JSON object with fields such as name, email and attributes.",
			"scenario":     "Who the customer is and what they came for. The simulated customer answers from it.",
			"simulated_at": "The moment the simulation pretends it is. Null to run at the real time.",
			"source_conversation_id": "Id of the real conversation the simulation was made from (conv_...). " +
				"Null when written by hand.",
			"workflow_id": "Id of the workflow the simulation belongs to (wf_...).",
		},
		"WorkflowSimulationHistory": {
			"passed": "How many of the counted runs passed.",
			"runs":   "The counted runs, newest first.",
			"state":  "passing when every counted run passed, failing when none did, flaky in between.",
			"total":  "How many runs are counted.",
			"window": "The most runs a history counts: only the latest this many.",
		},
		"WorkflowSimulationHistoryRun": {
			"passed": "true when every check of the run held.",
			"ran_at": "When the run happened.",
		},
		"WorkflowSimulationRun": {
			"counted":       "true when the run counts toward the simulation's history.",
			"errored":       "true when the run broke before its checks could be judged, which is not a pass.",
			"passed":        "true when every check of the run held.",
			"ran_at":        "When the run happened.",
			"result":        "How the run went, as a JSON object with the trace, each check's result and any error.",
			"simulation_id": "Id of the simulation that ran (wfs_...).",
			"version":       "The copy of the workflow the run was on: draft or live.",
		},
	}
}

/*
seriesSimulationSchemaTexts returns the texts of a series' dry run.
*/
func seriesSimulationSchemaTexts() map[string]map[string]string {
	return map[string]map[string]string{
		"SeriesSimulation": {
			"actions": "The changes the journey would make, such as tags and attributes, in order.",
			"decisions": "With a scenario, what the person did each time the series asked whether they " +
				"open, click, reply or do an event. Empty without a scenario.",
			"end":          "How the journey ended: finished, goal, exited or still waiting.",
			"entered":      "true when the person would enter the series.",
			"goal_reached": "true when the person would reach the series' goal.",
			"messages":     "The messages that would go out, in order.",
			"reason":       "Why the person would not enter, or why the journey ended. Empty when nothing to say.",
			"steps":        "Each step the person would reach, in order, with the path it took.",
		},
		"SeriesSimulationDecision": {
			"before": "The index in steps of the step this was asked before.",
			"did":    "true when the person did it.",
			"what":   "What the series asked, in words, such as open the message \"Welcome\".",
		},
		"SeriesSimulationAction": {
			"detail":  "What the change is, as an object.",
			"kind":    "What the step does, such as tag or set_attribute.",
			"node_id": "Id of the step.",
		},
		"SeriesSimulationMessage": {
			"body":    "The message as written, variables unfilled.",
			"kind":    "How it would go out: email, chat, post or slack.",
			"node_id": "Id of the step that sends it.",
			"subject": "The email subject. Empty for other kinds.",
			"to":      "Who would receive it.",
		},
		"SeriesSimulationStep": {
			"detail":  "A few facts the step kept, as an object.",
			"kind":    "What the step does, such as wait, condition or ai_path.",
			"node_id": "Id of the step.",
			"path":    "The path the step took. Empty when it has one way on.",
			"result":  "What the step did, as a short word.",
			"skipped": "true when the step was a wait the simulation skipped.",
		},
		"SimulateSeriesPerson": {
			"attributes": "The made-up person's attributes, by key.",
			"email":      "The made-up person's email address.",
			"name":       "The made-up person's name.",
		},
	}
}
