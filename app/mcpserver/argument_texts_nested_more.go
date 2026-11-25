/*
|--------------------------------------------------------------------------
| Written Argument Texts: More Fields Inside Arguments
|--------------------------------------------------------------------------
|
| The fields inside the arguments of the last tools by name, each by
| its path from the argument.
|
*/

package mcpserver

/*
moreNestedTexts returns the texts written for these tools.
*/
func moreNestedTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range []map[string]map[string]string{
		moreNestedTextsFromStartImport(),
		moreNestedTextsFromUpdateBroadcast(),
		moreNestedTextsFromUpdateDraft(),
		moreNestedTextsFromUpdateHelpCenter(),
		moreNestedTextsFromUpdateNotificationPreferences(),
		moreNestedTextsFromUpdateProcedureTestCase(),
		moreNestedTextsFromUpdateSnoozeList(),
		moreNestedTextsFromUpdateWidgetSettings(),
	} {
		mergeTexts(out, part)
	}

	return out
}

/*
moreNestedTextsFromStartImport returns texts of start_import, test_ai_action and
the tools after them.
*/
func moreNestedTextsFromStartImport() map[string]map[string]string {
	return map[string]map[string]string{
		"start_import": {
			"mapping.column": "Position of the column in the uploaded file, counting from 0, so the first " +
				"column is 0. It must be a column the file has, such as 2.",
			"mapping.field": "Field this column fills: email, name, external_id, phone, company_name, " +
				"company_domain or tags for contacts; name, domain, external_id or tags for companies; or " +
				"attribute.<key>, such as attribute.plan.",
		},
		"test_ai_action": {
			"contact.attributes": "Older name for custom_attributes, read only when custom_attributes is left " +
				"out. Attribute keys and values of the stand-in customer, such as {\"plan\": \"pro\"}.",
			"contact.custom_attributes": "Attribute keys and values of the stand-in customer the test runs as, " +
				"such as {\"plan\": \"pro\"}.",
			"contact.email": "Email address of the stand-in customer the test runs as, treated as verified, " +
				"such as ada@example.com.",
			"contact.external_id": "Your own system's id for the stand-in customer the test runs as, such as " +
				"user_123.",
		},
		"update_ai_action": {
			"response_fields.description": "Short note telling the agent what this value means, such as Date " +
				"the order shipped.",
			"response_fields.name": "Name the agent reads this value under: lowercase letters, digits and " +
				"underscores, starting with a letter, at most 48 characters, used once, such as order_status.",
			"response_fields.path": "Dotted path to the value in the JSON response, at most 200 characters; " +
				"use a number for a list position and # to take every item, such as data.order.status or " +
				"items.#.name.",
		},
		"update_ai_action_connection": {
			"auth_config.header_name": "Header the API key is sent in, needed when auth_type is " +
				"api_key_header: 1 to 64 letters, digits or dashes, such as X-API-Key.",
			"auth_config.prefix": "Word put before the token in the Authorization header when auth_type is " +
				"bearer. Bearer is used when left out; another example is Token.",
			"auth_config.query_param": "Query parameter the API key is sent in, needed when auth_type is " +
				"api_key_query: 1 to 64 letters, digits or dashes, such as api_key.",
		},
	}
}

/*
moreNestedTextsFromUpdateBroadcast returns texts of update_broadcast,
update_channel_room and the tools after them.
*/
func moreNestedTextsFromUpdateBroadcast() map[string]map[string]string {
	return map[string]map[string]string{
		"update_broadcast": {
			"schedule.at": "When the broadcast goes out, as an ISO 8601 time in the future, such as " +
				"2027-01-15T09:00:00Z. Needed when kind is at or local; leave out when kind is now.",
			"schedule.kind": "When the broadcast goes out: now sends right away, at sends at the time in at, " +
				"local sends at that clock time in each person's own time zone.",
		},
		"update_channel_room": {
			"settings.agent_delay_seconds": "Seconds the AI agent waits before answering in this room so a " +
				"teammate can reply first, from 0 to 900, such as 120.",
			"settings.agent_delay_when": "When the agent's wait applies: always, or inside_hours to wait only " +
				"during office hours.",
			"settings.agent_mode": "How the AI agent works in this room: auto lets it answer, draft makes it " +
				"write drafts instead.",
			"settings.auto_join": "true or false: the auto-join flag stored on this room.",
			"settings.auto_reply_after_seconds": "Seconds after a new conversation starts before the automatic " +
				"reply is posted, from 0 to 900; 0 sends no automatic reply. Such as 60.",
			"settings.request_sensitivity": "How readily a message counts as a request: conservative needs " +
				"more confidence, aggressive accepts less.",
			"settings.triage_channel_id": "Slack's own id of the channel where this room's new conversations " +
				"are posted for triage, 1 to 40 characters, such as C0123ABCD.",
		},
		"update_channel_whatsapp_template": {
			"buttons.phone_number": "Number a phone_number button calls, with a plus sign and country code and " +
				"7 to 19 digits, such as +2348012345678. Only for type phone_number.",
			"buttons.text": "Label shown on the button, 1 to 25 characters, such as Track order.",
			"buttons.type": "Kind of button: quick_reply sends the label back as a reply, url opens a link " +
				"(two at most), phone_number starts a call (one at most).",
			"buttons.url": "Full https address a url button opens, with no variables, at most 2000 characters, " +
				"such as https://example.com/orders. Only for type url.",
		},
		"update_draft": {
			"recipients.bcc": "List of up to 50 addresses to send a hidden copy to, saved with the draft as " +
				"given, such as [\"audit@example.com\"].",
		},
	}
}

