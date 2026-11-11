/*
|--------------------------------------------------------------------------
| Tool Catalog: Broadcasts
|--------------------------------------------------------------------------
|
| The entries for broadcasts: one message to many people by email, in
| the Messenger or in Slack. A broadcast is a draft until someone sends
| it, and every step that reaches customers asks the person first.
|
*/

package mcpserver

import "maps"

/*
outreachDocs returns the entries for broadcasts, their topics and
series.
*/
func outreachDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		broadcastDocs(), broadcastDraftDocs(), broadcastReviewDocs(), broadcastSendDocs(), broadcastTopicDocs(),
		seriesDocs(), seriesDraftDocs(), seriesLiveDocs(), seriesPeopleDocs(), journeyDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
broadcastDocs returns the entries for listing, reading and starting
broadcasts.
*/
func broadcastDocs() map[string]Doc {
	return map[string]Doc{
		"broadcasts.index": {
			Title:   "List broadcasts",
			Use:     "Lists the workspace's broadcasts, newest first, by status, channel or name.",
			Returns: "a page of broadcasts with their channel, status, schedule and counts.",
			Params: map[string]string{
				"channel": "Only broadcasts on this channel: email, chat, post or slack.",
				"status":  "Only broadcasts in this status: draft, scheduled, sending, paused, sent or stopped.",
				"offset":  "How many broadcasts to skip, for the next page.",
			},
		},
		"broadcasts.show": {
			Title:   "Get a broadcast",
			Use:     "Reads one broadcast with everything its draft holds and where it stands.",
			Returns: "the broadcast with its audience, content, sender, schedule, goal and status.",
			Next: []string{
				"dimmi_broadcasts_report_get: what it did once sent",
				"dimmi_broadcasts_update: change the draft",
			},
		},
		"broadcasts.store": {
			Title: "Create a broadcast",
			Use: "Starts a draft broadcast on one channel. Nothing reaches anyone until it is sent; the draft " +
				"saves half written.",
			Returns: "the draft broadcast.",
			Next:    []string{"dimmi_broadcasts_update: choose the audience and write the message"},
			Params: map[string]string{
				"name": "A name for the team, up to 120 characters.",
				"channel": "email, chat (a Messenger conversation), post (a card in the Messenger) or slack " +
					"(customers' Slack channels). It never changes later.",
			},
		},
		"broadcasts.duplicate.store": {
			Title:   "Duplicate a broadcast",
			Use:     "Copies any broadcast, sent or not, into a new draft to send again or adapt.",
			Returns: "the new draft.",
		},
		"broadcasts.destroy": {
			Title:   "Delete a broadcast draft",
			Use:     "Deletes a draft that was never sent. A broadcast that started sending cannot be deleted.",
			Returns: "nothing once it is deleted.",
			Risk:    new(RiskWrite),
			Hint:    "broadcast_not_draft means it already started: stop it with dimmi_broadcasts_stop_create.",
		},
	}
}

/*
broadcastDraftDocs returns the entry for changing a draft.
*/
func broadcastDraftDocs() map[string]Doc {
	return map[string]Doc{
		"broadcasts.update": {
			Title: "Change a broadcast draft",
			Use: "Saves fields over a draft: who gets it, the message, who it comes from, the schedule and the " +
				"goal. Only the fields sent change and an empty id clears one. Whether it can be sent is " +
				"checked on send.",
			Returns: "the draft as it now is.",
			Next: []string{
				"dimmi_broadcasts_estimate_get: count who it would reach",
				"dimmi_broadcasts_tests_create: send a test first",
			},
			Params: broadcastParams(),
			Hint:   "broadcast_not_draft means it already started sending and can no longer change.",
		},
	}
}

/*
broadcastParams explains the fields of a broadcast draft.
*/
func broadcastParams() map[string]string {
	return map[string]string{
		"audience_id": "Saved audience (audn_...) that gets it. Find it with dimmi_audiences_list.",
		"rules":       audienceRules() + " Used instead of a saved audience.",
		"room_ids":    "For slack: the Slack channels (chr_...) to post in. Find them with dimmi_channels_rooms_list.",
		"topic_id": "Topic (bctp_...) it is sent under; people who left the topic do not get it. Find it " +
			"with dimmi_broadcast_topics_list.",
		"service": "true marks a service notice, such as an outage, that reaches people who left the " +
			"marketing topics. Never use it for marketing.",
		"subject":          "Email subject, up to 200 characters.",
		"preview":          "Email preview text, up to 200 characters.",
		"body":             "The message in Markdown, up to 50000 characters.",
		"sender_id":        "Teammate (mem_...) it comes from. Find it with dimmi_members_list.",
		"email_address_id": "Sending email address (ema_...). Find it with dimmi_email_addresses_list.",
		"slack_as":         "For slack: bot posts as the dimmi app, member as the sender's own Slack account.",
		"reply_team_id":    "Team (team_...) that gets the replies.",
		"reply_member_id":  "Teammate (mem_...) who gets the replies.",
		"ai_replies":       "true lets the AI agent answer replies.",
		"track_clicks":     "true counts who clicks the links.",
		"skip_frequency_cap": "true sends even to people who got the most messages the workspace allows " +
			"lately.",
		"schedule": "When it goes out: {\"kind\": \"now\"}, {\"kind\": \"at\", \"at\": RFC 3339 time} or " +
			"{\"kind\": \"local\", \"at\": time} for that time in each person's own zone.",
		"goal": "What it hopes people do: {\"event\": event name, \"days\": days after getting it}. " +
			"Find events with dimmi_event_definitions_list.",
	}
}

/*
broadcastReviewDocs returns the entries for checking a broadcast before
and after it is sent.
*/
func broadcastReviewDocs() map[string]Doc {
	return map[string]Doc{
		"broadcasts.estimate.show": {
			Title: "Estimate a broadcast's reach",
			Use: "Counts who the broadcast would reach right now and why the rest would be left out, with the " +
				"same checks sending makes. Nothing is written. Very large audiences are counted up to a cap.",
			Returns: "the count it would reach, the counts left out by reason and whether the cap was hit.",
		},
		"broadcasts.report.show": {
			Title: "Get a broadcast's report",
			Use: "Reads what a broadcast did: how many it reached, opened, clicked, replied and reached the " +
				"goal, why the rest were left out, which links drew clicks and how each account responded.",
			Returns: "the broadcast's counts, reasons, links and accounts.",
			Next:    []string{"dimmi_broadcasts_recipients_list: the people behind a number"},
		},
		"broadcasts.recipients.index": {
			Title:   "List a broadcast's recipients",
			Use:     "Lists the people a broadcast went to, narrowed by what happened or by account.",
			Returns: "a page of recipients with their contact, address, status and what they did.",
			Params: map[string]string{
				"status": "queued, sent, delivered, failed, bounced, complained, suppressed, skipped, recalled, " +
					"or opened, clicked, replied or unsubscribed.",
				"company_id": "Only recipients of this company (cmp_...).",
				"offset":     "How many recipients to skip, for the next page.",
			},
		},
		"broadcasts.tests.store": {
			Title: "Send a test of a broadcast",
			Use: "Sends the broadcast once to one email address or one Slack channel, filled in with the " +
				"person's own details, marked as a test and counted nowhere. Messenger broadcasts cannot " +
				"be tested this way.",
			Returns: "nothing once it is sent.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"email":   "Address the test goes to, usually the person's own.",
				"room_id": "Slack channel (chr_...) the test goes to, instead of an email.",
			},
		},
	}
}
