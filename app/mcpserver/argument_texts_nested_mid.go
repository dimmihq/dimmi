/*
|--------------------------------------------------------------------------
| Written Argument Texts: Fields Inside Arguments, Continued
|--------------------------------------------------------------------------
|
| The fields inside the arguments of the tools in the middle by name,
| each by its path from the argument.
|
*/

package mcpserver

/*
midNestedTexts returns the texts written for these tools.
*/
func midNestedTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range []map[string]map[string]string{
		midNestedTextsFromCreateSavedReplyImport(),
		midNestedTextsFromCreateTicketTypeField(),
		midNestedTextsFromPreviewWidgetSettings(),
		midNestedTextsFromPreviewWidgetSettingsAndProposeArticleEdit(),
		midNestedTextsFromSimulateProcedure(), midNestedTextsFromSimulateProcedureContinued(),
	} {
		mergeTexts(out, part)
	}

	return out
}

/*
midNestedTextsFromCreateSavedReplyImport returns texts of
create_saved_reply_import,
create_schedule and the tools after them.
*/
func midNestedTextsFromCreateSavedReplyImport() map[string]map[string]string {
	return map[string]map[string]string{
		"create_saved_reply_import": {
			"replies.body": "The text of the saved reply in Markdown, 1 to 65536 characters, such as Hi, " +
				"thanks for writing in. Your refund is on its way.",
			"replies.shared": "true to share the reply with the whole workspace; false or left out to keep it " +
				"to yourself.",
			"replies.shortcut": "A short word to find the reply by, at most 40 characters, such as refund. A " +
				"leading # or / is dropped.",
			"replies.title": "The name of the saved reply, 1 to 120 characters, such as Refund confirmed.",
		},
		"create_schedule": {
			"holidays.date": "The day the team is closed all day, written YYYY-MM-DD, such as 2026-12-25.",
			"holidays.name": "What the holiday is called, such as Christmas Day. It may be left out.",
			"holidays.yearly": "true to close on the same month and day every year; false or left out for that " +
				"one date only. A yearly February 29 closes only in leap years.",
		},
		"create_ticket": {
			"link_to.conversation_id": "Id of the conversation to link the new ticket to, starting with conv_, " +
				"such as conv_01jx2m8k4t. A tracker link needs it. Send this or ticket_id.",
			"link_to.kind": "What the link means: back_office (from a customer ticket to back-office work, " +
				"needs ticket_id), tracker (from a tracker ticket to a conversation, needs conversation_id), " +
				"follow_up or related. Such as \"related\".",
			"link_to.ticket_id": "Id of the other ticket to link the new ticket to, starting with tkt_, such " +
				"as tkt_01jx2m8k4t. Find it with dimmi_list_tickets. A back_office link needs it. Send this or " +
				"conversation_id.",
		},
		"create_ticket_type": {
			"move_tickets.from_state_id": "Id of a state that state_ids no longer lists but that still holds " +
				"tickets of this type, starting with tkts_, such as tkts_01jx2m8k4t. Find it with " +
				"dimmi_list_ticket_states.",
			"move_tickets.to_state_id": "Id of the state those tickets move to, one of the states state_ids " +
				"keeps, starting with tkts_, such as tkts_01jx2m8k4t. Find it with dimmi_list_ticket_states.",
		},
	}
}

/*
midNestedTextsFromCreateTicketTypeField returns texts of
create_ticket_type_field,
create_widget_settings and the tools after them.
*/
func midNestedTextsFromCreateTicketTypeField() map[string]map[string]string {
	return map[string]map[string]string{
		"create_ticket_type_field": {
			"show_when.equals": "The values of the other field that make this field show, as a list of up to " +
				"50 texts, such as [\"Refund\"]. Left out or empty, any value shows it.",
			"show_when.field": "The key of another field of the same ticket type that must have a value for " +
				"this field to show, at most 40 characters, such as reason.",
		},
		"create_widget_settings": {
			"home_links.icon": "The icon shown beside the link: link, calendar, book, mail, phone, chat, video " +
				"or help. Left out, it is link. Such as \"calendar\".",
			"home_links.title": "The words the link shows on the messenger's home screen, 1 to 40 characters, " +
				"such as Book a demo.",
			"home_links.url": "Where the link goes: an https:// address or a mailto: address, at most 2048 " +
				"characters, such as https://example.com/demo or mailto:help@example.com.",
		},
		"preview_import": {
			"mapping.column": "Which column of the file this is, counted in the header from 0, such as 0 for " +
				"the first column.",
			"mapping.field": "The field the column fills: email, name, external_id, phone, company_name, " +
				"company_domain or tags for contacts; name, domain, external_id or tags for companies; or " +
				"attribute.<key> for a custom attribute, such as attribute.plan.",
		},
		"preview_widget_settings": {
			"request_body.allow_anonymous": "Whether visitors may write in the messenger before they sign in " +
				"to your product. true lets anonymous visitors start a session, false refuses them. Example: " +
				"true.",
			"request_body.allowed_origins": "Sites that may load the messenger, as a list of up to 50, such as " +
				"[\"https://app.example.com\"]; *.example.com covers one label. An empty list allows any site.",
			"request_body.appearance": "The messenger's look, as an object whose keys merge one by one: " +
				"primary_color, theme (light, dark, auto), position (right, left), padding_x, padding_y, font, " +
				"sound, launcher, background and more. Such as {\"theme\": \"dark\"}.",
			"request_body.attachments_enabled": "Whether visitors may upload files in the messenger. true " +
				"allows attachments, false refuses uploads. Example: false.",
		},
	}
}

