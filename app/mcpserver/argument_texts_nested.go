/*
|--------------------------------------------------------------------------
| Written Argument Texts: Fields Inside Arguments
|--------------------------------------------------------------------------
|
| The fields inside the arguments of the first tools by name, each by
| its path from the argument.
|
*/

package mcpserver

/*
nestedTexts returns the texts written for these tools.
*/
func nestedTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range []map[string]map[string]string{
		nestedTextsFromCreateAiAction(),
		nestedTextsFromCreateChannelWhatsappTemplate(),
		nestedTextsFromCreateContactEvent(),
		nestedTextsFromCreateConversationBulkChange(),
		nestedTextsFromCreateDraft(),
		nestedTextsFromCreateOperatorRoutine(), nestedTextsFromCreateSavedReply(),
	} {
		mergeTexts(out, part)
	}

	return out
}

/*
nestedTextsFromCreateAiAction returns texts of create_ai_action,
create_ai_action_connection and the tools after them.
*/
func nestedTextsFromCreateAiAction() map[string]map[string]string {
	return map[string]map[string]string{
		"create_ai_action": {
			"response_fields.description": "An optional note saying what this value means, such as The order's " +
				"shipping status.",
			"response_fields.name": "The name the AI reads this value under: lowercase letters, digits and " +
				"underscores, starting with a letter, at most 48 characters, unique in the list, such as " +
				"order_status.",
			"response_fields.path": "Dotted path to the value in the other system's JSON response: letters, " +
				"digits, _ and - joined by dots, # to go through a list, at most 200 characters, such as " +
				"data.order.status or items.#.name.",
		},
		"create_ai_action_connection": {
			"auth_config.header_name": "Needed when auth_type is api_key_header: the name of the HTTP header " +
				"the key travels in. Letters, digits and hyphens, at most 64 characters, such as X-Api-Key.",
			"auth_config.prefix": "Only for auth_type bearer: the word put before the token in the " +
				"Authorization header in place of Bearer, such as Token. Left out, it is Bearer.",
			"auth_config.query_param": "Needed when auth_type is api_key_query: the name of the query " +
				"parameter the key travels in. Letters, digits and hyphens only, at most 64 characters, such " +
				"as apikey.",
		},
		"create_audience_evaluation": {
			"context.country": "The two-letter country code the visitor's session would report, such as PT. It " +
				"is read in capitals.",
			"context.device": "The device the visitor would be on: desktop, mobile or tablet, such as " +
				"\"mobile\".",
			"context.locale": "The language the visitor's browser would report, as a language tag of at most " +
				"35 characters, such as en-US.",
			"context.page_url": "The full address of the page the visitor would be on, with its host, at most " +
				"2048 characters, such as https://app.example.com/pricing?plan=pro.",
		},
		"create_channel_whatsapp_template": {
			"buttons.phone_number": "Only for a phone_number button: the number it calls, with + and its " +
				"country code, digits only, at most 20 characters, such as +2348012345678.",
			"buttons.text": "The label on the button, 1 to 25 characters, such as Track my order.",
		},
	}
}

