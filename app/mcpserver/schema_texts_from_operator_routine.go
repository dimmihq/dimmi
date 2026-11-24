/*
|--------------------------------------------------------------------------
| Written Schema Texts: OperatorRoutine To Option
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromOperatorRoutine returns the texts from OperatorRoutine on.
*/
func schemaTextsFromOperatorRoutine() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorRoutine": {
			"cadence": "How often it runs: daily, weekly, monthly, cron (its own schedule) or once.",
			"consecutive_failures": "How many runs in a row have failed. Reaching " +
				"max_consecutive_failures pauses the routine.",
			"created_by": "Id of the teammate who created it (starts with mem_). Null when they are " +
				"gone.",
			"cron_expression": "Its schedule as a cron expression, read in its timezone, such as \"0 9 " +
				"* * 1\". Null only for a routine that runs once.",
			"description": "Short note on what the routine is for, up to 500 characters. May be empty.",
			"inbox_ids": "Ids of the inboxes it is pointed at (each starts with inbox_). Empty when it " +
				"names none.",
			"last_outcome": "How its last run ended: findings (it suggested changes), noop (nothing to " +
				"change), failed or skipped. Null before the first run.",
			"last_run_at":              "When it last ran. Null before the first run.",
			"last_success_at":          "When it last ran without failing. Null when it never has.",
			"max_consecutive_failures": "How many failed runs in a row pause it, from 1 to 10.",
			"month_spend_micros": "What its runs cost this month, in micros, millionths of the billing " +
				"currency unit.",
			"name":        "Name of the routine, up to 120 characters.",
			"next_run_at": "When it runs next. Null when no run is scheduled.",
			"next_runs":   "Its next three runs, soonest first. Empty when none are due.",
			"notify_policy": "When it reports a run: on_findings (only when it suggested something), " +
				"always or on_error_only.",
			"owner_id": "Id of the teammate it runs as, with their access narrowed (starts with mem_). " +
				"Null when they are gone.",
			"pause_reason": "Why it paused, such as owner_permissions_lost or consecutive_failures " +
				"when it paused itself. Null when not paused.",
			"prompt": "The instructions Operator follows on every run, up to 8000 characters.",
			"run_at": "The one moment it runs. Set only when cadence is once; null otherwise.",
			"state": "What it kept from earlier runs, a small JSON object it writes itself, such as " +
				"{\"next_offset\": 15}. An empty object when it kept nothing.",
			"status": "Whether it runs: active, paused, or completed for a one-time routine that has " +
				"run.",
		},
	}
}

/*
schemaTextsFromOperatorRoutineAndOperatorRoutineCounts returns the texts
from OperatorRoutine on.
*/
func schemaTextsFromOperatorRoutineAndOperatorRoutineCounts() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorRoutine": {
			"template_key": "Key of the template it was started from, such as " +
				"tpl_knowledge_gap_finder. Null when it has none.",
			"timezone": "IANA time zone its schedule is read in, such as Europe/Paris. Defaults to " +
				"UTC.",
		},
		"OperatorRoutineCounts": {
			"applied":  "How many of the routine's suggestions were applied.",
			"declined": "How many of the routine's suggestions a teammate declined.",
			"failed":   "How many of the routine's suggestions failed when applied.",
			"pending":  "How many of the routine's suggestions still wait for review.",
			"reverted": "How many of the routine's suggestions were applied and then undone.",
		},
		"OperatorRoutineDelivery": {
			"email_member_ids": "Ids of the teammates emailed about its runs (each starts with mem_). " +
				"Empty for none.",
			"in_app_member_ids": "Ids of the teammates told about its runs in the app (each starts " +
				"with mem_). Empty for none.",
			"slack_destination_ids": "Ids of the Slack destinations its runs are posted to (each " +
				"starts with ndst_). Empty for none.",
		},
		"OperatorRoutineGitHubScope": {
			"base_branch": "The branch it reads pull requests against, such as main. Null when none is " +
				"set.",
			"ignore_labels":   "Pull request labels it skips. Empty for none.",
			"installation_id": "Id of the GitHub connection it reads through (starts with itg_).",
			"repositories":    "The repositories it reads. Empty when none are named.",
		},
		"OperatorRoutineRun": {
			"ai_run_id": "Id of the AI run behind it (starts with air_), for its steps and cost. Null " +
				"when it never started one.",
			"changesets_count": "How many changesets the run suggested.",
			"completed_at":     "When the run ended. Null while queued or running.",
			"cost_micros":      "What the run cost, in micros, millionths of the billing currency unit.",
		},
	}
}

