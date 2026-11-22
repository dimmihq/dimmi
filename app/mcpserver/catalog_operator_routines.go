/*
|--------------------------------------------------------------------------
| Tool Catalog: Operator Routines
|--------------------------------------------------------------------------
|
| The entries for routines: prompts Operator runs on a clock, as a
| narrowed copy of their owner, leaving suggestions the team reviews.
|
*/

package mcpserver

/*
operatorRoutineDocs returns the entries for Operator's routines.
*/
func operatorRoutineDocs() map[string]Doc {
	return map[string]Doc{
		"operator.routine_templates.index": {
			Title:   "List routine templates",
			Use:     "Lists the templates a routine can start from, with their prompt, cadence and what they need.",
			Returns: "the templates, the custom one last.",
		},
		"operator.routines.index": {
			Title:   "List Operator routines",
			Use:     "Lists the workspace's routines: prompts Operator runs on a clock and reports on.",
			Returns: "the routines with their cadence, next run, status and last outcome.",
			Params:  map[string]string{"filter[status]": "active, paused or completed."},
		},
		"operator.routines.show": {
			Title: "Get an Operator routine",
			Use: "Reads one routine: its prompt, when it runs, what it may read and change, and this " +
				"month's spend.",
			Returns: "the routine with its next three runs and its spend this month in micro-dollars.",
		},
		"operator.routines.store": {
			Title: "Create an Operator routine",
			Use: "Puts Operator on a clock: a prompt it runs daily, weekly, monthly, on a cron or once, " +
				"leaving suggestions the team reviews. Every run spends the workspace's AI budget. Confirm the " +
				"prompt and cadence with the person first.",
			Returns: "the routine with its next runs.",
			Next:    []string{"dimmi_operator_routines_runs_create: try it now as a dry run"},
			Risk:    new(RiskDanger),
			Hint:    "A routine runs at most once an hour and is never given more than you hold.",
		},
		"operator.routines.update": {
			Title: "Change an Operator routine",
			Use: "Changes a routine's prompt, cadence, access or delivery, pauses or resumes it, or makes it " +
				"forget what it kept between runs.",
			Returns: "the routine as changed.",
			Risk:    new(RiskDanger),
		},
		"operator.routines.destroy": {
			Title: "Delete an Operator routine",
			Use:   "Stops a routine for good. Its threads and suggestions stay.",
			Risk:  new(RiskDanger),
		},
	}
}

/*
operatorRoutineRunDocs returns the entries for a routine's runs.
*/
func operatorRoutineRunDocs() map[string]Doc {
	return map[string]Doc{
		"operator.routines.runs.store": {
			Title: "Run an Operator routine now",
			Use: "Runs a routine now. A dry run reads real data but leaves only previews and keeps no state; a " +
				"live run leaves real suggestions. Both spend the AI budget.",
			Returns: "the queued run; read it again to see how it went.",
			Next:    []string{"dimmi_operator_routines_runs_get: see how the run went"},
			Risk:    new(RiskWrite),
			Hint:    "A live run is refused while another is going; a dry run never is.",
		},
		"operator.routines.runs.index": {
			Title:   "List a routine's runs",
			Use:     "Lists a routine's runs, newest first.",
			Returns: "the runs with their trigger, outcome, suggestions, cost and thread.",
		},
		"operator.routines.runs.show": {
			Title:   "Get a routine's run",
			Use:     "Reads one run of a routine: how it went, why it was skipped or failed, and its thread.",
			Returns: "the run.",
		},
	}
}