/*
moreNestedTextsFromUpdateDraft returns texts of update_draft,
update_email_address
and the tools after them.
*/
func moreNestedTextsFromUpdateDraft() map[string]map[string]string {
	return map[string]map[string]string{
		"update_draft": {
			"recipients.cc": "List of up to 50 addresses to copy, saved with the draft as given, such as " +
				"[\"lead@example.com\"].",
			"recipients.channel_connection_id": "Id of the connected channel the draft is meant to go out " +
				"through, at most 64 characters, saved with the draft as given.",
			"recipients.subject": "Subject line saved with the draft, at most 998 characters, such as Your " +
				"refund request.",
			"recipients.to": "List of up to 50 addresses the draft is meant for, saved with the draft as " +
				"given, such as [\"ada@example.com\"].",
		},
		"update_email_address": {
			"settings.away_reply": "true answers a new email that arrives outside office hours once, saying " +
				"when the team is back; false sends nothing. New addresses start with false.",
			"settings.away_reply_text": "What the away reply says, at most 1000 characters. {{reply_time}} " +
				"becomes when the team is back.",
			"settings.brand_color": "Color used in branded replies, as # and six hex digits, such as #10b981. " +
				"Only used when template is branded.",
			"settings.bulk_mail_handling": "What happens to bulk mail such as newsletters that starts a new " +
				"thread: close files it as a closed conversation, open leaves it open, drop ignores it. New " +
				"addresses use close.",
			"settings.footer_markdown": "Footer that closes branded replies, written in Markdown, at most 500 " +
				"characters, such as Sent by the Acme support team.",
			"settings.quote_previous": "true puts the message being answered below each reply; false leaves it " +
				"out. New addresses start with true.",
			"settings.template": "Look of replies from this address: plain or branded. Branded uses " +
				"brand_color and footer_markdown.",
		},
		"update_help_center": {
			"settings.appearance": "Color theme of the help center: auto, light or dark.",
			"settings.changelog": "true shows the changelog on the help center, false hides it. It is on until " +
				"set to false.",
			"settings.changelog_authors": "true shows authors on changelog entries, false hides them. It is on " +
				"until set to false.",
			"settings.changelog_email": "What subscribers are mailed for a new changelog entry: full sends the " +
				"whole entry, summary sends only its summary.",
			"settings.changelog_reactions": "true turns reactions on for changelog entries, false turns them " +
				"off. It is on until set to false.",
			"settings.changelog_subscriptions": "true turns changelog subscriptions on, false turns them off. " +
				"It is on until set to false.",
		},
	}
}

/*
moreNestedTextsFromUpdateHelpCenter returns texts of update_help_center,
update_many_members and the tools after them.
*/
func moreNestedTextsFromUpdateHelpCenter() map[string]map[string]string {
	return map[string]map[string]string{
		"update_help_center": {
			"settings.changelog_title": "Heading of the changelog pages, at most 60 characters, such as What's " +
				"new. An empty text goes back to Changelog.",
			"settings.changelog_undo_minutes": "Minutes to wait after publishing a changelog entry before it " +
				"is mailed: 0, 5, 15 or 60.",
			"settings.contact_url": "Contact link of the help center; it must start with https://, such as " +
				"https://example.com/contact. An empty text clears it.",
			"settings.email_sign_in": "true lets readers sign in with a code mailed to them, false stops that. " +
				"It is on until set to false.",
			"settings.feedback_approval": "true holds what customers post on the feature requests board until " +
				"the team approves it.",
			"settings.feedback_board": "true opens the feature requests board to customers on the help center.",
			"settings.footer_text": "Text shown in the help center's footer, at most 500 characters, such as " +
				"Acme Inc. An empty text clears it.",
			"settings.home_subtitle": "Smaller line under the home page title, at most 200 characters, such as " +
				"Search our guides or browse by topic.",
			"settings.home_title": "Main heading of the help center's home page, at most 80 characters, such " +
				"as How can we help?",
			"settings.login_url": "Https address where readers sign in to your product, which sends them back " +
				"signed in, such as https://app.example.com/login. An empty text clears it.",
			"settings.messenger": "true shows a messenger on the help center's pages, false hides it. It is on " +
				"until set to false.",
			"settings.messenger_id": "Id of the workspace messenger shown on the help center, starting with " +
				"wgt_. An empty text uses the workspace's first messenger.",
			"settings.primary_color": "Main color of the help center, as # and six hex digits, such as " +
				"#10b981. An empty text clears it.",
		},
		"update_many_members": {
			"changes.custom_role_id": "Id of the custom role to give the members, starting with rol_; null " +
				"takes their custom role away. Send this or role, never both.",
			"changes.role": "Built-in role to give the members: admin or member. Send this or custom_role_id, " +
				"never both.",
		},
		"update_notification_preferences": {
			"preferences.channel": "Where the notification is delivered: in_app, email, push or slack.",
		},
	}
}

