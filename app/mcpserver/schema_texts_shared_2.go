/*
|--------------------------------------------------------------------------
| Written Schema Texts: Fields Several Operations Share, Continued
|--------------------------------------------------------------------------
|
| More of the texts of fields several operations share.
|
*/

package mcpserver

/*
sharedTexts7 returns the texts from
TicketLinkBody.conversation_id on.
*/
func sharedTexts7() map[string]string {
	return map[string]string{
		"TicketLinkBody.conversation_id": "Id of the conversation the ticket is linked to, " +
			"starting with conv_, such as conv_01jx2m8k4t. Find it with dimmi_list_conversations or " +
			"dimmi_search_conversations_and_contacts. A tracker link needs it. Send this or ticket_id, " +
			"never both.",
		"TicketLinkBody.kind": "What the link means: back_office (from a customer ticket to " +
			"back-office work, needs ticket_id), tracker (from a tracker ticket to a conversation, needs " +
			"conversation_id), follow_up or related (join any two tickets). Such as \"related\".",
		"TicketLinkBody.ticket_id": "Id of the other ticket the ticket is linked to, starting with " +
			"tkt_, such as tkt_01jx2m8k4t. Find it with dimmi_list_tickets. A back_office link needs it. " +
			"Send this or conversation_id, never both.",
		"TicketStateBody.group": "The stage the state belongs to: submitted, in_progress, " +
			"waiting_customer, waiting_team or resolved, such as \"in_progress\". It is set when the state " +
			"is added and never changes; when updating, a value sent here is ignored.",
		"TicketStateMove.from_state_id": "Id of a state that state_ids no longer lists, so the " +
			"ticket type no longer uses it, but that still holds tickets of this type which must move. " +
			"Starts with tkts_, such as tkts_01jx2m8k4t. Find it with dimmi_list_ticket_states.",
		"TicketStateMove.to_state_id": "Id of the state those tickets move to, one of the states " +
			"state_ids keeps, starting with tkts_, such as tkts_01jx2m8k4t. Find it with " +
			"dimmi_list_ticket_states.",
		"TicketTypeBody.auto_resolve_message": "The message sent in the ticket's conversation when " +
			"a waiting ticket of this type is resolved on its own after auto_resolve_days, such as \"We " +
			"closed this as we did not hear back.\" At most 2000 characters. When updating, send null to " +
			"send no message.",
		"TicketTypeBody.customer_visible": "true to let customers open tickets of this type and " +
			"see them; false to keep the type for the team only. A new type of kind customer starts with " +
			"true, other kinds with false.",
		"TicketTypeBody.kind": "What the type is for: customer (a customer's request tied to a " +
			"conversation), back_office (work for another team) or tracker (gathers many reports of one " +
			"problem), such as \"customer\". It is set when the type is added and never changes; when " +
			"updating, a value sent here is ignored.",
		"UpdateConversationRequestBody.add_tags": "Tags to put on the conversation, or on each one " +
			"in a bulk change, as a list of up to 50 tag ids (tag_...) or tag names; a name no tag has yet " +
			"becomes a new tag. Such as [\"tag_01jx2m8k4t\", \"billing\"].",
		"UpdateConversationRequestBody.assignee_id": "Who the conversation, or each one in a bulk " +
			"change, goes to: a teammate's id starting with mem_, such as mem_01jx2m8k4t (find it with " +
			"dimmi_list_members), me for yourself, or null to hand it to nobody.",
	}
}

/*
sharedTexts8 returns the texts from
UpdateConversationRequestBody.inbox_id on.
*/
func sharedTexts8() map[string]string {
	return map[string]string{
		"UpdateConversationRequestBody.inbox_id": "Id of the inbox to move the conversation, or " +
			"each one in a bulk change, to, starting with inbox_, such as inbox_01jx2m8k4t. Find it with " +
			"dimmi_list_inboxes.",
		"UpdateConversationRequestBody.note": "Only with status closed: a note for the team kept " +
			"with the closing, 1 to 65536 characters, such as Refund sent, nothing more to do.",
		"UpdateConversationRequestBody.priority": "How urgent the conversation is, or each one in " +
			"a bulk change: none, low, medium, high or urgent, such as \"high\".",
		"UpdateConversationRequestBody.remove_tags": "Tags to take off the conversation, or off " +
			"each one in a bulk change, as a list of up to 50 tag ids (tag_...), such as " +
			"[\"tag_01jx2m8k4t\"]. Find them with dimmi_list_tags.",
		"UpdateConversationRequestBody.snooze_option": "Only with status snoozed: the key of one " +
			"of the workspace's own snooze choices, from dimmi_list_snooze_options, at most 40 characters. " +
			"Send exactly one of snoozed_until, snooze_preset or snooze_option.",
		"UpdateConversationRequestBody.snooze_preset": "Only with status snoozed: later_today " +
			"(three hours on), tomorrow or next_week (at nine) or one_week (seven days on, at nine), in " +
			"your time zone, such as \"tomorrow\". Send exactly one of snoozed_until, snooze_preset or " +
			"snooze_option.",
		"UpdateConversationRequestBody.snoozed_until": "Only with status snoozed: when the snooze " +
			"ends, as an RFC 3339 (ISO 8601) time from a minute to a year ahead, such as " +
			"2026-12-01T09:00:00Z. Send exactly one of snoozed_until, snooze_preset or snooze_option.",
		"UpdateConversationRequestBody.spam": "true moves the conversation, or each one in a bulk " +
			"change, to the spam view and out of every other; false brings it back.",
		"UpdateConversationRequestBody.status": "The status to set: open, closed or snoozed, such " +
			"as \"closed\". snoozed also needs exactly one of snoozed_until, snooze_preset or " +
			"snooze_option. A closed conversation cannot be snoozed: reopen it first.",
		"UpdateConversationRequestBody.subject": "The new subject of the conversation, or of every " +
			"conversation in a bulk change, 1 to 255 characters, such as Refund for order 1042.",
		"UpdateConversationRequestBody.team_id": "Id of the team the conversation, or each one in " +
			"a bulk change, goes to, starting with team_, such as team_01jx2m8k4t (find it with " +
			"dimmi_list_teams), or null for no team.",
		"UpdateConversationRequestBody.unread": "true marks the conversation, or each one in a " +
			"bulk change, unread for you alone; false marks it read.",
		"UpdateMemberBody.access_team_ids": "Ids of the teams the conversation_access setting " +
			"refers to, as a list, each starting with team_, such as [\"team_01jx2m8k4t\"]. Find them with " +
			"dimmi_list_teams.",
	}
}

