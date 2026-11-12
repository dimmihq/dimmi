/*
|--------------------------------------------------------------------------
| Tool Catalog: Reports
|--------------------------------------------------------------------------
|
| The entries for the reports: which there are, one report's numbers and
| charts for a range and filters, the conversations behind a number, and
| the caller's digest by mail.
|
*/

package mcpserver

import "maps"

/*
reportDocs returns the entries for the reports.
*/
func reportDocs() map[string]Doc {
	out := map[string]Doc{
		"reports.index": {
			Title: "List reports",
			Use: "Lists the reports the caller may read: overview, responsiveness, resolution, team, live " +
				"workload, satisfaction, AI, help center and any others the workspace has.",
			Returns: "each report's key, title and one line about it, and whether it shows the present " +
				"instead of a range.",
			Next: []string{"dimmi_reports_get: read one report"},
			Risk: new(RiskRead),
		},
		"reports.conversations.index": {
			Title: "List the conversations behind a number",
			Use: "Lists the conversations one of a report's numbers counts, with the same range and filters, each " +
				"with the value it added. Stops at ten thousand.",
			Returns: "the conversations with number, subject, status, channel, start, close, assignee and value, " +
				"slowest or newest first, and how many there are.",
			Next: []string{"dimmi_conversations_get: open one conversation"},
			Risk: new(RiskRead),
			Params: map[string]string{
				"metric": "the drill key the number names, such as first_response, resolution, csat or open.",
				"limit":  "how many to return, at most 100.",
				"offset": "how many to skip, for the next page.",
			},
		},
	}
	out["reports.show"] = reportShowDoc()
	maps.Copy(out, reportDigestDocs())

	return out
}

/*
reportShowDoc returns the entry for reading one report.
*/
func reportShowDoc() Doc {
	return Doc{
		Title: "Read a report",
		Use: "Reads one report for a range of days in the workspace's time zone, counting only conversations the " +
			"caller may see. Durations are medians in seconds with the 90th percentile and mean beside them. " +
			"Spam, test, deleted and merged-away conversations never count.",
		Returns: "the headline numbers, each with its unit, value, value in the period before when compare is " +
			"set, sample size and the metric that lists its conversations; and the charts as series or tables.",
		Next: []string{"dimmi_reports_conversations_list: list the conversations behind a number"},
		Risk: new(RiskRead),
		Params: map[string]string{
			"report": "the report's key, such as overview, responsiveness, resolution, team, live, satisfaction, " +
				"ai or help_center.",
			"from":             "the first day, 2026-11-01; with to left out, the thirty days ending today.",
			"to":               "the last day, included.",
			"compare":          "true sets each number beside the same number of days just before.",
			"business_hours":   "true counts durations only while the inbox's office hours were open.",
			"filter[inbox]":    "a comma list of inbox ids.",
			"filter[team]":     "a comma list of team ids.",
			"filter[member]":   "a comma list of teammate ids holding the conversation.",
			"filter[channel]":  "a comma list of channel types, such as email,widget.",
			"filter[tag]":      "a comma list of tag ids.",
			"filter[priority]": "a comma list of priorities.",
			"filter[ai]":       "involved or not_involved: conversations the AI answered in, or not.",
		},
	}
}

/*
reportDigestDocs returns the entries for the caller's report digest.
*/
func reportDigestDocs() map[string]Doc {
	return map[string]Doc{
		"report-digest.show": {
			Title:   "Read my report digest",
			Use:     "Reads whether the caller gets the reports by mail, how often, and what the digest covers.",
			Returns: "the frequency, null when there is none, the filters and when it next goes.",
			Risk:    new(RiskRead),
		},
		"report-digest.update": {
			Title: "Set my report digest",
			Use: "Has the overview mailed to the caller each morning or each Monday, compared with the " +
				"period before.",
			Returns: "the digest as saved, with when it next goes.",
			Risk:    new(RiskWrite),
			Params: map[string]string{
				"frequency": "daily, covering the day before, or weekly, covering the week before.",
			},
		},
		"report-digest.destroy": {
			Title:   "Stop my report digest",
			Use:     "Stops mailing the reports to the caller.",
			Returns: "nothing.",
			Risk:    new(RiskWrite),
		},
	}
}