/*
moreNestedTextsFromUpdateNotificationPreferences returns texts of
update_notification_preferences,
update_operator_routine and the tools after them.
*/
func moreNestedTextsFromUpdateNotificationPreferences() map[string]map[string]string {
	return map[string]map[string]string{
		"update_notification_preferences": {
			"preferences.enabled": "true turns this trigger on for this channel, false turns it off.",
			"preferences.trigger": "Event the setting is for: new_conversation, conversation_assigned, " +
				"customer_replied, mentioned, ai_handoff_requested, ai_action_approval_requested, " +
				"snooze_expired, team_joined or operator_routine_finished.",
		},
		"update_operator_routine": {
			"delivery.email_member_ids": "Ids of up to 100 teammates emailed about the routine's runs, each " +
				"starting with mem_.",
			"delivery.in_app_member_ids": "Ids of up to 100 teammates told in the app about the routine's " +
				"runs, each starting with mem_.",
			"delivery.slack_destination_ids": "Ids of up to 20 Slack notification destinations told about the " +
				"routine's runs, each starting with ndst_.",
			"scope.github": "GitHub account and repositories the routine may read: installation_id and " +
				"repositories are needed, base_branch and ignore_labels are optional. The caller needs " +
				"github:read.",
			"scope.github.base_branch": "Branch whose merged pull requests the routine reads, at most 200 " +
				"characters, such as main.",
			"scope.github.ignore_labels": "Up to 50 label names; pull requests carrying any of them are " +
				"skipped, such as [\"internal\", \"chore\"].",
			"scope.github.installation_id": "Id of a connected, active GitHub installation in this workspace, " +
				"starting with itg_.",
			"scope.github.repositories": "1 to 20 repositories the routine may read, each written as " +
				"owner/repo, such as [\"acme/web\"].",
			"scope.help_center_ids": "Ids of up to 20 help centers the routine drafts in, each starting with " +
				"hc_.",
		},
		"update_procedure_test_case": {
			"messages.confirm": "true means the customer first presses Confirm on the actions waiting from the " +
				"turn before. With true the text may be empty.",
			"messages.text": "What the made-up customer writes in this turn, 1 to 4000 characters, such as I " +
				"want a refund for my last order.",
			"persona.attributes": "Attribute keys and values of the made-up customer, all treated as trusted, " +
				"such as {\"plan\": \"pro\"}.",
		},
	}
}

/*
moreNestedTextsFromUpdateProcedureTestCase returns texts of
update_procedure_test_case,
update_saved_reply and the tools after them.
*/
func moreNestedTextsFromUpdateProcedureTestCase() map[string]map[string]string {
	return map[string]map[string]string{
		"update_procedure_test_case": {
			"persona.channel": "Channel the made-up customer writes from: widget, email, slack, whatsapp, " +
				"instagram, facebook or api. Empty means widget.",
			"persona.company": "Attribute keys and values of the made-up customer's company, such as " +
				"{\"plan\": \"enterprise\"}.",
			"persona.email": "Email address of the made-up customer, at most 320 characters, such as " +
				"ada@example.com.",
			"persona.external_id": "Your own system's id for the made-up customer, at most 200 characters, " +
				"such as user_123. Giving one makes them a known user rather than a lead.",
			"persona.inbox_id": "Id of the inbox the test conversation is placed in, starting with inbox_.",
			"persona.name":     "Name of the made-up customer, at most 200 characters, such as Ada Obi.",
			"persona.verified": "true means the made-up customer is signed in, so their email counts as " +
				"verified; false means they are not.",
		},
		"update_saved_reply": {
			"actions.assignee_id": "Id of the teammate the conversation is assigned to when the reply is used, " +
				"starting with mem_.",
			"actions.priority": "Priority the conversation is set to when the reply is used: none, low, " +
				"medium, high or urgent.",
			"actions.snooze_preset": "How long to snooze when status is snoozed: later_today, tomorrow, " +
				"next_week or one_week.",
			"actions.status": "Status the conversation moves to when the reply is used: open, closed or " +
				"snoozed. With snoozed, also give snooze_preset.",
			"actions.tag_ids": "Ids of up to 10 tags added to the conversation when the reply is used, each " +
				"starting with tag_.",
			"actions.team_id": "Id of the team the conversation is assigned to when the reply is used, " +
				"starting with team_.",
		},
		"update_schedule": {
			"holidays.date": "Day the team is closed all day, written as YYYY-MM-DD, such as 2026-12-25.",
			"holidays.name": "Name of the holiday, such as Christmas Day.",
			"holidays.yearly": "true closes the same month and day every year; false or left out closes only " +
				"on that one date.",
		},
	}
}