/*
nestedTextsFromCreateChannelWhatsappTemplate returns texts of
create_channel_whatsapp_template,
create_contact and the tools after them.
*/
func nestedTextsFromCreateChannelWhatsappTemplate() map[string]map[string]string {
	return map[string]map[string]string{
		"create_channel_whatsapp_template": {
			"buttons.type": "What the button does: quick_reply (sends its label back as a reply), url (opens a " +
				"link) or phone_number (calls a number). At most two url buttons and one phone_number button. " +
				"Such as \"url\".",
			"buttons.url": "Only for a url button: the full https address it opens, with no variables in it, " +
				"at most 2000 characters, such as https://example.com/orders.",
		},
		"create_contact": {
			"company.custom_attributes": "The company's custom attributes, as an object of attribute key to " +
				"value, merged over the ones it has, such as {\"plan\": \"enterprise\"}.",
			"company.domain": "The company's domain name, at most 255 characters, such as acme.com. The " +
				"company is found by external_id first, then by this; one of the two is needed.",
			"company.external_id": "Your own system's id of the company, 1 to 255 characters, such as org_512. " +
				"The company is found by this first, then by domain; a new one is added when none matches.",
			"company.name": "The company's name, 1 to 255 characters, such as Acme Inc.",
		},
		"create_contact_event": {
			"events.contact_id": "Id of the contact who did it, starting with ctc_, such as ctc_01jx2m8k4t. " +
				"Name the person by exactly one of contact_id, external_id and email.",
			"events.email": "Email address of the contact who did it, such as ana@example.com. Name the person " +
				"by exactly one of contact_id, external_id and email.",
			"events.external_id": "Your own system's id of the contact who did it, such as user_8421. Name the " +
				"person by exactly one of contact_id, external_id and email.",
			"events.idempotency_key": "A unique text you make up for this event, at most 128 characters, so " +
				"sending it again within 48 hours does not record it twice, such as order-1042-paid.",
			"events.metadata": "Details of the event, as a flat object of at most 20 keys whose values are " +
				"text, numbers or true/false, 2 KB in all, such as {\"plan\": \"pro\", \"amount\": 49}.",
			"events.name": "The event's name: 1 to 64 lowercase letters, digits, underscores, dots, colons or " +
				"hyphens, such as order_placed.",
		},
	}
}

/*
nestedTextsFromCreateContactEvent returns texts of create_contact_event,
create_conversation and the tools after them.
*/
func nestedTextsFromCreateContactEvent() map[string]map[string]string {
	return map[string]map[string]string{
		"create_contact_event": {
			"events.occurred_at": "When it happened, as an ISO 8601 time, such as 2026-11-20T14:30:00Z. Left " +
				"out, it is now. Times over 30 days back or over an hour ahead are moved into that range.",
		},
		"create_conversation": {
			"contact.email": "The customer's email address, at most 320 characters, such as ana@example.com. " +
				"Used to find them when no id or external_id is given; a new contact is added when none " +
				"matches.",
			"contact.external_id": "Your own system's id of the customer, 1 to 255 characters, such as " +
				"user_8421. Used to find them when no id is given; a new contact is added when none matches.",
			"contact.id": "Id of an existing contact, starting with ctc_, such as ctc_01jx2m8k4t. Find it with " +
				"dimmi_list_contacts. It wins over external_id and email.",
			"contact.name": "The name to give the customer when they are new, at most 255 characters, such as " +
				"Ana Silva.",
		},
		"create_conversation_bulk_change": {
			"changes.add_tags": "Tags to put on each conversation, as a list of up to 50 tag ids (tag_...) or " +
				"tag names; a name no tag has yet becomes a new tag. Such as [\"tag_01jx2m8k4t\", " +
				"\"billing\"].",
			"changes.assignee_id": "Who the conversations go to: a teammate's id starting with mem_, such as " +
				"mem_01jx2m8k4t (find it with dimmi_list_members), me for yourself, or null to hand them to " +
				"nobody.",
			"changes.inbox_id": "Id of the inbox to move the conversations to, starting with inbox_, such as " +
				"inbox_01jx2m8k4t. Find it with dimmi_list_inboxes.",
			"changes.note": "Only with status closed: a note for the team kept with the closing, 1 to 65536 " +
				"characters, such as Refund sent, nothing more to do.",
			"changes.priority": "How urgent the conversations are: none, low, medium, high or urgent, such as " +
				"\"high\".",
			"changes.remove_tags": "Tags to take off each conversation, as a list of up to 50 tag ids, such as " +
				"[\"tag_01jx2m8k4t\"]. Find them with dimmi_list_tags.",
			"changes.snooze_option": "Only with status snoozed: the key of one of the workspace's own snooze " +
				"choices, from dimmi_list_snooze_options. Send exactly one of snoozed_until, snooze_preset or " +
				"snooze_option.",
		},
	}
}