/*
sharedTexts9 returns the texts from
UpdateMemberBody.assign_on_reply on.
*/
func sharedTexts9() map[string]string {
	return map[string]string{
		"UpdateMemberBody.assign_on_reply": "The member's own rule for taking a conversation by " +
			"replying to it: inherit follows the inbox's rule, always assigns what they answer to them, " +
			"never means replying does not assign it to them. When updating yourself it sets your own " +
			"rule. Such as always.",
		"UpdateMemberBody.away_reassign": "Whether, while the member is away, conversations whose " +
			"customers reply go back to the team. true is only accepted when the member is away; going " +
			"online turns it off. When updating yourself it applies to your own conversations. Example: " +
			"true.",
		"UpdateMemberBody.capacity_by_channel": "Object of the most open conversations the member " +
			"holds per channel type: keys are widget, email or api, values are whole numbers from 1 to " +
			"500. It replaces the whole stored object. Such as {\"email\":20,\"widget\":5}.",
		"UpdateMemberBody.conversation_access": "Which conversations the member sees: all, " +
			"assigned (only theirs), teams (their access teams) or all_except_teams. Owners always see " +
			"everything. Nobody can change their own access, so it is refused when updating yourself. Such " +
			"as all.",
		"UpdateMemberBody.conversation_access_unassigned": "Whether a narrow access mode also " +
			"shows the member conversations nobody is assigned to. It is part of conversation access, " +
			"which nobody can change for themselves, so it is refused when updating yourself. Example: " +
			"true.",
		"UpdateMemberBody.conversation_capacity": "Most open conversations the member (or you, " +
			"when updating yourself) holds at once, 1 to 500, such as 20. null for no limit.",
		"UpdateMemberBody.custom_role_id": "Id of a custom role to give the member, starting with " +
			"rol_, such as rol_01jx2m8k4t, or null to take it away. Find it with dimmi_list_roles.",
		"UpdateMemberBody.keep_owner": "Only allowed together with role set to owner. true keeps " +
			"you an owner too after making someone else an owner; otherwise you become an admin. Example: " +
			"true.",
		"UpdateMemberBody.reassign_to_member_id": "With status deactivated: id of the teammate who " +
			"takes the member's conversations, starting with mem_, such as mem_01jx2m8k4t. Find it with " +
			"dimmi_list_members. It has no use when updating yourself, since nobody deactivates " +
			"themselves.",
		"UpdateMemberBody.role": "The member's built-in role: owner, admin or member. Any change " +
			"to or from owner, your own included, needs an owner acting from their own browser in the app " +
			"with a recent confirmation. Such as member.",
		"UpdateMemberBody.status": "Whether the member is active or deactivated. Nobody " +
			"deactivates themselves, so deactivated is refused when updating yourself. Such as active.",
		"WhatsAppTemplateButtonBody.phone_number": "Only for a phone_number button: the number it " +
			"calls, a plus sign, the country code and digits only, 7 to 19 digits and at most 20 " +
			"characters in all, such as +2348012345678.",
		"WhatsAppTemplateButtonBody.text": "The label on the button, 1 to 25 characters, such as " +
			"Track my order.",
	}
}

