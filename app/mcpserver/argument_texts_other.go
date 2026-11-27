/*
|--------------------------------------------------------------------------
| Written Argument Texts: Channels, Messenger, Team And The Rest
|--------------------------------------------------------------------------
|
| The arguments of the tools for WhatsApp, Messenger and Instagram
| channels, the messenger, teammates, the workspace, help centers,
| feature requests, broadcasts, reports and what is left.
|
*/

package mcpserver

/*
otherTexts returns the texts written for these tools.
*/
func otherTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range []map[string]map[string]string{
		otherTextsFromChangeFeedbackStatus(),
		otherTextsFromCreateChannelWhatsapp(),
		otherTextsFromCreateConversationBulkChange(),
		otherTextsFromCreateWidgetSettings(),
		otherTextsFromSendWhatsappTemplateToConversation(),
		otherTextsFromUpdateChannelWhatsappTemplate(),
		otherTextsFromUpdateInboxPreferences(),
		otherTextsFromUpdateNotificationSettings(),
		otherTextsFromUpdateWidgetSettings(),
		helpCenterThemeTexts(),
	} {
		mergeTexts(out, part)
	}

	return out
}

/*
otherTextsFromChangeFeedbackStatus returns texts of change_feedback_status,
create_attribute_definition and the tools after them.
*/
func otherTextsFromChangeFeedbackStatus() map[string]map[string]string {
	return map[string]map[string]string{
		"change_feedback_status": {
			"category": "What the feedback status means, in the order work moves: open, planned, in_progress, " +
				"done (shipped) or closed. New requests start in the first open status; closed ones leave the " +
				"public roadmap. Such as planned.",
			"roadmap": "Whether the public roadmap shows this status as a column. A status in the closed " +
				"category is never shown even when true. Example: true.",
		},
		"create_attribute_definition": {
			"label": "The name people see for the attribute, 1 to 80 characters. Such as Plan tier.",
		},
		"create_broadcast": {
			"holds": "List of who to leave out of a conversation broadcast; only allowed when channel is " +
				"conversation. Values: customer_waiting (customers still waiting on an answer) and " +
				"team_replied (for a tracker, customers a teammate already answered since it last moved). Such " +
				"as [\"customer_waiting\"].",
		},
		"create_channel_facebook": {
			"account_id": "The id of the Facebook Page to connect, taken from the accounts list the Facebook " +
				"sign-in step returned together with the grant. Such as 104857600123456.",
			"grant": "The grant string the Facebook sign-in step (POST /channels/facebook/signups) returned. " +
				"It names that sign-in and expires after 15 minutes. Such as k3f9x0q2m7v1b8n4c6z5a1s2d3f4g5h6.",
		},
		"create_channel_instagram": {
			"account_id": "The id of the Instagram account to connect, taken from the accounts list the " +
				"Instagram sign-in step returned together with the grant. Such as 17841400123456789.",
			"grant": "The grant string the Instagram sign-in step (POST /channels/instagram/signups) returned. " +
				"It names that sign-in and expires after 15 minutes. Such as k3f9x0q2m7v1b8n4c6z5a1s2d3f4g5h6.",
		},
	}
}

/*
otherTextsFromCreateChannelWhatsapp returns texts of create_channel_whatsapp,
create_channel_whatsapp_account and the tools after them.
*/
func otherTextsFromCreateChannelWhatsapp() map[string]map[string]string {
	return map[string]map[string]string{
		"create_channel_whatsapp": {
			"phone_number_id": "Meta's id of the WhatsApp phone number to connect, one of the numbers of the " +
				"WhatsApp account named by waba_id. Required, up to 64 characters. Such as 106540352242922.",
			"waba_id": "Meta's id of the WhatsApp Business Account the access token reads and the number " +
				"belongs to. Required, up to 64 characters. Such as 102290129340398.",
		},
		"create_channel_whatsapp_account": {
			"waba_id": "Meta's id of the WhatsApp Business Account to read, with its numbers, using the access " +
				"token. Required, up to 64 characters. Such as 102290129340398.",
		},
		"create_channel_whatsapp_template": {
			"bindings": "Object saying which customer detail fills each template variable. Keys are " +
				"\"header.1\" or \"body.2\" (part and variable number); each value is {\"key\": lowercase " +
				"dotted name, \"fallback\": optional text up to 64 characters}. Such as " +
				"{\"body.1\":{\"key\":\"contact.first_name\",\"fallback\":\"there\"}}.",
			"buttons": "List of up to 10 buttons. Each is " +
				"{\"type\":\"quick_reply\"|\"url\"|\"phone_number\",\"text\": label up to 25 characters}, plus " +
				"\"url\" (full https address, no variables; at most 2 url buttons) or \"phone_number\" (such " +
				"as +2348012345678; at most 1). Such as [{\"type\":\"quick_reply\",\"text\":\"Yes\"}].",
			"category": "The template's category at Meta: utility or marketing. Required. Such as utility.",
			"footer": "Optional footer line under the message: plain text of at most 60 characters with no " +
				"{{variables}}. Such as Reply STOP to opt out.",
			"header": "Optional text header above the message: at most 60 characters, with at most one " +
				"variable written {{1}}, which needs a sample under \"header.1\". Such as Order {{1}}.",
			"samples": "Object with a sample value for every variable in the header and body, which Meta " +
				"reviews the template with. Keys are \"header.1\", \"body.1\", \"body.2\"; values are text up " +
				"to 200 characters. Such as {\"body.1\":\"Ada\",\"body.2\":\"#1042\"}.",
		},
	}
}