/*
midNestedTextsFromPreviewWidgetSettings returns texts of
preview_widget_settings.
*/
func midNestedTextsFromPreviewWidgetSettings() map[string]map[string]string {
	return map[string]map[string]string{
		"preview_widget_settings": {
			"request_body.changelog_tab_enabled": "Whether the messenger shows a changelog tab. The tab only " +
				"appears when the messenger also has a help center set (help_center_id). Example: true.",
			"request_body.collect_email": "When the messenger asks a visitor for their email: never, offline " +
				"or always, such as \"offline\".",
			"request_body.enabled": "true to show the messenger; false to hide it on every site.",
			"request_body.help_center_id": "Id of the help center the help tab shows, starting with hc_, such " +
				"as hc_01jx2m8k4t. Find it with dimmi_list_help_centers.",
			"request_body.help_tab_enabled": "Whether the messenger shows a help tab with articles. The tab " +
				"only appears when the messenger also has a help center set (help_center_id). Example: true.",
			"request_body.home_links": "The links on the messenger's home screen, as a list of up to 6 objects " +
				"with title, url and icon; it replaces the whole list. Such as [{\"title\": \"Book a demo\", " +
				"\"url\": \"https://example.com/demo\", \"icon\": \"calendar\"}].",
			"request_body.home_links.icon": "The icon shown beside the link: link, calendar, book, mail, " +
				"phone, chat, video or help. Left out, it is link. Such as \"calendar\".",
			"request_body.home_links.title": "The words the link shows on the messenger's home screen, 1 to 40 " +
				"characters, such as Book a demo.",
			"request_body.home_links.url": "Where the link goes: an https:// address or a mailto: address, at " +
				"most 2048 characters, such as https://example.com/demo or mailto:help@example.com.",
			"request_body.inbox_id": "Id of the inbox the messenger's conversations land in, starting with " +
				"inbox_, such as inbox_01jx2m8k4t. Find it with dimmi_list_inboxes.",
			"request_body.intercom_shim": "Whether the messenger's loader keeps an old Intercom snippet's " +
				"calls working while a customer moves over to dimmi. Example: true.",
			"request_body.locale": "The messenger's language: auto to follow the visitor's browser, or a " +
				"language tag, such as fr or pt-BR.",
			"request_body.max_attachment_mb": "The largest file a visitor may attach, in megabytes, a whole " +
				"number from 1 to 25, such as 10.",
			"request_body.name": "The messenger's name, 1 to 60 characters, such as Main website.",
			"request_body.native_apps_enabled": "Whether requests without an Origin header, which native " +
				"mobile apps send, are accepted even when the messenger has a list of allowed sites. Example: " +
				"true.",
			"request_body.reply_time": "The reply time visitors are told: auto, minutes, hours or day, such as " +
				"\"hours\".",
		},
	}
}

/*
midNestedTextsFromPreviewWidgetSettingsAndProposeArticleEdit returns
texts of preview_widget_settings,
propose_article_edit and the tools after them.
*/
func midNestedTextsFromPreviewWidgetSettingsAndProposeArticleEdit() map[string]map[string]string {
	return map[string]map[string]string{
		"preview_widget_settings": {
			"request_body.require_email_when_offline": "Whether a visitor with no email must give one before " +
				"sending a message while the team is away. Example: true.",
			"request_body.show_availability": "Whether the messenger shows visitors the team's availability. " +
				"Example: false.",
			"request_body.show_branding": "Whether the messenger shows the 'Powered by' badge: true shows it, " +
				"false hides it. On dimmi cloud, hiding is refused until the workspace has paid an invoice. " +
				"Example: true.",
			"request_body.show_team_avatars": "Whether the messenger shows visitors the team's avatars. " +
				"Example: false.",
			"request_body.texts": "Object of the messenger's own words: text key to plain text. Only known " +
				"keys such as greeting, welcome_message, offline_message, launcher_label. null or \"\" brings " +
				"back the built-in words. Such as {\"greeting\": \"Hi there\"}.",
			"request_body.texts_by_locale": "Object of the messenger's words per language: language tag to an " +
				"object with the same keys as texts; null for a language removes all its words. Such as " +
				"{\"fr\": {\"greeting\": \"Bonjour\"}}.",
		},
		"propose_article_edit": {
			"edits.find": "The passage to replace, copied word for word from the article's body_markdown as it " +
				"is now; one whole sentence or list item is best. It is replaced everywhere it appears. Such " +
				"as Refunds take 5 days.",
			"edits.replace": "What the passage should say instead, in Markdown, such as Refunds take 10 days. " +
				"An empty text removes the passage.",
		},
		"reply_to_conversation": {
			"snooze.option": "The key of one of this conversation's snooze choices, from " +
				"dimmi_list_snooze_options, at most 40 characters. With then=snooze send exactly one of until, " +
				"preset or option.",
			"snooze.preset": "When the snooze ends, as a preset: later_today, tomorrow, next_week or one_week, " +
				"such as \"tomorrow\". With then=snooze send exactly one of until, preset or option.",
			"snooze.until": "When the snooze ends, as an ISO 8601 time, such as 2026-12-01T09:00:00Z. With " +
				"then=snooze send exactly one of until, preset or option.",
		},
	}
}