/*
sharedTexts10 returns the texts from
WhatsAppTemplateButtonBody.type on.
*/
func sharedTexts10() map[string]string {
	return map[string]string{
		"WhatsAppTemplateButtonBody.type": "What the button does: quick_reply (sends its label " +
			"back as a reply), url (opens a link) or phone_number (calls a number). At most two url " +
			"buttons and one phone_number button. Such as \"url\".",
		"WhatsAppTemplateButtonBody.url": "Only for a url button: the full https address it opens, " +
			"with no variables in it, at most 2000 characters, such as https://example.com/orders.",
		"WidgetSettingsBody.allow_anonymous": "Whether visitors may write in the messenger before " +
			"they sign in to your product. true lets anonymous visitors start a session, false refuses " +
			"them. New messengers start with true. Example: true.",
		"WidgetSettingsBody.allowed_origins": "Sites that may load the messenger, as a list, such " +
			"as [\"https://app.company.com\"]; *.company.com covers one label. An empty list allows any " +
			"site.",
		"WidgetSettingsBody.appearance": "The messenger's look, as a JSON object. Keys merge one " +
			"by one and null resets a key; for example theme light, dark or auto, position right or left. " +
			"Such as {\"theme\": \"dark\", \"position\": \"left\"}.",
		"WidgetSettingsBody.attachments_enabled": "Whether visitors may upload files in the " +
			"messenger. true allows attachments, false refuses uploads. New messengers start with true. " +
			"Example: false.",
		"WidgetSettingsBody.changelog_tab_enabled": "Whether the messenger shows a changelog tab. " +
			"The tab only appears when the messenger also has a help center set (help_center_id). Example: " +
			"true.",
		"WidgetSettingsBody.collect_email": "When the messenger asks a visitor for an email: " +
			"never, offline or always. Such as \"offline\".",
		"WidgetSettingsBody.enabled": "true turns the messenger on; false hides it on every site.",
		"WidgetSettingsBody.help_center_id": "Id of the help center the messenger's help tab " +
			"shows, starting with hc_, such as hc_01jx2m8k4t. Find it with dimmi_list_help_centers.",
		"WidgetSettingsBody.help_tab_enabled": "Whether the messenger shows a help tab with " +
			"articles. The tab only appears when the messenger also has a help center set " +
			"(help_center_id). Example: true.",
		"WidgetSettingsBody.home_links": "Replaces the messenger's home screen links, as a list: " +
			"each a title, an https or mailto url and an icon (link, calendar, book, mail, phone, chat, " +
			"video or help). Such as [{\"title\": \"Book a demo\", \"url\": \"https://example.com/demo\", " +
			"\"icon\": \"calendar\"}].",
		"WidgetSettingsBody.inbox_id": "Id of the inbox the messenger's conversations land in, " +
			"starting with inbox_, such as inbox_01jx2m8k4t. Find it with dimmi_list_inboxes.",
		"WidgetSettingsBody.intercom_shim": "Whether the messenger's loader keeps an old Intercom " +
			"snippet's calls working while a customer moves over to dimmi. Off by default. Example: true.",
		"WidgetSettingsBody.locale": "The messenger's language, as a language code, such as en or " +
			"pt-BR. In a preview it is the language the texts are shown in.",
		"WidgetSettingsBody.max_attachment_mb": "Largest file customers may attach, in megabytes, " +
			"a whole number from 1 to 25, such as 10.",
	}
}

/*
sharedTexts11 returns the texts from
WidgetSettingsBody.native_apps_enabled on.
*/
func sharedTexts11() map[string]string {
	return map[string]string{
		"WidgetSettingsBody.native_apps_enabled": "Whether requests without an Origin header, " +
			"which is what native mobile apps send, are accepted even when the messenger has a list of " +
			"allowed sites. Off by default. Example: true.",
		"WidgetSettingsBody.reply_time": "The reply time customers are told: auto, minutes, hours " +
			"or day. Such as \"hours\".",
		"WidgetSettingsBody.require_email_when_offline": "Whether a visitor with no email must " +
			"give one before sending a message while the team is away. Off by default. Example: true.",
		"WidgetSettingsBody.show_availability": "Whether the messenger shows visitors the team's " +
			"availability. Part of the config the messenger loads; new messengers start with true. " +
			"Example: false.",
		"WidgetSettingsBody.show_branding": "Whether the messenger shows the 'Powered by' badge: " +
			"true shows it, false hides it. On dimmi cloud, hiding is refused until the workspace has paid " +
			"an invoice. Example: true.",
		"WidgetSettingsBody.show_team_avatars": "Whether the messenger shows visitors the team's " +
			"avatars. Part of the config the messenger loads; new messengers start with true. Example: " +
			"false.",
		"WidgetSettingsBody.texts": "Object of the messenger's own words: text key to plain text. " +
			"Only known keys such as greeting, welcome_message, offline_message, composer_placeholder, " +
			"launcher_label; each has a length limit. null or \"\" brings back the built-in words. Such as " +
			"{\"greeting\":\"Hi there\",\"launcher_label\":\"Chat\"}.",
		"WidgetSettingsBody.texts_by_locale": "Object of the messenger's words per language: " +
			"language tag to an object with the same keys as texts. null for a language removes all its " +
			"words; up to 40 languages. Such as {\"fr\":{\"greeting\":\"Bonjour\"},\"de\":null}.",
	}
}