/*
nestedTextsFromCreateConversationBulkChange returns texts of
create_conversation_bulk_change,
create_data_export.
*/
func nestedTextsFromCreateConversationBulkChange() map[string]map[string]string {
	return map[string]map[string]string{
		"create_conversation_bulk_change": {
			"changes.snooze_preset": "Only with status snoozed: later_today, tomorrow, next_week or one_week, " +
				"such as \"tomorrow\". Send exactly one of snoozed_until, snooze_preset or snooze_option.",
			"changes.snoozed_until": "Only with status snoozed: when the snooze ends, as an ISO 8601 time, " +
				"such as 2026-12-01T09:00:00Z. Send exactly one of snoozed_until, snooze_preset or " +
				"snooze_option.",
			"changes.spam": "true moves the conversations to the spam view and out of every other; false " +
				"brings them back.",
			"changes.status": "The status to set: open, closed or snoozed, such as \"closed\". snoozed also " +
				"needs one of snoozed_until, snooze_preset or snooze_option.",
			"changes.subject": "The new subject of every conversation changed, 1 to 255 characters, such as " +
				"Refund for order 1042.",
			"changes.team_id": "Id of the team the conversations go to, starting with team_, such as " +
				"team_01jx2m8k4t (find it with dimmi_list_teams), or null for no team.",
			"changes.unread": "true marks the conversations unread for you alone; false marks them read.",
		},
		"create_data_export": {
			"selection.conditions": "Contact exports only: the same conditions as the contact list takes, a " +
				"JSON list written as text, at most 8000 characters, such as " +
				"[{\"field\":\"email\",\"op\":\"contains\",\"value\":\"@acme.com\"}].",
			"selection.filter": "Filters as an object of filter name to text value, the same names the " +
				"matching list takes; for kind conversation_facts the report's from, to, inbox, team and so " +
				"on. Such as {\"status\": \"closed\"}.",
			"selection.ids": "The exact records to export, as a list of up to 10000 ids: conversation ids " +
				"(conv_...) for conversations, contact ids (ctc_...) for contacts. When given, the filters are " +
				"not used. Such as [\"conv_01jx2m8k4t\"].",
			"selection.q": "Words to search for, so only the records that match are exported, such as refund.",
			"selection.view_id": "Conversation exports only: the list to export, as the conversation list's " +
				"view filter takes it, such as a saved view's id cvw_01jx2m8k4t (from " +
				"dimmi_list_conversation_views) or all.",
		},
	}
}

/*
nestedTextsFromCreateDraft returns texts of create_draft, create_invitation and
the tools after them.
*/
func nestedTextsFromCreateDraft() map[string]map[string]string {
	return map[string]map[string]string{
		"create_draft": {
			"recipients.bcc": "Who gets a hidden copy when the draft is sent, as a list of up to 50 texts, " +
				"such as [\"audit@example.com\"]. Kept with the draft as given.",
			"recipients.cc": "Who gets a copy when the draft is sent, as a list of up to 50 texts, such as " +
				"[\"lee@example.com\"]. Kept with the draft as given.",
			"recipients.channel_connection_id": "The id of the channel the draft is to be sent through, at " +
				"most 64 characters, such as chn_01jx2m8k4t. Kept with the draft as given.",
			"recipients.subject": "The subject line to use when the draft is sent, at most 998 characters, " +
				"such as Your refund request.",
			"recipients.to": "Who the draft is addressed to when it is sent, as a list of up to 50 texts, such " +
				"as [\"ana@example.com\"]. Kept with the draft as given.",
		},
		"create_invitation": {
			"invitations.custom_role_id": "Id of the custom role the person starts with, starting with rol_, " +
				"such as rol_01jx2m8k4t. Find it with dimmi_list_roles. Its base role then takes the place of " +
				"role.",
			"invitations.email": "The email address the invitation is sent to, such as lee@example.com.",
			"invitations.role": "The role the person joins with: admin or member, such as \"member\". Owners " +
				"are never invited.",
			"invitations.team_ids": "Ids of the teams the person joins on accepting, as a list of up to 50, " +
				"such as [\"team_01jx2m8k4t\"]. Find them with dimmi_list_teams.",
		},
		"create_operator_routine": {
			"delivery.email_member_ids": "Ids of the teammates told about runs by email, as a list of up to " +
				"100, such as [\"mem_01jx2m8k4t\"]. Find them with dimmi_list_members.",
			"delivery.in_app_member_ids": "Ids of the teammates told about runs inside dimmi, as a list of up " +
				"to 100, such as [\"mem_01jx2m8k4t\"]. Find them with dimmi_list_members.",
			"delivery.slack_destination_ids": "Ids of the Slack notification destinations told about runs, as " +
				"a list of up to 20, such as [\"ndst_01jx2m8k4t\"]. Find them with " +
				"dimmi_list_notification_destinations.",
			"scope.github": "The GitHub connection the routine reads, as an object with installation_id and " +
				"repositories (both needed), plus optional base_branch and ignore_labels, such as " +
				"{\"installation_id\": \"itg_01jx2m8k4t\", \"repositories\": [\"acme/app\"]}.",
		},
	}
}