/*
otherTextsFromCreateConversationBulkChange returns texts of
create_conversation_bulk_change,
create_conversation_feedback_post and the tools after them.
*/
func otherTextsFromCreateConversationBulkChange() map[string]map[string]string {
	return map[string]map[string]string{
		"create_conversation_bulk_change": {
			"changes": "Object of changes to apply to every listed conversation; send this or trash, never " +
				"both. Fields: status (open, closed, snoozed), priority (none, low, medium, high, urgent), " +
				"assignee_id, team_id, inbox_id, spam, unread, add_tags, remove_tags, note. Such as " +
				"{\"status\":\"closed\",\"add_tags\":[\"billing\"]}.",
			"trash": "Set true to move every listed conversation to the trash instead of changing it. Send " +
				"this or changes, never both. Example: true.",
		},
		"create_conversation_feedback_post": {
			"body_markdown": "The description of the new feedback request, written in Markdown, up to 65536 " +
				"characters. Only used when a new request is made with a title, not when post_id names an " +
				"existing one. Such as Customers want to **export** reports as CSV.",
		},
		"create_widget_settings": {
			"allow_anonymous": "Whether visitors may write in the messenger before they sign in to your " +
				"product. true lets anonymous visitors start a session, false refuses them. New messengers " +
				"start with true. Example: true.",
			"attachments_enabled": "Whether visitors may upload files in the messenger. true allows " +
				"attachments, false refuses uploads. New messengers start with true. Example: false.",
			"changelog_tab_enabled": "Whether the messenger shows a changelog tab. The tab only appears when " +
				"the messenger also has a help center set (help_center_id). Example: true.",
			"help_tab_enabled": "Whether the messenger shows a help tab with articles. The tab only appears " +
				"when the messenger also has a help center set (help_center_id). Example: true.",
			"intercom_shim": "Whether the messenger's loader keeps an old Intercom snippet's calls working " +
				"while a customer moves over to dimmi. Off by default. Example: true.",
			"native_apps_enabled": "Whether requests without an Origin header, which is what native mobile " +
				"apps send, are accepted even when the messenger has a list of allowed sites. Off by default. " +
				"Example: true.",
			"require_email_when_offline": "Whether a visitor with no email must give one before sending a " +
				"message while the team is away. Off by default. Example: true.",
			"show_availability": "Whether the messenger shows visitors the team's availability. Part of the " +
				"config the messenger loads; new messengers start with true. Example: false.",
		},
	}
}

