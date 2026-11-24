/*
|--------------------------------------------------------------------------
| Written Schema Texts: Workflow Analytics And Series Control Groups
|--------------------------------------------------------------------------
|
| What each field of a workflow's numbers, its failed runs, the workflows
| report and a series' control group means, by schema and field.
|
*/

package mcpserver

import "maps"

/*
schemaTextsFromAutomation returns the texts of the automation numbers.
*/
func schemaTextsFromAutomation() map[string]map[string]string {
	figures := map[string]string{
		"completed":   "Runs in the range that reached the end of their path.",
		"failed":      "Runs in the range that failed, or where a step failed.",
		"handed_over": "Runs in the range that handed the conversation to a person.",
		"triggered":   "Runs that started in the range and did something. Test conversations never count.",
		"goal_met": "Runs in the range whose conversation reached the workflow's goal. Null when the " +
			"workflow has no goal.",
	}
	metrics := map[string]string{
		"active":      "Runs in the range still running or waiting.",
		"days":        "Runs started, completed and failed per day of the range.",
		"engaged":     "Runs in the range where the customer wrote while the run went on.",
		"from":        "The first moment the numbers count.",
		"to":          "The moment the numbers stop counting.",
		"steps":       "Each step's numbers, by step id: reached, waiting now, failed, dropped and paths taken.",
		"version":     "The published version the numbers count. Null when they count every version.",
		"workflow_id": "The workflow the numbers are about, such as wf_....",
	}
	maps.Copy(metrics, figures)
	report := reportTexts()
	maps.Copy(report, figures)

	return map[string]map[string]string{
		"WorkflowMetrics":     metrics,
		"WorkflowReportRow":   report,
		"WorkflowMetricsDay":  metricsDayTexts(),
		"WorkflowStepMetrics": stepMetricsTexts(),
		"WorkflowGoal":        goalTexts(),
		"WorkflowVariantMetrics": {
			"reached": "Runs that took the variant.",
			"goal": "Runs that took the variant and whose conversation reached the workflow's goal. Zero when " +
				"the workflow has no goal.",
		},
	}
}

/*
reportTexts returns the texts of a workflows report row besides its
counts.
*/
func reportTexts() map[string]string {
	return map[string]string{
		"name":        "The workflow's name.",
		"status":      "Where the workflow stands: draft, live or paused.",
		"trigger":     "The event that starts the workflow, such as conversation.first_message.",
		"workflow_id": "Id of the workflow, such as wf_....",
		"goal":        "The workflow's goal. Null when it has none.",
	}
}

/*
metricsDayTexts returns the texts of one day of a workflow's numbers.
*/
func metricsDayTexts() map[string]string {
	return map[string]string{
		"day":       "The day, as YYYY-MM-DD in UTC.",
		"triggered": "Runs started that day.",
		"completed": "Runs started that day that completed.",
		"failed":    "Runs started that day that failed.",
	}
}

/*
stepMetricsTexts returns the texts of one step's numbers.
*/
func stepMetricsTexts() map[string]string {
	return map[string]string{
		"reached": "Runs that reached the step.",
		"waiting": "Runs waiting on the step now.",
		"failed":  "Runs where the step failed.",
		"dropped": "Runs that stopped on the step without finishing: exited, canceled, failed or expired.",
		"paths":   "How many runs left by each of the step's paths, by path id.",
		"variants": "For an A/B split, each variant's runs and goal, by variant (path) id. Missing for " +
			"other steps.",
	}
}

/*
goalTexts returns the texts of a workflow's goal.
*/
func goalTexts() map[string]string {
	return map[string]string{
		"kind": "What counts as the workflow working: resolved (closed with no teammate reply or " +
			"handover) or csat (rated at least min_rating).",
		"min_rating": "With csat, the lowest rating that counts, 1 to 5. Null otherwise.",
		"window":     "How long after the run starts the goal may be reached, such as 7d.",
	}
}

/*
schemaTextsFromAutomationFailures returns the texts of a workflow's
failed runs and a series' control group.
*/
func schemaTextsFromAutomationFailures() map[string]map[string]string {
	return map[string]map[string]string{
		"WorkflowFailure": {
			"conversation_id":     "Id of the conversation the run worked on, such as conv_....",
			"conversation_number": "The conversation's number, as the inbox shows it.",
			"detail":              "A few small facts the failing step kept, as an object.",
			"end_reason":          "Why the run ended. Null while it has not.",
			"ended_at":            "When the run ended. Null while it has not.",
			"error":               "What went wrong, as the run recorded it. Null when it recorded nothing.",
			"outcome":             "The failing step's outcome, such as failed.",
			"run_id":              "Id of the run, such as wfr_....",
			"started_at":          "When the run started.",
			"status":              "Where the run stands, such as failed or completed.",
			"step_id":             "Id of the first step that failed.",
			"step_kind":           "What the failing step does, such as http_request or classify.",
			"version_number":      "The published version the run ran on.",
		},
		"SeriesControl": {
			"percent": "The share of entrants held out of the series to measure its lift, 0 to 50.",
			"groups":  "Entrants and goals reached for the control group and for everyone else.",
		},
		"SeriesControlGroup": {
			"group":   "Which group: control (held out, sent nothing) or treated (traveled the series).",
			"entered": "How many entrants were in the group.",
			"goal":    "How many of them reached the series' goal.",
		},
	}
}
