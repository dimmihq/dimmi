/*
|--------------------------------------------------------------------------
| Tool Catalog: Notification Destinations
|--------------------------------------------------------------------------
|
| The entries for the Slack places the team hears about its work in: a
| channel the team picked, or a teammate's own direct message with the
| bot. They reach only the team, never customers.
|
*/

package mcpserver

/*
destinationDocs returns the entries for listing, reading and testing
notification destinations.
*/
func destinationDocs() map[string]Doc {
	return map[string]Doc{
		"notification-destinations.index": {
			Title:   "List notification destinations",
			Use:     "Lists the Slack channels the team hears in, and the person's own Slack direct message.",
			Returns: "every destination with its type, what it hears about, its digests and status.",
		},
		"notification-destinations.show": {
			Title:   "Get a notification destination",
			Use:     "Reads one destination: where it posts, what it hears about and when its next digest is due.",
			Returns: "the destination.",
		},
		"notification-destinations.tests.store": {
			Title:   "Test a notification destination",
			Use:     "Posts a test card to the destination now, to check the bot can reach it.",
			Returns: "whether the post went through, with the error when it did not.",
			Hint:    "slack_unavailable means the Slack installation must be reconnected at a browser.",
		},
	}
}

/*
destinationChangeDocs returns the entries for adding, changing and
removing notification destinations.
*/
func destinationChangeDocs() map[string]Doc {
	return map[string]Doc{
		"notification-destinations.store": {
			Title: "Add a notification destination",
			Use: "Makes a Slack channel of the workspace hear about the team's work, or links the person's own " +
				"Slack direct message. A workspace posts to at most 25 channels.",
			Returns: "the destination.",
			Next:    []string{"dimmi_notification_destinations_tests_create: check it works"},
			Params:  destinationParams(),
		},
		"notification-destinations.update": {
			Title: "Change a notification destination",
			Use: "Changes what a destination hears about, its digests, or turns it off, or back on after an " +
				"error. Fields left out stay as they are.",
			Returns: "the destination as it now is.",
			Params:  destinationParams(),
		},
		"notification-destinations.destroy": {
			Title:   "Remove a notification destination",
			Use:     "Stops posting to a destination. Nothing changes in Slack itself.",
			Returns: "nothing once it is removed.",
		},
	}
}

/*
destinationParams explains the fields of a notification destination.
*/
func destinationParams() map[string]string {
	return map[string]string{
		"type": "slack_channel for a channel of the workspace, slack_dm for the person's own direct message.",
		"integration_id": "Slack installation (itg_...) it posts through. Find it with " +
			"dimmi_integrations_list.",
		"channel_id": "For slack_channel: the Slack channel id. Find it with dimmi_integrations_slack_channels_list.",
		"name":       "A name for the team, up to 80 characters.",
		"triggers": "What it hears about: new_conversation, customer_replied, conversation_assigned, " +
			"ai_handoff_requested and snooze_expired.",
		"inbox_ids":          "Only conversations of these inboxes (inbox_...); left out means every inbox.",
		"all_inboxes":        "true goes back to every inbox.",
		"include_ai_handled": "true also posts conversations the AI agent is handling.",
		"show_previews":      "false hides what customers wrote.",
		"digests":            "daily_standup, daily_summary and weekly_themes.",
		"digest_time":        "When digests post, as HH:MM in the workspace's time zone.",
		"enabled":            "false turns it off; true turns it back on.",
	}
}