/*
otherTextsFromCreateWidgetSettings returns texts of create_widget_settings,
list_conversations_behind_report_number and the tools after them.
*/
func otherTextsFromCreateWidgetSettings() map[string]map[string]string {
	return map[string]map[string]string{
		"create_widget_settings": {
			"show_branding": "Whether the messenger shows the 'Powered by' badge: true shows it, false hides " +
				"it. On dimmi cloud, hiding is refused until the workspace has paid an invoice. Example: true.",
			"show_team_avatars": "Whether the messenger shows visitors the team's avatars. Part of the config " +
				"the messenger loads; new messengers start with true. Example: false.",
			"texts": "Object of the messenger's own words: text key to plain text. Only known keys such as " +
				"greeting, welcome_message, offline_message, composer_placeholder, launcher_label; each has a " +
				"length limit. null or \"\" brings back the built-in words. Such as {\"greeting\":\"Hi " +
				"there\",\"launcher_label\":\"Chat\"}.",
			"texts_by_locale": "Object of the messenger's words per language: language tag to an object with " +
				"the same keys as texts. null for a language removes all its words; up to 40 languages. Such " +
				"as {\"fr\":{\"greeting\":\"Bonjour\"},\"de\":null}.",
		},
		"list_conversations_behind_report_number": {
			"report": "The key of the report the number is on: overview, responsiveness, resolution, team, " +
				"live, satisfaction, ai or help_center. Such as overview.",
		},
		"get_migration_by_id": {
			"migration": "The migration's id (mig_...), as starting or listing migrations returns it. Such " +
				"as mig_01jc8q3x7m2n4p5r6s7t8v9w0y.",
		},
		"look_up_migration_records": {
			"migration": "The migration (mig_...) the records belong to. Such as mig_01jc8q3x7m2n4p5r6s7t8v9w0y.",
			"kind": "The kind of the records, as the import format names it: contact, conversation, article and " +
				"so on. Such as contact.",
			"source_ids": "Up to 100 ids the records had in the other tool. Such as [\"6512ab\",\"6512ac\"].",
		},
		"roll_back_migration": {
			"migration": "The migration (mig_...) to undo, as starting or listing migrations returns it. Such " +
				"as mig_01jc8q3x7m2n4p5r6s7t8v9w0y.",
		},
		"list_imports": {
			"kind": "Which imports to list: contacts (imports of customers) or companies. Required. Such as " +
				"contacts.",
		},
		"preview_widget_settings": {
			"request_body": "Object of unsaved messenger settings to preview, with the same fields " +
				"dimmi_update_widget_settings takes (name, appearance, texts, locale, show_branding and so " +
				"on); nothing is saved. Such as {\"texts\":{\"greeting\":\"Hi " +
				"there\"},\"help_tab_enabled\":true}.",
		},
		"resend_invitation": {
			"channel": "How to deliver the fresh invitation link: email mails it to the invited person (the " +
				"default), link only returns it to pass on by hand. Either way the link sent before stops " +
				"working. Such as link.",
		},
	}
}

/*
otherTextsFromSendWhatsappTemplateToConversation returns texts of
send_whatsapp_template_to_conversation,
update_attribute_definition and the tools after them.
*/
func otherTextsFromSendWhatsappTemplateToConversation() map[string]map[string]string {
	return map[string]map[string]string{
		"send_whatsapp_template_to_conversation": {
			"client_message_id": "An id you make up for this send, 1 to 64 characters, so that a retried " +
				"request finds the first message instead of sending twice. Such as 7f3c2b1a-send-1.",
			"values": "Object with the text for every variable of the template; a missing one is refused. Keys " +
				"are \"header.1\" or \"body.1\" (part and variable number), or just the number such as \"1\". " +
				"Such as {\"body.1\":\"Ada\",\"body.2\":\"#1042\"}.",
		},
		"update_attribute_definition": {
			"label": "The new name people see for the attribute, 1 to 80 characters. Such as Plan tier.",
			"options": "The values a list attribute may take: 1 to 200 different non-empty strings, replacing " +
				"the old list. Only for attributes whose data type is list; other types refuse any options. " +
				"Such as [\"free\",\"pro\",\"enterprise\"].",
		},
		"update_broadcast": {
			"holds": "List of who to leave out of a conversation broadcast, replacing the old list; only " +
				"allowed on a draft whose channel is conversation. Values: customer_waiting (customers still " +
				"waiting on an answer) and team_replied (for a tracker, customers a teammate already " +
				"answered). Such as [\"customer_waiting\"].",
		},
		"update_channel_whatsapp_template": {
			"bindings": "Object saying which customer detail fills each template variable. Keys are " +
				"\"header.1\" or \"body.2\" (part and variable number); each value is {\"key\": lowercase " +
				"dotted name, \"fallback\": optional text up to 64 characters}. Such as " +
				"{\"body.1\":{\"key\":\"contact.first_name\",\"fallback\":\"there\"}}.",
			"buttons": "List of up to 10 buttons. Each is " +
				"{\"type\":\"quick_reply\"|\"url\"|\"phone_number\",\"text\": label up to 25 characters}, plus " +
				"\"url\" (full https address, no variables; at most 2 url buttons) or \"phone_number\" (such " +
				"as +2348012345678; at most 1). Such as [{\"type\":\"quick_reply\",\"text\":\"Yes\"}].",
			"category": "The template's category at Meta: utility or marketing. Required. Such as utility.",
			"footer": "Optional footer line under the message: plain text of at most 60 characters with no " +
				"{{variables}}. Such as Reply STOP to opt out.",
		},
	}
}

