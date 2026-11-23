/*
|--------------------------------------------------------------------------
| Written Schema Texts: RatingFace To SLAPolicy
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTexts106 returns the texts from RatingFace on.
*/
func schemaTexts106() map[string]map[string]string {
	return map[string]map[string]string{
		"RatingFace": {
			"face":  "The emoji shown for this score, such as 😍 for 5.",
			"label": "The word shown for this score: Terrible, Bad, OK, Good or Amazing.",
			"score": "The score the customer gives by picking this face, from 1 (worst) to 5 (best).",
		},
		"Reaction": {
			"count": "How many teammates reacted to the note with this emoji.",
			"emoji": "The emoji teammates reacted with, such as 👍.",
			"member_ids": "Ids of the teammates who reacted with it, in the order they did (each " +
				"starts with mem_).",
		},
		"RedactionRule": {
			"enabled":  "True when the rule is applied to messages before they are stored.",
			"name":     "Name of the rule.",
			"pattern":  "The regular expression whose matches are replaced, up to 256 characters.",
			"position": "Where the rule comes in the order rules apply, lowest first.",
			"replacement": "What each match becomes in the stored text, up to 80 characters, such as " +
				"[redacted].",
		},
		"RedactionRules": {
			"rules": "The workspace's own redaction rules, in the order they apply. Empty when it has " +
				"none.",
		},
		"RedactionTest": {
			"redactions": "Each rule that changed the tried text, named by its id (starts with rxr_) " +
				"for a workspace rule, and how many matches it replaced. Empty when none did.",
			"text": "The tried text as it would be stored, with every match replaced.",
		},
		"Remover": {
			"name": "Name of the teammate who deleted the entry.",
		},
	}
}

/*
schemaTexts107 returns the texts from Report on.
*/
func schemaTexts107() map[string]map[string]string {
	return map[string]map[string]string{
		"Report": {
			"errors": "Problems in the workflow's definition that stop it from being published, each " +
				"with a code, a message and the step, path and field it is about.",
			"warnings": "Issues that do not stop a publish but must be acknowledged by the publisher, " +
				"each with a code, a message and the step it is about.",
		},
		"ReportChart": {
			"axis": "What the chart runs along: day (dates as YYYY-MM-DD), hour (hours of the week) or " +
				"category (groups).",
			"columns": "The columns of a table chart, each with its key, label and unit. Empty for " +
				"other kinds.",
			"drill": "Key of the metric whose conversations the chart's points list when drilled into, " +
				"such as first_response or ai_handoff. Null when it cannot be drilled.",
			"key":  "The chart's key within its report, such as ai_daily or handoff_reasons.",
			"kind": "How to draw it: line, bar, stacked_bar, heatmap, histogram or table.",
			"rows": "The rows of a table chart, each with its values under the columns. Empty for " +
				"other kinds.",
			"series": "The chart's lines or sets of bars, each with its points; a point's value is " +
				"null for no data. Empty for a table.",
			"title": "Title of the chart, such as \"What the AI did\".",
			"unit": "Unit of the chart's values: count, percent, seconds, micros (millionths of the " +
				"billing currency unit) or score.",
		},
		"ReportColumn": {
			"key":   "The column's key; a row's values are found under it, such as median.",
			"label": "The column's heading, such as \"Median\".",
			"unit":  "Unit of the column's values: count, percent, seconds, micros or score.",
		},
		"ReportConversation": {
			"assignee_id": "Id of the teammate the conversation is assigned to, such as mem_...; null " +
				"when nobody is assigned.",
			"channel": "Channel the conversation came through: widget, email, api, slack, whatsapp, " +
				"facebook or instagram.",
			"closed_at": "When the conversation was closed; null if it has not been.",
		},
	}
}