/*
schemaTextsFromOperatorRoutineRun returns the texts from OperatorRoutineRun on.
*/
func schemaTextsFromOperatorRoutineRun() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorRoutineRun": {
			"delivery_results": "How telling people about the run went, as a JSON array with one entry " +
				"per place it reported to. Empty when it told nobody.",
			"error_code":    "Code of what went wrong. Null unless the run failed.",
			"error_message": "What went wrong, in words. Null unless the run failed.",
			"noop_reason": "Why it found nothing to change, in Operator's words. Null unless the " +
				"outcome is noop.",
			"notified": "True when the run was reported to the people the routine tells.",
			"outcome": "How it ended: findings (it suggested changes), noop (nothing to change), " +
				"failed or skipped. Null until it ends.",
			"routine_id":    "Id of the routine this is a run of (starts with ort_).",
			"scheduled_for": "The scheduled moment this run was for. Null for a manual or dry run.",
			"skip_reason": "Why it was skipped, such as overlap_active_run when the previous run was " +
				"still going. Null unless skipped.",
			"started_at": "When it began working. Null while queued or when skipped.",
			"status":     "Where it stands: queued, running, completed, failed or skipped.",
			"steps": "The tool calls it has finished so far, in order, as a JSON array that grows " +
				"while it runs; each has tool, summary and ok.",
			"thread_id": "Id of the shared Operator thread it wrote on (starts with opt_); read it for " +
				"the full run. Null when there is none.",
			"trigger": "What started it: scheduled, manual (a teammate ran it now) or dry_run (a " +
				"preview that applies and saves nothing).",
			"triggered_by": "Id of the teammate who started it (starts with mem_). Null for a " +
				"scheduled run.",
		},
		"OperatorRoutineScope": {
			"help_center_ids": "Ids of the help centers it is pointed at (each starts with hc_). Empty " +
				"when it names none.",
		},
		"OperatorRoutineState": {
			"routine_id": "Id of the routine the state belongs to (starts with ort_).",
			"saved":      "True when the state was kept. False for a dry run, which keeps nothing.",
			"state": "What the routine kept from earlier runs, a small JSON object such as " +
				"{\"next_offset\": 15}.",
		},
	}
}

/*
schemaTextsFromOperatorRoutineTemplate returns the texts from
OperatorRoutineTemplate on.
*/
func schemaTextsFromOperatorRoutineTemplate() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorRoutineTemplate": {
			"cadence": "How often a routine from this template runs by default: daily, weekly, " +
				"monthly, cron or once.",
			"cron_expression": "The template's default schedule as a cron expression.",
			"description":     "A sentence on what the template's routine does.",
			"key": "The template's key, such as tpl_knowledge_gap_finder; send it as template_key when " +
				"creating a routine from it.",
			"name": "Name of the template.",
			"notify_policy": "When its routine reports a run by default: on_findings, always or " +
				"on_error_only.",
			"prompt": "The instructions the template fills in for Operator to follow.",
			"requires": "What the workspace must have for it to work: any of github, help_center, " +
				"conversations and procedures. Empty for none.",
		},
		"OperatorSettings": {
			"auto_groups": "The policy groups that may be set to auto, so Operator applies their " +
				"changes on its own.",
			"brief_on_open": "True when Operator writes a brief of a conversation when a teammate " +
				"opens it.",
			"enabled": "True when Operator is turned on for the workspace.",
			"instructions": "The workspace's own standing instructions to Operator, up to 8000 " +
				"characters. May be empty.",
			"model": "Id of the model Operator runs on. Null to use the default.",
			"policies": "Policy per group of changes (conversations, notes, contacts, articles, " +
				"procedures, actions): auto to apply on its own, ask to propose for review, off to never.",
			"triage_on_arrival": "True when Operator triages each new conversation as it arrives.",
		},
		"OperatorSkill": {
			"skill_name": "Name of the skill; use it to read the skill with its steps.",
			"steps": "The skill's full steps to follow. Left out when skills are listed; read one " +
				"skill to get them.",
			"when_to_use": "A sentence saying which kind of routine work the skill is for.",
		},
	}
}

/*
schemaTextsFromOperatorSuggestionOutcome returns the texts from
OperatorSuggestionOutcome on.
*/
func schemaTextsFromOperatorSuggestionOutcome() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorSuggestionOutcome": {
			"made": "True when the suggestion was made. False when nothing was, as when the same " +
				"suggestion was made before.",
			"message": "A sentence saying what happened to the suggestion, to read back.",
		},
		"OperatorThread": {
			"conversation_id": "Id of the conversation the thread is about (starts with conv_). Null " +
				"when it is not tied to one.",
			"last_message_at": "When the last message was written on the thread. Null when it has " +
				"none.",
			"routine_id": "Id of the routine whose run this shared thread belongs to (starts with " +
				"ort_). Null for a teammate's own thread.",
			"routine_run_id": "Id of the routine run this shared thread belongs to (starts with orr_). " +
				"Null for a teammate's own thread.",
			"title": "Title of the thread, up to 200 characters. May be empty.",
		},
		"OperatorThreadView": {
			"changesets": "The changes Operator proposed in this thread, each a changeset to review.",
			"conversation_id": "Id of the conversation the thread is about (starts with conv_). Null " +
				"when it is not tied to one.",
			"last_message_at": "When the last message was written on the thread. Null when it has " +
				"none.",
			"messages": "The thread's turns, oldest first: the teammate's questions and Operator's " +
				"answers.",
			"routine_id": "Id of the routine whose run this shared thread belongs to (starts with " +
				"ort_). Null for a teammate's own thread.",
			"routine_run_id": "Id of the routine run this shared thread belongs to (starts with orr_). " +
				"Null for a teammate's own thread.",
			"title": "Title of the thread, up to 200 characters. May be empty.",
		},
		"OperatorUsage": {
			"agent_cost_micros": "The whole AI agent's spend that month, the customer-facing agent's " +
				"and Operator's together, which the spend cap holds, in micros, millionths of the billing " +
				"currency unit.",
			"by_member": "Operator's runs and cost that month for each teammate.",
			"by_model":  "Operator's cost and calls that month for each model.",
		},
	}
}