/*
otherTextsFromUpdateChannelWhatsappTemplate returns texts of
update_channel_whatsapp_template,
update_event_definition and the tools after them.
*/
func otherTextsFromUpdateChannelWhatsappTemplate() map[string]map[string]string {
	return map[string]map[string]string{
		"update_channel_whatsapp_template": {
			"header": "Optional text header above the message: at most 60 characters, with at most one " +
				"variable written {{1}}, which needs a sample under \"header.1\". Such as Order {{1}}.",
			"samples": "Object with a sample value for every variable in the header and body, which Meta " +
				"reviews the template with. Keys are \"header.1\", \"body.1\", \"body.2\"; values are text up " +
				"to 200 characters. Such as {\"body.1\":\"Ada\",\"body.2\":\"#1042\"}.",
		},
		"update_event_definition": {
			"client_writable": "Whether the messenger may send this event from the visitor's browser. true " +
				"opens the event name to the messenger, false closes it. Example: true.",
			"label": "The new name people see for the event, 1 to 80 characters. Such as Checkout started.",
		},
		"update_event_settings": {
			"messenger_new_events": "Whether the messenger may send any event name, defining the ones the " +
				"workspace has not seen before. A plain boolean; leaving it out counts as false. Example: " +
				"true.",
		},
		"update_feedback_board": {
			"approval_required": "Whether what customers post on this board is held until the team approves " +
				"it. Example: true.",
			"visibility": "Who can see the board: public (everyone), private (signed-in customers only) or " +
				"internal (the team alone). Such as private.",
		},
		"update_feedback_post": {
			"body_markdown": "The new description of the feedback request, written in Markdown, up to 65536 " +
				"characters. Such as Customers want to **export** reports as CSV.",
		},
		"update_help_center": {
			"ask_ai_enabled": "Whether readers of the help center may ask a question and get an answer from " +
				"its articles (the Ask AI feature). Example: true.",
			"indexing_enabled": "Whether search engines such as Google may index the help center's articles. " +
				"false marks the site as not to be indexed. Example: true.",
		},
	}
}

/*
otherTextsFromUpdateInboxPreferences returns texts of
update_inbox_preferences, update_member
and the tools after them.
*/
func otherTextsFromUpdateInboxPreferences() map[string]map[string]string {
	return map[string]map[string]string{
		"update_inbox_preferences": {
			"sidebar": "Free-form object of the caller's own inbox sidebar settings. It replaces the whole " +
				"stored object, so send every key to keep. The dimmi app keeps keys such as pinned_views, " +
				"hide_events and customer_time in it. Such as {\"hide_events\":true}.",
		},
		"update_member": {
			"assign_on_reply": "The member's own rule for taking a conversation by replying to it: inherit " +
				"follows the inbox's rule, always assigns what they answer to them, never means replying does " +
				"not assign it to them. Such as always.",
			"away_reassign": "Whether, while the member is away, conversations whose customers answer go back " +
				"to the team. true is only accepted when the member is away; going online turns it off. " +
				"Example: true.",
			"capacity_by_channel": "Object of the most open conversations the member holds per channel type: " +
				"keys are widget, email or api, values are whole numbers from 1 to 500. It replaces the whole " +
				"stored object. Such as {\"email\":20,\"widget\":5}.",
		},
		"update_my_availability": {
			"conversation_access": "Which conversations the member sees: all, assigned, teams or " +
				"all_except_teams. This tool changes the caller's own member, and nobody can change their own " +
				"access, so sending it here is refused. Such as all.",
			"conversation_access_unassigned": "Whether a narrow access mode also shows conversations nobody is " +
				"assigned to. It is part of the caller's own conversation access, which nobody can change for " +
				"themselves, so sending it here is refused. Example: true.",
			"keep_owner": "Only allowed together with role set to owner. true keeps the caller an owner after " +
				"making someone else an owner; otherwise the caller becomes an admin. Example: true.",
			"role": "The caller's own built-in role: owner, admin or member. Any change to or from owner needs " +
				"an owner acting from their own browser with a recent confirmation. Such as member.",
			"status": "Whether the caller's own member is active or deactivated. Members cannot deactivate " +
				"themselves, so deactivated is refused here. Such as active.",
		},
	}
}