/*
schemaTexts108 returns the texts from ReportConversation on.
*/
func schemaTexts108() map[string]map[string]string {
	return map[string]map[string]string{
		"ReportConversation": {
			"number":     "The conversation's number within the workspace, the one teammates see.",
			"started_at": "When the conversation started.",
			"status":     "The conversation's status: open, snoozed or closed.",
			"subject":    "The conversation's subject line; null when it has none.",
			"value": "What this conversation added to the number: seconds for a duration, the score " +
				"for a rating, the AI's cost in millionths of a dollar; null for a plain count.",
		},
		"ReportConversationsOutputBody": {
			"capped": "True when more conversations match than the list counts, so total stops at the " +
				"cap of 10000.",
			"data":   "The page of conversations behind the number.",
			"limit":  "Page size used for this page, at most 100.",
			"offset": "How many conversations were skipped before this page.",
			"total":  "How many conversations are behind the number, counted up to the cap of 10000.",
		},
		"ReportDigest": {
			"business_hours": "True when the digest counts durations in business hours only.",
			"channels": "Channels the digest is narrowed to, such as email or widget; empty for every " +
				"channel.",
			"frequency": "How often the digest is sent: daily or weekly. Null when the caller has no " +
				"digest.",
			"inbox_ids": "Ids of the inboxes the digest is narrowed to, such as inbox_...; empty for " +
				"every inbox.",
			"last_sent_at": "When the digest was last sent; null if it never was or there is none.",
			"next_send_at": "When the digest goes out next; null when the caller has no digest.",
			"team_ids": "Ids of the teams the digest is narrowed to, such as team_...; empty for every " +
				"team.",
		},
		"ReportMetric": {
			"drill": "Name of the metric to pass to the report's conversations list to see the " +
				"conversations behind this number; null when it cannot be listed.",
			"key": "Stable name of the number within its report.",
		},
	}
}

/*
schemaTexts109 returns the texts from ReportMetric on.
*/
func schemaTexts109() map[string]map[string]string {
	return map[string]map[string]string{
		"ReportMetric": {
			"label": "Name of the number as people read it.",
			"mean": "Average, in seconds, shown beside a duration; null for other numbers or with no " +
				"data.",
			"p90": "90th percentile, in seconds, shown beside a duration; null for other numbers or " +
				"with no data.",
			"previous": "The same number in the period compared with; null without a comparison or " +
				"with no data.",
			"sample": "How many conversations the number rests on.",
			"unit": "What value measures: count, percent, seconds, micros (millionths of a dollar) or " +
				"score.",
			"value": "The number for the range asked for, in its unit; for a duration it is the " +
				"median. Null when there is no data.",
		},
		"ReportPoint": {
			"x": "Where the point sits: a day such as 2026-01-31, a two-digit hour such as 09, or a " +
				"group's name, depending on the chart's axis.",
			"y": "The value at that point, in the chart's unit; null for no data.",
		},
		"ReportResult": {
			"business_hours": "True when durations in the report are counted in business hours only.",
			"charts":         "The report's charts, each read as series of points or as a table of rows.",
			"from":           "First day of the range, such as 2026-01-01, in the report's timezone.",
			"key": "Name of the report, such as overview, responsiveness, resolution, team, live, " +
				"satisfaction, ai or help_center.",
			"live":    "True when the report shows the present moment instead of a date range.",
			"metrics": "The report's headline numbers.",
			"previous_from": "First day of the range compared with, such as 2025-12-01; null without a " +
				"comparison.",
			"previous_to": "Last day, included, of the range compared with; null without a comparison.",
			"timezone":    "Timezone the days of the report are counted in, such as Europe/Paris.",
			"title":       "Name of the report as people read it, such as Overview.",
			"to":          "Last day of the range, included, such as 2026-01-31.",
		},
	}
}

/*
schemaTexts110 returns the texts from ReportRow on.
*/
func schemaTexts110() map[string]map[string]string {
	return map[string]map[string]string{
		"ReportRow": {
			"filter": "Query filter that narrows the report to this row, such as {\"member\": " +
				"\"mem_...\"}; empty when the row cannot be narrowed to.",
			"key":   "Stable name of the row within its table.",
			"label": "Name of the row as people read it, such as a teammate's name.",
			"values": "The row's values in the order of the table's columns, each in its column's " +
				"unit; null where there is no data.",
		},
		"ReportSeries": {
			"drill": "Name of the metric to pass to the report's conversations list for this series; " +
				"null to use the chart's own or when it cannot be listed.",
			"key":    "Stable name of the series within its chart.",
			"label":  "Name of the series as people read it.",
			"points": "The series' values, one per day, hour or group along the chart's axis.",
		},
		"ReportType": {
			"key": "Name of the report to ask for it by, such as overview, responsiveness, resolution, " +
				"team, live, satisfaction, ai or help_center.",
			"live":    "True when the report shows the present moment instead of a date range.",
			"summary": "One line saying what the report is about.",
			"title":   "Name of the report as people read it, such as Overview.",
		},
		"ReportTypesOutputBody": {
			"data": "The reports the caller may read.",
		},
		"ResponseError": {
			"code": "Stable code for programs to tell errors apart, such as validation_failed or " +
				"internal_error.",
			"errors": "Each rejected field mapped to its messages. Only present when the input was " +
				"invalid.",
			"message": "What went wrong, in words safe to show a person.",
		},
		"Retention": {
			"changed_at": "When the retention periods were last changed; left out if they never were.",
		},
	}
}

