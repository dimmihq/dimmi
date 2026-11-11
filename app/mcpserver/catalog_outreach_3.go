/*
|--------------------------------------------------------------------------
| Tool Catalog: Series
|--------------------------------------------------------------------------
|
| The entries for building series, the automated journeys people or
| accounts travel step by step. A series is built as a working copy
| that saves freely and changes nothing customers get until published.
|
*/

package mcpserver

/*
seriesDocs returns the entries for listing, reading and starting
series.
*/
func seriesDocs() map[string]Doc {
	return map[string]Doc{
		"series.index": {
			Title:   "List series",
			Use:     "Lists the workspace's series, most recently changed first, by status or name.",
			Returns: "a page of series with their subject, status and counts.",
			Params: map[string]string{
				"status": "Only series in this status: draft, live, paused or archived.",
				"offset": "How many series to skip, for the next page.",
			},
		},
		"series.show": {
			Title: "Get a series",
			Use: "Reads one series with its working copy: who enters, what makes them leave, the goal and the " +
				"graph of steps, plus its settings and lock_version.",
			Returns: "the series with its spec, settings, status and lock_version.",
			Next: []string{
				"dimmi_series_update: change the working copy",
				"dimmi_series_report_get: how it performs",
			},
		},
		"series.store": {
			Title: "Create a series",
			Use: "Starts a draft series for contacts or for companies, empty or from a whole definition. " +
				"Nothing runs until it is published.",
			Returns: "the draft series.",
			Next: []string{
				"dimmi_series_update: build its steps",
				"dimmi_series_check_create: see what it still misses",
			},
			Params: map[string]string{
				"name":    "A name for the team, up to 120 characters and unlike the other series.",
				"subject": "contact for people, company for an account series. It never changes later.",
				"spec":    seriesSpec(),
			},
		},
		"series.duplicate.store": {
			Title:   "Duplicate a series",
			Use:     "Copies a series, in any status, into a new draft.",
			Returns: "the new draft series.",
		},
		"series.destroy": {
			Title:   "Delete a series draft",
			Use:     "Deletes a draft series that never went live. One that went live is archived instead.",
			Returns: "nothing once it is deleted.",
			Risk:    new(RiskWrite),
			Hint:    "series_not_draft means it went live once: archive it with dimmi_series_archive_create.",
		},
	}
}

/*
seriesDraftDocs returns the entries for saving, checking and restoring
a series' working copy.
*/
func seriesDraftDocs() map[string]Doc {
	return map[string]Doc{
		"series.update": {
			Title: "Change a series",
			Use: "Saves the working copy and the series' settings. What runs does not change until it is " +
				"published. Every field but spec is saved as sent, so send them all as read; spec left out " +
				"keeps the working copy.",
			Returns: "the series as it now is, with its new lock_version.",
			Next:    []string{"dimmi_series_publications_create: make the change live"},
			Params:  seriesParams(),
			Hint:    "edit_conflict means someone saved since you read it: read it again and redo the change.",
		},
		"series.check.store": {
			Title: "Check a series",
			Use: "Lists what a series misses before it can go live, step by step: a loop, a path to nowhere, " +
				"a message without a body, a rule that does not parse. Nothing is saved.",
			Returns: "the problems, each with the step it belongs to.",
			Risk:    new(RiskRead),
			Params:  map[string]string{"spec": "A definition to check instead of the saved working copy."},
		},
		"series.versions.index": {
			Title:   "List a series' versions",
			Use:     "Lists every published version of a series, newest first.",
			Returns: "the versions with their number, note, who published them and when.",
			Next:    []string{"dimmi_series_versions_restore_create: copy an old version back"},
		},
		"series.versions.restore.store": {
			Title: "Restore a series version",
			Use: "Copies an old version into the working copy. What runs does not change until the series is " +
				"published again.",
			Returns: "the series with the restored working copy.",
			Params: map[string]string{
				"version": "Version (srsv_...) to restore. Find it with dimmi_series_versions_list.",
			},
		},
	}
}

/*
seriesSpec explains a series definition.
*/
func seriesSpec() string {
	return "The definition as JSON, in the shape dimmi_series_get returns: entry (audience_id, rules or " +
		"event, include_existing, reentry), exit, goal and the graph of nodes (id, kind, settings) joined " +
		"by edges (from, path, to). Kinds are entry, email, chat, post, slack, wait, wait_for, condition, " +
		"split, ai_path, tag, untag, set_attribute, topic, webhook and exit. Step ids never change."
}

/*
seriesParams explains the fields a series is saved with.
*/
func seriesParams() map[string]string {
	return map[string]string{
		"lock_version":     "The lock_version you read, so a save over someone else's is refused.",
		"name":             "A name for the team, up to 120 characters.",
		"description":      "What the series is for, up to 500 characters.",
		"spec":             seriesSpec(),
		"topic_id":         "Topic (bctp_...) its messages go under. Find it with dimmi_broadcast_topics_list.",
		"service":          "true marks its messages as service notices that reach people who left topics.",
		"email_address_id": "Sending email address (ema_...). Find it with dimmi_email_addresses_list.",
		"sender_id":        "Teammate (mem_...) its messages come from.",
		"slack_as":         "bot posts as the dimmi app, member as the sender's own Slack account.",
		"reply_team_id":    "Team (team_...) that gets the replies.",
		"reply_member_id":  "Teammate (mem_...) who gets the replies.",
		"ai_replies":       "true lets the AI agent answer replies.",
	}
}