/*
otherTextsFromUpdateNotificationSettings returns texts of
update_notification_settings,
update_report_digest and the tools after them.
*/
func otherTextsFromUpdateNotificationSettings() map[string]map[string]string {
	return map[string]map[string]string{
		"update_notification_settings": {
			"quiet_hours_enabled": "Whether the caller's own quiet hours are on. The hours themselves are set " +
				"with quiet_hours_start, quiet_hours_end and quiet_hours_days. Example: true.",
		},
		"update_report_digest": {
			"channels": "List of up to 20 channel names the caller's report digest is narrowed to, replacing " +
				"the old list; conversations are matched on their channel, such as widget, email, api, slack, " +
				"whatsapp, facebook, instagram. The values are not checked. Such as [\"email\",\"widget\"].",
		},
		"update_widget_settings": {
			"allow_anonymous": "Whether visitors may write in the messenger before they sign in to your " +
				"product. true lets anonymous visitors start a session, false refuses them. New messengers " +
				"start with true. Example: true.",
			"attachments_enabled": "Whether visitors may upload files in the messenger. true allows " +
				"attachments, false refuses uploads. New messengers start with true. Example: false.",
			"changelog_tab_enabled": "Whether the messenger shows a changelog tab. The tab only appears when " +
				"the messenger also has a help center set (help_center_id). Example: true.",
			"help_tab_enabled": "Whether the messenger shows a help tab with articles. The tab only appears " +
				"when the messenger also has a help center set (help_center_id). Example: true.",
			"intercom_shim": "Whether the messenger's loader keeps an old Intercom snippet's calls working " +
				"while a customer moves over to dimmi. Off by default. Example: true.",
			"native_apps_enabled": "Whether requests without an Origin header, which is what native mobile " +
				"apps send, are accepted even when the messenger has a list of allowed sites. Off by default. " +
				"Example: true.",
			"require_email_when_offline": "Whether a visitor with no email must give one before sending a " +
				"message while the team is away. Off by default. Example: true.",
			"show_availability": "Whether the messenger shows visitors the team's availability. Part of the " +
				"config the messenger loads; new messengers start with true. Example: false.",
			"show_branding": "Whether the messenger shows the 'Powered by' badge: true shows it, false hides " +
				"it. On dimmi cloud, hiding is refused until the workspace has paid an invoice. Example: true.",
		},
	}
}

/*
otherTextsFromUpdateWidgetSettings returns texts of update_widget_settings,
update_workspace.
*/
func otherTextsFromUpdateWidgetSettings() map[string]map[string]string {
	return map[string]map[string]string{
		"update_widget_settings": {
			"show_team_avatars": "Whether the messenger shows visitors the team's avatars. Part of the config " +
				"the messenger loads; new messengers start with true. Example: false.",
			"texts": "Object of the messenger's own words: text key to plain text. Only known keys such as " +
				"greeting, welcome_message, offline_message, composer_placeholder, launcher_label; each has a " +
				"length limit. null or \"\" brings back the built-in words. Such as {\"greeting\":\"Hi " +
				"there\",\"launcher_label\":\"Chat\"}.",
			"texts_by_locale": "Object of the messenger's words per language: language tag to an object with " +
				"the same keys as texts. null for a language removes all its words; up to 40 languages. Such " +
				"as {\"fr\":{\"greeting\":\"Bonjour\"},\"de\":null}.",
		},
		"update_workspace": {
			"gif_picker": "Whether teammates may search GIFs from the reply composer. On unless turned off; " +
				"the picker also needs the install to have a GIF search key. Example: false.",
			"seen_receipt_shows_name": "Whether the customer's messenger names the teammate who read their " +
				"message. Off by default: the customer only learns that the team saw it. Example: true.",
			"seen_receipts": "Whether the customer's messenger is told when the team read their message. On " +
				"unless turned off. Example: false.",
			"show_viewers_on_list": "Whether the inbox list shows which teammates are viewing each " +
				"conversation. On unless turned off. Example: false.",
			"team_mentions_enabled": "Whether an internal note may mention a whole team. On unless turned off. " +
				"Example: false.",
		},
	}
}