/*
schemaTexts111 returns the texts from Retention on.
*/
func schemaTexts111() map[string]map[string]string {
	return map[string]map[string]string{
		"Retention": {
			"closed_conversation_days": "Days a closed conversation is kept; null when closed " +
				"conversations are kept for ever.",
			"export_days": "Days a data export is kept; 7 unless the workspace chose otherwise.",
			"raw_email_days": "Days raw email is kept after it is read; 30 unless the workspace chose " +
				"otherwise.",
			"spam_days":  "Days spam is kept; 30 unless the workspace chose otherwise.",
			"trash_days": "Days the trash is kept; 30 unless the workspace chose otherwise.",
		},
		"RetentionPreview": {
			"count": "How many closed conversations the proposed retention period would move out.",
		},
		"Role": {
			"base_role": "Built-in role this one is or narrows: owner, admin or member.",
			"description": "What the custom role is for; null for a built-in role or when none was " +
				"written.",
			"kind": "built_in for a role dimmi ships, custom for one the workspace made. A built-in " +
				"role has no id; its name identifies it.",
			"member_count": "How many members hold the custom role; null for a built-in role and for " +
				"callers who do not manage roles.",
			"name":        "The role's name.",
			"permissions": "Keys of the permissions the role holds, such as conversations:read.",
		},
		"RoleSummary": {
			"name": "Name of the custom role the member holds; may be empty when the response did not " +
				"fill it in.",
		},
		"RowError": {
			"message": "Why the row could not be imported, in plain words.",
			"row":     "Line of the file the failed row is on.",
		},
		"SLAClock": {
			"breached_at": "When the clock passed its due time unmet; null if it has not.",
		},
	}
}

/*
schemaTexts112 returns the texts from SLAClock on.
*/
func schemaTexts112() map[string]map[string]string {
	return map[string]map[string]string{
		"SLAClock": {
			"conversation_id": "Id of the conversation the clock runs on, such as conv_...; null when " +
				"it runs on a ticket.",
			"due_at":     "When the target must be met, counting only the policy's open hours.",
			"paused_at":  "When the clock was paused; null while it is not paused.",
			"policy_id":  "Id of the service level policy the clock follows, such as sla_....",
			"started_at": "When the clock started.",
			"state":      "Where the clock stands: running, paused, met, breached or canceled.",
			"stopped_at": "When the clock ended; null while it still runs or is paused.",
			"target":     "What the clock promises: first_reply, next_reply, follow_up or resolution.",
			"ticket_id": "Id of the ticket the clock runs on, such as tkt_...; null when it runs on a " +
				"plain conversation.",
			"warn_at": "When the team is warned ahead of the due time; null when no warning is set.",
		},
		"SLAPolicy": {
			"archived_at": "When the policy was archived; null while it is in use.",
			"breach_priority": "Priority a breach raises the work to: none, low, medium, high or " +
				"urgent; null to leave the priority alone.",
			"breach_team_id": "Id of the team a breach moves the work to, such as team_...; null to " +
				"leave it where it is.",
			"description": "What the policy is for; empty when none was written.",
			"first_reply_seconds": "Seconds allowed for the first reply, counted in the schedule's " +
				"open hours; null when the policy sets no such target.",
			"follow_up_seconds": "Seconds allowed for a follow-up when the team spoke last; null when " +
				"the policy sets no such target.",
			"name": "The policy's name.",
			"next_reply_seconds": "Seconds allowed for every reply after the first; null when the " +
				"policy sets no such target.",
			"pause_on_waiting_customer": "True when the policy's clocks pause while the work waits on " +
				"the customer.",
			"pause_on_waiting_team": "True when the policy's clocks pause while the work waits on " +
				"another team.",
			"position": "The policy's place in the order policies are tried; the first one matching a " +
				"new ticket's type and priority applies.",
			"priorities": "Priorities of new tickets the policy applies to: none, low, medium, high or " +
				"urgent. Empty when it names none.",
		},
	}
}