/*
moreNestedTextsFromUpdateSnoozeList returns texts of update_snooze_list,
update_ticket_field and the tools after them.
*/
func moreNestedTextsFromUpdateSnoozeList() map[string]map[string]string {
	return map[string]map[string]string{
		"update_snooze_list": {
			"options.at": "Clock time the snooze ends, as 24-hour HH:MM in the reader's time zone, such as " +
				"09:00. Needed when kind is day_at or weekday_at.",
			"options.days_ahead": "How many days from today the snooze ends, from 1 to 365, such as 3. Needed " +
				"when kind is day_at.",
			"options.key": "Short name that identifies this choice in the list, 1 to 40 characters and used " +
				"only once, such as monday_morning.",
			"options.kind": "How the end time is worked out: preset uses a built-in preset, in counts minutes " +
				"from now, day_at is a number of days ahead at a time, weekday_at is the next given weekday at " +
				"a time.",
			"options.label": "Text teammates see for this choice, 1 to 60 characters, such as Monday morning.",
			"options.minutes": "Minutes from now until the snooze ends, from 5 to 525600, such as 120. Needed " +
				"when kind is in.",
			"options.preset": "Built-in preset to use: later_today, tomorrow, next_week or one_week. Needed " +
				"when kind is preset.",
			"options.weekday": "Day of the week the snooze ends, from 1 for Monday to 7 for Sunday, never " +
				"today, such as 1. Needed when kind is weekday_at.",
		},
		"update_ticket_field": {
			"show_when.equals": "Up to 50 values of the other field that make this field show, such as " +
				"[\"refund\", \"exchange\"]. Leave out to show whenever the other field has any value.",
			"show_when.field": "Key of the other ticket field this one depends on, at most 40 characters, such " +
				"as request_kind.",
		},
		"update_ticket_type": {
			"move_tickets.from_state_id": "Id of a state the ticket type no longer uses, whose tickets must " +
				"move, starting with tkts_.",
			"move_tickets.to_state_id": "Id of the state those tickets move to, one of the states kept in " +
				"state_ids, starting with tkts_.",
		},
		"update_widget_settings": {
			"home_links.icon": "Icon shown beside the link: link, calendar, book, mail, phone, chat, video or " +
				"help. link is used when left out.",
		},
	}
}

/*
moreNestedTextsFromUpdateWidgetSettings returns texts of update_widget_settings,
update_workspace.
*/
func moreNestedTextsFromUpdateWidgetSettings() map[string]map[string]string {
	return map[string]map[string]string{
		"update_widget_settings": {
			"home_links.title": "Text of the link on the messenger's home screen, 1 to 40 characters, such as " +
				"Book a demo.",
			"home_links.url": "Where the link goes: an https:// address or a mailto: address, at most 2048 " +
				"characters, such as https://example.com/demo.",
		},
		"update_workspace": {
			"link_safety.allow_domains": "Up to 500 domains whose links skip the warning; each also covers " +
				"every name under it, such as [\"example.com\"]. The list sent replaces the old one.",
			"link_safety.block_domains": "Up to 500 domains whose links cannot be clicked; each also covers " +
				"every name under it, such as [\"bad.example\"]. The list sent replaces the old one.",
			"link_safety.enabled": "true turns link warnings on, false turns them off. They start on.",
			"link_safety.strip_inbound_email_links": "true shows the links in incoming email as plain text " +
				"until a teammate reveals them; false leaves them as links.",
			"retention.closed_conversation_days": "Days closed conversations are kept, from 30 to 3650, such " +
				"as 365. null keeps them for ever.",
			"retention.export_days": "Days a data export is kept, from 1 to 30, such as 7.",
			"retention.raw_email_days": "Days raw email is kept, from 0 to 365, such as 30. 0 removes it at " +
				"once.",
			"retention.spam_days":  "Days spam is kept, from 7 to 365, such as 30.",
			"retention.trash_days": "Days trash is kept, from 7 to 365, such as 30.",
		},
	}
}