/*
nestedTextsFromCreateOperatorRoutine returns texts of create_operator_routine,
create_procedure_test_case.
*/
func nestedTextsFromCreateOperatorRoutine() map[string]map[string]string {
	return map[string]map[string]string{
		"create_operator_routine": {
			"scope.github.base_branch": "The branch pull requests are merged into; the routine lists the pull " +
				"requests merged into it. At most 200 characters, such as main.",
			"scope.github.ignore_labels": "Labels whose pull requests the routine skips, as a list of up to 50 " +
				"label names, matched without regard to capitals, such as [\"no-docs\", \"chore\"].",
			"scope.github.installation_id": "Id of the connected GitHub integration, starting with itg_, such " +
				"as itg_01jx2m8k4t. Find it with dimmi_list_integrations.",
			"scope.github.repositories": "The only repositories the routine may read, as a list of 1 to 20 " +
				"names written owner/repo, such as [\"acme/app\"].",
			"scope.help_center_ids": "Ids of the help centers the routine works on, as a list of up to 20, " +
				"such as [\"hc_01jx2m8k4t\"]. Find them with dimmi_list_help_centers.",
		},
		"create_procedure_test_case": {
			"messages.confirm": "true to first confirm the writes left waiting by the turn before, as the " +
				"customer pressing Confirm on the card would; false or left out to only send the text.",
			"messages.text": "What the made-up customer writes in this turn, at most 4000 characters, such as " +
				"I want a refund for order 1042.",
			"persona.attributes": "The made-up customer's custom attributes, as an object of attribute key to " +
				"value. They count as trusted. Such as {\"plan\": \"pro\"}.",
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

/*
nestedTextsFromCreateSavedReply returns texts of create_saved_reply.
*/
func nestedTextsFromCreateSavedReply() map[string]map[string]string {
	return map[string]map[string]string{
		"create_saved_reply": {
			"actions.assignee_id": "Id of the teammate the conversation is assigned to when the reply is sent, " +
				"starting with mem_, such as mem_01jx2m8k4t. Find it with dimmi_list_members.",
			"actions.priority": "The priority the conversation gets when the reply is sent: none, low, medium, " +
				"high or urgent, such as \"high\".",
			"actions.snooze_preset": "Only when status is snoozed: when the snooze ends, one of later_today, " +
				"tomorrow, next_week or one_week, such as \"tomorrow\".",
			"actions.status": "The status the conversation gets when the reply is sent: open, closed or " +
				"snoozed, such as \"closed\".",
			"actions.tag_ids": "Ids of the tags added to the conversation when the reply is sent, as a list of " +
				"up to 10, such as [\"tag_01jx2m8k4t\"]. Find them with dimmi_list_tags.",
			"actions.team_id": "Id of the team the conversation goes to when the reply is sent, starting with " +
				"team_, such as team_01jx2m8k4t. Find it with dimmi_list_teams.",
		},
	}
}
