/*
|--------------------------------------------------------------------------
| Tool Catalog: Testing Procedures
|--------------------------------------------------------------------------
|
| The entries for trying a procedure before customers see it: a preview
| turn, saved simulations an AI-played customer replays, the published
| versions to compare or restore, and how procedures did in real
| conversations.
|
*/

package mcpserver

import "maps"

/*
procedureTestingDocs returns the entries for previewing, simulating,
versions and procedure metrics.
*/
func procedureTestingDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{procedureSimulationDocs(), procedureTestCaseDocs(), procedureVersionDocs()} {
		maps.Copy(out, part)
	}

	return out
}

/*
procedureSimulationDocs returns the entry for a preview turn.
*/
func procedureSimulationDocs() map[string]Doc {
	return map[string]Doc{
		"procedures.simulate.store": {
			Title: "Preview a procedure turn",
			Use: "Sends one customer message to the AI agent as it would follow this procedure, and returns its " +
				"reply. Nothing reaches a customer and actions answer with their saved test result, never your " +
				"systems. Every turn runs real models and spends the workspace's AI budget.",
			Returns: "the agent's reply, the actions it called, the procedure's state to send with the next turn.",
			Params: map[string]string{
				"message":      "What the customer writes, up to 4000 characters.",
				"history":      "The turns so far, oldest first, as returned by earlier calls.",
				"active_state": "The state the previous turn returned, so the agent picks up where it was.",
				"target":       "draft tests the working copy, live the published one.",
			},
		},
	}
}

/*
procedureTestCaseDocs returns the entries for saved simulations.
*/
func procedureTestCaseDocs() map[string]Doc {
	return map[string]Doc{
		"procedures.test-cases.index": {
			Title:   "List a procedure's simulations",
			Use:     "Lists the saved simulations of a procedure and how each last went.",
			Returns: "the simulations with their messages, checks and last result.",
		},
		"procedures.test-cases.show": {
			Title:   "Get a procedure simulation",
			Use:     "Reads one saved simulation of a procedure and its last result.",
			Returns: "the simulation with its messages, checks, date and last result turn by turn.",
		},
		"procedures.test-cases.store": {
			Title: "Save a procedure simulation",
			Use: "Saves a chat as a simulation: an AI-played customer replays it against the procedure, on the " +
				"date it was saved, and the checks say whether the agent did what it should.",
			Returns: "the saved simulation.",
			Next:    []string{"dimmi_run_procedure_test_case: run it"},
			Params: map[string]string{
				"messages":   "The customer's messages, oldest first, at most 12.",
				"assertions": "Checks such as procedure_matched, action_called, handed_off or llm_rubric.",
			},
		},
		"procedures.test-cases.update": {
			Title:   "Edit a procedure simulation",
			Use:     "Changes a saved simulation's name, messages, customer or checks.",
			Returns: "the simulation as saved.",
		},
		"procedures.test-cases.destroy": {
			Title: "Delete a procedure simulation",
			Use:   "Deletes a saved simulation and its last result.",
			Risk:  new(RiskWrite),
		},
		"procedures.test-cases.run.store": {
			Title: "Run procedure simulations",
			Use: "Runs a procedure's saved simulations against its working copy and waits for the verdicts. " +
				"Each run calls real models for the agent, the customer and the checks, and spends the " +
				"workspace's AI budget. A working copy with problems to fix is not run.",
			Returns: "each simulation with its result: passed, failed or an error, and every check's verdict.",
			Params:  map[string]string{"test_case_ids": "Only these simulations; leave out to run them all."},
		},
	}
}

/*
procedureVersionDocs returns the entries for published versions and
procedure metrics.
*/
func procedureVersionDocs() map[string]Doc {
	return map[string]Doc{
		"procedures.versions.index": {
			Title:   "List a procedure's versions",
			Use:     "Lists every published version of a procedure, newest first, with who published it and why.",
			Returns: "the versions with their number, note and publish time.",
			Params:  map[string]string{"before": "Version number to page from.", "limit": "At most 100."},
		},
		"procedures.versions.show": {
			Title:   "Get a procedure version",
			Use:     "Reads one published version of a procedure as it was.",
			Returns: "the version with its full definition.",
		},
		"procedures.versions.restore.store": {
			Title: "Restore a procedure version",
			Use: "Copies a published version into the working copy. Customers see no change until it is " +
				"published again.",
			Returns: "the procedure with the restored working copy.",
			Next:    []string{"dimmi_publish_procedure: make it live again"},
		},
		"procedures.metrics.show": {
			Title:   "Get a procedure's metrics",
			Use:     "Reads how a procedure did in real conversations: how often it ran, finished and handed over.",
			Returns: "the counts, rates, a daily series and where conversations left the procedure.",
			Params: map[string]string{
				"days":    "How many days back, up to 90.",
				"version": "Only conversations on this published version.",
			},
		},
		"reports.procedures.index": {
			Name:    "report_procedures",
			Title:   "Report on procedures",
			Use:     "Compares every procedure's runs, completions and handovers over a period.",
			Returns: "one row per procedure with its counts and rates.",
			Params:  map[string]string{"from": "First day, YYYY-MM-DD.", "to": "Last day, YYYY-MM-DD."},
		},
		"reports.workflows.index": {
			Name:  "report_workflows",
			Title: "Report on workflows",
			Use: "Compares every workflow that ran over a period: runs started, completed, handed to a person, " +
				"failed, and how many reached the workflow's goal.",
			Returns: "one row per workflow with its counts, the busiest first.",
			Params:  map[string]string{"from": "First day, YYYY-MM-DD.", "to": "Last day, YYYY-MM-DD."},
		},
	}
}