/*
midNestedTextsFromSimulateProcedure returns texts of simulate_procedure.
*/
func midNestedTextsFromSimulateProcedure() map[string]map[string]string {
	return map[string]map[string]string{
		"simulate_procedure": {
			"active_state.branches": "Copy it unchanged from the state the last simulated turn handed back: an " +
				"object of step id to the side that step took.",
			"active_state.called_action_ids": "Copy it unchanged from the state the last simulated turn handed " +
				"back: the list of ids of the actions already called, such as [\"aac_01jx2m8k4t\"].",
			"active_state.completed_step_ids": "Copy it unchanged from the state the last simulated turn " +
				"handed back: the list of ids of the procedure's steps already done.",
			"active_state.pending": "Copy it unchanged from the state the last simulated turn handed back: the " +
				"list of writes still waiting to be confirmed or approved, each with action_id, tool, args, " +
				"state and summary.",
			"active_state.pending.action_id": "Id of the action whose write is waiting, starting with aac_, " +
				"such as aac_01jx2m8k4t, copied from the last turn's state. Put it in confirmed_action_ids to " +
				"confirm it.",
			"active_state.pending.args": "The arguments the waiting write would be called with, as an object, " +
				"copied unchanged from the last turn's state, such as {\"order_id\": \"1042\"}.",
			"active_state.pending.state": "Why the write waits, copied from the last turn's state: " +
				"awaiting_confirmation (the customer must confirm) or pending_approval (a teammate must " +
				"approve).",
			"active_state.pending.summary": "The short text that says what the waiting write would do, copied " +
				"unchanged from the last turn's state.",
			"active_state.pending.tool": "The tool name of the action whose write is waiting, copied unchanged " +
				"from the last turn's state.",
			"active_state.procedure_id": "Id of the procedure the conversation is in, starting with prc_, such " +
				"as prc_01jx2m8k4t, copied from the state the last simulated turn handed back.",
			"active_state.status": "Where the procedure stands, copied from the state the last simulated turn " +
				"handed back: active, completed, handed_off or exited. Only active carries the procedure on.",
			"active_state.step_outputs": "Copy it unchanged from the state the last simulated turn handed " +
				"back: an object of what the procedure's steps have produced so far.",
			"history.role": "Who wrote this earlier message: customer or agent, such as \"customer\".",
			"history.text": "What that earlier message said, at most 4000 characters, such as Hi, where is my " +
				"order?",
			"persona.attributes": "The made-up customer's custom attributes, as an object of attribute key to " +
				"value. They count as trusted. Such as {\"plan\": \"pro\"}.",
		},
	}
}

/*
midNestedTextsFromSimulateProcedureContinued returns texts of
simulate_procedure.
*/
func midNestedTextsFromSimulateProcedureContinued() map[string]map[string]string {
	return map[string]map[string]string{
		"simulate_procedure": {
			"persona.channel": "The channel the made-up customer writes from: widget, email, slack, whatsapp, " +
				"instagram, facebook or api. Left out, it is widget. Such as \"email\".",
			"persona.company": "The custom attributes of the made-up customer's company, as an object of " +
				"attribute key to value, such as {\"plan\": \"enterprise\"}.",
			"persona.email": "The made-up customer's email address, at most 320 characters, such as " +
				"ada@example.com.",
			"persona.external_id": "Your own system's id of the made-up customer, at most 200 characters, such " +
				"as user_8421. Giving one makes them a known user, not a lead.",
			"persona.inbox_id": "Id of the inbox the made-up conversation is in, starting with inbox_, such as " +
				"inbox_01jx2m8k4t. Find it with dimmi_list_inboxes.",
			"persona.name": "The made-up customer's name, at most 200 characters, such as Ada Lovelace.",
			"persona.verified": "true when the made-up customer is signed in, so their identity and email " +
				"count as verified; false or left out for an unverified visitor.",
		},
	}
}