/*
schemaTextsFromOperatorUsage returns the texts from OperatorUsage on.
*/
func schemaTextsFromOperatorUsage() map[string]map[string]string {
	//nolint:gosec // words about secrets, never a secret
	return map[string]map[string]string{
		"OperatorUsage": {
			"by_purpose": "Operator's runs and cost that month by why they ran: ask, brief, triage or " +
				"routine.",
			"by_routine": "Operator's runs and cost that month for each routine.",
			"changesets": "How many of the month's changesets ended in each status.",
			"cost_micros": "What Operator cost that month, in micros, millionths of the billing " +
				"currency unit.",
			"daily":        "Operator's cost and runs for each day of the month.",
			"failed_count": "How many of Operator's runs that month failed.",
			"input_tokens": "Tokens Operator sent to models that month.",
			"monthly_spend_cap_micros": "The AI agent's monthly spend cap, in micros, millionths of " +
				"the billing currency unit. Null when no cap is set.",
			"output_tokens": "Tokens models wrote back for Operator that month.",
			"period_start": "First day of the month these numbers cover, as YYYY-MM-DD, such as " +
				"2026-11-01.",
			"recent": "Operator's latest runs, each with why it ran, for whom, how it ended and what " +
				"it cost.",
			"runs_count": "How many times Operator ran that month.",
		},
		"OperatorUsageChangesets": {
			"count":  "How many changesets ended in this status that month.",
			"status": "The changeset status counted: pending, applied, declined, reverted or failed.",
		},
		"OperatorUsageMember": {
			"cost_micros": "What Operator's runs for this teammate cost that month, in micros, " +
				"millionths of the billing currency unit.",
			"member_id":  "Id of the teammate (starts with mem_).",
			"name":       "The teammate's name.",
			"runs_count": "How many times Operator ran for this teammate that month.",
		},
		"OperatorUsagePurpose": {
			"cost_micros": "What the runs for this purpose cost that month, in micros, millionths of " +
				"the billing currency unit.",
			"purpose": "Why the runs happened: ask (a teammate asked), brief, triage or routine.",
		},
	}
}

/*
schemaTextsFromOperatorUsagePurpose returns the texts from
OperatorUsagePurpose on.
*/
func schemaTextsFromOperatorUsagePurpose() map[string]map[string]string {
	return map[string]map[string]string{
		"OperatorUsagePurpose": {
			"runs_count": "How many runs happened for this purpose that month.",
		},
		"OperatorUsageRoutine": {
			"cost_micros": "What this routine's runs cost that month, in micros, millionths of the " +
				"billing currency unit.",
			"name":       "Name of the routine.",
			"routine_id": "Id of the routine (starts with ort_).",
			"runs_count": "How many times this routine ran that month.",
		},
		"OperatorUsageRun": {
			"conversation_id": "Id of the conversation the run was about (starts with conv_). Null " +
				"when it was not about one.",
			"conversation_number": "Number of the conversation the run was about. Null when it was not " +
				"about one.",
			"cost_micros": "What the run cost, in micros, millionths of the billing currency unit.",
			"latency_ms":  "How long the run took, in milliseconds. Null when not measured.",
			"member_name": "Name of the teammate the run was for. Empty when there is none, as for a " +
				"routine.",
			"purpose": "Why it ran: ask, brief, triage or routine.",
			"routine_id": "Id of the routine it ran for (starts with ort_). Null when it was not a " +
				"routine's run.",
			"status": "How the run ended: running, completed, failed or skipped.",
			"thread_id": "Id of the Operator thread it wrote on (starts with opt_). Null when there is " +
				"none.",
			"thread_title": "Title of the Operator thread it wrote on. Empty when there is none.",
		},
		"Option": {
			"default": "True for the option to preselect, such as a repository's default branch. Left " +
				"out when false.",
			"detail": "A short detail shown beside the name. Left out when there is none.",
			"label":  "The name shown for the option.",
			"value":  "The value to store in the setting when this option is picked.",
		},
	}
}
