/*
|--------------------------------------------------------------------------
| Written Schema Texts: Fields Several Operations Share
|--------------------------------------------------------------------------
|
| A request field several operations share through one schema can carry
| only one description, so each is written once here to be true for all
| of them, saying where creating and updating differ. Each is kept by
| the schema that owns the field, then the field.
|
*/

package mcpserver

import "maps"

/*
sharedTexts returns the one text of each field several operations
share, by schema and field joined with a dot.
*/
func sharedTexts() map[string]string {
	out := map[string]string{}
	for _, part := range []map[string]string{
		sharedTextsFromAIActionBodyStatus(),
		sharedTextsFromChangelogLabelBodyPosition(),
		sharedTextsFromContactCompanies(),
		sharedTextsFromFeedbackBoardBodySlug(),
		sharedTextsFromOperatorRoutineDeliveryInputEmailMemberIDs(),
		sharedTextsFromSavedReplyMacroInputAssigneeID(),
		sharedTextsFromStoreSocialChannelRequestBodyAccountID(),
		sharedTextsFromTicketLinkBodyConversationID(),
		sharedTextsFromUpdateConversationRequestBodyInboxID(),
		sharedTextsFromUpdateMemberBodyAssignOnReply(),
		sharedTextsFromWhatsAppTemplateButtonBodyType(),
		sharedTextsFromWidgetSettingsBodyNativeAppsEnabled(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
sharedTextsFromAIActionBodyStatus returns the texts from
AIActionBody.status on.
*/
func sharedTextsFromAIActionBodyStatus() map[string]string {
	return map[string]string{
		"AIActionBody.status": "When creating: has no effect, a new action always starts as a " +
			"draft and goes live through dimmi_publish_ai_action, so leave it out. When updating: " +
			"\"disabled\" or \"published\" switches an action published before off or back on; " +
			"\"published\" is refused for an action never published. Such as \"disabled\".",
		"AIActionConnectionBody.auth_type": "How the connection signs its requests: \"none\", " +
			"\"bearer\", \"api_key_header\", \"api_key_query\" or \"basic\". api_key_header also needs " +
			"auth_config.header_name and api_key_query needs auth_config.query_param; \"none\" clears the " +
			"stored secret. Such as \"bearer\".",
		"AIActionConnectionBody.base_url": "The https address every action's path is added to; " +
			"requests can never leave it. A full https address with no credentials, query or fragment, at " +
			"most 512 characters, such as https://api.example.com.",
		"AIActionConnectionBody.kind": "The connection's kind: \"http\" for an HTTP API or \"mcp\" " +
			"for an MCP server. When creating: needed, and fixed from then on. When updating: it does not " +
			"change, so leave it out.",
		"AIActionConnectionBody.status": "\"active\", or \"disabled\" to stop every action on the " +
			"connection. When creating: it has no effect, a new connection always starts active, so leave " +
			"it out. When updating: switches the connection off or back on; " +
			"dimmi_update_ai_action_connection is the way to disable one.",
		"AIActionConnectionBody.token": "The secret bearer token or API key, as text, at most 4096 " +
			"characters. It is only written: stored encrypted and never returned or shown again.",
		"AIActionConnectionBody.username": "The user name for basic auth, sent together with " +
			"password, at most 256 characters, such as api-user.",
		"AIActionConnectionBody.variables": "Named values the connection's actions can use in " +
			"their templates as {connection.name}, as an object of name to text. At most 20; names are " +
			"lowercase letters, digits and _, up to 40 characters; values up to 256 characters. Such as " +
			"{\"account_id\": \"acct_42\", \"region\": \"eu\"}.",
		"AudienceBody.lock_version": "The lock_version the audience had when you read it, such as " +
			"3. It stops you overwriting a teammate's newer save.",
		"AudienceBody.name": "The name of the audience, unique in the workspace, at most 80 " +
			"characters, such as Paying customers.",
		"AudienceBody.rules": "The audience as JSON: {\"visitors\": everyone, identified, leads, " +
			"anonymous or anonymous_and_leads, \"rules\": a tree of all, any and not groups over leaves " +
			"{field, op, value}}. Fields come from dimmi_list_audience_fields, such as " +
			"contact.custom.plan. Four levels, 50 leaves, 8 KB at most. When creating, empty means " +
			"everyone.",
		"ChangelogLabelBody.name": "The name of the changelog label, 1 to 40 characters, unique in " +
			"the changelog, such as Improvement.",
	}
}

/*
sharedTextsFromChangelogLabelBodyPosition returns the texts from
ChangelogLabelBody.position on.
*/
func sharedTextsFromChangelogLabelBodyPosition() map[string]string {
	return map[string]string{
		"ChangelogLabelBody.position": "Where the label sits in the list, counting from 0, lowest " +
			"first, such as 2.",
		"CollectionBody.parent_id": "Id of the parent collection, starting with col_, such as " +
			"col_01jx2m8k4t. When creating: leave it out for a top-level collection. When updating: it " +
			"moves the collection under that parent, and an empty text moves it to the top level.",
		"CompanyBody.custom_attributes": "The company's custom attribute values, as an object of " +
			"attribute key to value, with keys as dimmi_list_attribute_definitions defines them for " +
			"companies. When the company already exists they are merged over the ones it has. Such as " +
			"{\"plan\": \"enterprise\"}.",
		"CompanyBody.domain": "The company's domain name, at most 255 characters, such as " +
			"acme.com. When a company is looked up (a contact's company, or an upsert), it is found by " +
			"external_id first, then by this; a contact's company needs one of the two.",
		"CompanyBody.external_id": "Your own system's id of the company, 1 to 255 characters, such " +
			"as org_512. When a company is looked up (a contact's company, or an upsert), it is found by " +
			"this first, then by domain; a new one is added when none matches.",
		"CompanyBody.name": "The company's name, 1 to 255 characters, such as Acme Inc.",
		"ConnectionAuth.header_name": "Needed when auth_type is api_key_header: the name of the " +
			"HTTP header the key travels in. 1 to 64 letters, digits or hyphens, such as X-Api-Key.",
		"ConnectionAuth.prefix": "Only for auth_type bearer: the word put before the token in the " +
			"Authorization header in place of Bearer, such as Token. Left out, it is Bearer.",
		"ConnectionAuth.query_param": "Needed when auth_type is api_key_query: the name of the " +
			"query parameter the key travels in. 1 to 64 letters, digits or hyphens only (no underscore), " +
			"such as apikey.",
		"DraftRecipientsBody.bcc": "Who gets a hidden copy when the draft is sent, as a list of up " +
			"to 50 addresses, such as [\"audit@example.com\"]. Saved with the draft as given.",
		"DraftRecipientsBody.cc": "Who gets a copy when the draft is sent, as a list of up to 50 " +
			"addresses, such as [\"lee@example.com\"]. Saved with the draft as given.",
		"DraftRecipientsBody.channel_connection_id": "Id of the connected channel the draft is " +
			"meant to go out through, at most 64 characters, such as chn_01jx2m8k4t. Saved with the draft " +
			"as given.",
		"DraftRecipientsBody.subject": "The subject line saved with the draft and used when it is " +
			"sent, at most 998 characters, such as Your refund request.",
		"DraftRecipientsBody.to": "Who the draft is addressed to when it is sent, as a list of up " +
			"to 50 addresses, such as [\"ana@example.com\"]. Saved with the draft as given.",
		"FeedbackBoardBody.approval_required": "Whether what customers post on this board is held " +
			"until the team approves it: true holds it, false shows it right away. Example: true.",
		"FeedbackBoardBody.name": "The name of the feedback board, 1 to 60 characters, such as " +
			"Feature requests.",
	}
}

/*
sharedTextsFromFeedbackBoardBodySlug returns the texts from
FeedbackBoardBody.slug on.
*/
func sharedTextsFromFeedbackBoardBodySlug() map[string]string {
	return map[string]string{
		"FeedbackBoardBody.slug": "The part of the board's address on the portal that names it, in " +
			"lowercase words joined by hyphens, at most 60 characters, such as feature-requests. When " +
			"creating, it is taken from the name when left out.",
		"FeedbackBoardBody.visibility": "Who can see the board: public (everyone), private " +
			"(signed-in customers only) or internal (the team alone). Such as private.",
		"FeedbackStatusBody.category": "What the feedback status means, in the order work moves: " +
			"open, planned, in_progress, done (shipped) or closed. New requests start in the first open " +
			"status; closed ones leave the public roadmap. Such as planned.",
		"FeedbackStatusBody.color": "A color as a hex code, such as #2563eb.",
		"FeedbackStatusBody.name": "The name of the feedback status, which readers see, 1 to 40 " +
			"characters, such as Planned.",
		"FeedbackStatusBody.roadmap": "Whether the public roadmap shows this status as a column: " +
			"true shows it, false keeps it off. A status in the closed category is never shown even when " +
			"true. Example: true.",
		"Holiday.date": "The day the team is closed all day, written YYYY-MM-DD, such as " +
			"2026-12-25.",
		"Holiday.name": "What the holiday is called, such as Christmas Day. It may be left out.",
		"Holiday.yearly": "true to close on the same month and day every year; false or left out " +
			"for that one date only. A yearly February 29 closes only in leap years.",
		"HomeLinkBody.icon": "The icon shown beside the link: link, calendar, book, mail, phone, " +
			"chat, video or help. Left out, it is link. Such as \"calendar\".",
		"HomeLinkBody.title": "The words the link shows on the messenger's home screen, 1 to 40 " +
			"characters, such as Book a demo.",
		"HomeLinkBody.url": "Where the link goes: an https:// address or a mailto: address, at " +
			"most 2048 characters, such as https://example.com/demo or mailto:help@example.com.",
		"ImportColumn.column": "Which column of the uploaded file this is, counted in the header " +
			"from 0, so the first column is 0. It must be a column the file has, such as 2.",
		"ImportColumn.field": "The field the column fills: email, name, external_id, phone, " +
			"company_name, company_domain or tags for contacts; name, domain, external_id or tags for " +
			"companies; or attribute.<key> for a custom attribute, such as attribute.plan.",
		"ImportPlanRequestBody.mapping": "A list of {column, field}: column is the index in the " +
			"header from 0; field is email, name, external_id, phone, company_name, company_domain or tags " +
			"for contacts, name, domain, external_id or tags for companies, or attribute.<key>. For the " +
			"real run, send the same mapping as the dry run.",
		"ImportPlanRequestBody.mode": "What happens to records already there: update changes them, " +
			"skip leaves them alone. Such as \"skip\".",
	}
}

/*
sharedTextsFromOperatorRoutineDeliveryInputEmailMemberIDs returns the texts from
OperatorRoutineDeliveryInput.email_member_ids on.
*/
func sharedTextsFromOperatorRoutineDeliveryInputEmailMemberIDs() map[string]string {
	return map[string]string{
		"OperatorRoutineDeliveryInput.email_member_ids": "Ids of the teammates told about the " +
			"routine's runs by email, as a list of up to 100, each starting with mem_, such as " +
			"[\"mem_01jx2m8k4t\"]. Find them with dimmi_list_members.",
		"OperatorRoutineDeliveryInput.in_app_member_ids": "Ids of the teammates told about the " +
			"routine's runs inside dimmi, as a list of up to 100, each starting with mem_, such as " +
			"[\"mem_01jx2m8k4t\"]. Find them with dimmi_list_members.",
		"OperatorRoutineDeliveryInput.slack_destination_ids": "Ids of the Slack notification " +
			"destinations told about the routine's runs, as a list of up to 20, each starting with ndst_, " +
			"such as [\"ndst_01jx2m8k4t\"]. Find them with dimmi_list_notification_destinations.",
		"OperatorRoutineGitHubScopeInput.base_branch": "The branch pull requests are merged into; " +
			"the routine reads the pull requests merged into it. At most 200 characters, such as main.",
		"OperatorRoutineGitHubScopeInput.ignore_labels": "Labels whose pull requests the routine " +
			"skips, as a list of up to 50 label names, matched without regard to capitals, such as " +
			"[\"no-docs\", \"chore\"]. A pull request carrying any of them is skipped.",
		"OperatorRoutineGitHubScopeInput.installation_id": "Id of a connected, active GitHub " +
			"integration in this workspace, starting with itg_, such as itg_01jx2m8k4t. Find it with " +
			"dimmi_list_integrations.",
		"OperatorRoutineGitHubScopeInput.repositories": "The only repositories the routine may " +
			"read, as a list of 1 to 20 names written owner/repo, such as [\"acme/app\"].",
		"OperatorRoutineScopeInput.help_center_ids": "Ids of the help centers the routine works " +
			"and drafts in, as a list of up to 20, each starting with hc_, such as [\"hc_01jx2m8k4t\"]. " +
			"Find them with dimmi_list_help_centers.",
		"ResponseField.description": "An optional short note telling the AI what this value means, " +
			"such as The order's shipping status.",
		"ResponseField.name": "The name the AI reads this value under: lowercase letters, digits " +
			"and underscores, starting with a letter, at most 48 characters, used only once in the list, " +
			"such as order_status.",
		"ResponseField.path": "Dotted path to the value in the other system's JSON response: " +
			"letters, digits, _ and - joined by dots, a number for a list position and # to take every " +
			"item, at most 200 characters, such as data.order.status or items.#.name.",
		"SLAPolicyBody.pause_on_waiting_customer": "true to pause the clocks while a ticket is in " +
			"a state waiting on the customer, or a conversation is snoozed; false to keep them running. A " +
			"new policy starts with true.",
		"SLAPolicyBody.pause_on_waiting_team": "true to pause the clocks while a ticket is in a " +
			"state waiting on another team; false to keep them running. A new policy starts with false.",
		"SLAPolicyBody.priorities": "The ticket priorities this policy applies to, as a list of " +
			"none, low, medium, high, urgent, such as [\"high\", \"urgent\"]. An empty list means any " +
			"priority. A policy with no priorities and no ticket types is never applied on its own. When " +
			"updating, sending it replaces the list.",
	}
}

/*
sharedTextsFromSavedReplyMacroInputAssigneeID returns the texts from
SavedReplyMacroInput.assignee_id on.
*/
func sharedTextsFromSavedReplyMacroInputAssigneeID() map[string]string {
	return map[string]string{
		"SavedReplyMacroInput.assignee_id": "Id of the teammate the conversation is assigned to " +
			"when the reply is sent, starting with mem_, such as mem_01jx2m8k4t. Find it with " +
			"dimmi_list_members.",
		"SavedReplyMacroInput.priority": "The priority the conversation gets when the reply is " +
			"sent: none, low, medium, high or urgent, such as \"high\".",
		"SavedReplyMacroInput.snooze_preset": "Only when status is snoozed: when the snooze ends, " +
			"one of later_today, tomorrow, next_week or one_week, such as \"tomorrow\".",
		"SavedReplyMacroInput.status": "The status the conversation gets when the reply is sent: " +
			"open, closed or snoozed, such as \"closed\". With snoozed, also give snooze_preset.",
		"SavedReplyMacroInput.tag_ids": "Ids of the tags added to the conversation when the reply " +
			"is sent, as a list of up to 10, each starting with tag_, such as [\"tag_01jx2m8k4t\"]. Find " +
			"them with dimmi_list_tags.",
		"SavedReplyMacroInput.team_id": "Id of the team the conversation is assigned to when the " +
			"reply is sent, starting with team_, such as team_01jx2m8k4t. Find it with dimmi_list_teams.",
		"ShowWhenBody.equals": "The values of the other field that make this field show, as a list " +
			"of up to 50 texts, such as [\"refund\", \"exchange\"]. Left out or empty, this field shows " +
			"whenever the other field has any value.",
		"ShowWhenBody.field": "Key of another field of the same ticket type that this field " +
			"depends on: it must have a value for this field to show. At most 40 characters, such as " +
			"reason.",
		"SimPersonaInput.attributes": "The made-up customer's custom attributes, as an object of " +
			"attribute key to value. They count as trusted. Such as {\"plan\": \"pro\"}.",
		"SimPersonaInput.channel": "The channel the made-up customer writes from: widget, email, " +
			"slack, whatsapp, instagram, facebook or api. Left out or empty, it is widget. Such as " +
			"\"email\".",
		"SimPersonaInput.company": "The custom attributes of the made-up customer's company, as an " +
			"object of attribute key to value, such as {\"plan\": \"enterprise\"}.",
		"SimPersonaInput.email": "The made-up customer's email address, at most 320 characters, " +
			"such as ada@example.com.",
		"SimPersonaInput.external_id": "Your own system's id of the made-up customer, at most 200 " +
			"characters, such as user_8421. Giving one makes them a known user, not a lead.",
		"SimPersonaInput.inbox_id": "Id of the inbox the made-up conversation is placed in, " +
			"starting with inbox_, such as inbox_01jx2m8k4t. Find it with dimmi_list_inboxes.",
		"SimPersonaInput.name": "The made-up customer's name, at most 200 characters, such as Ada " +
			"Lovelace.",
		"SimPersonaInput.verified": "true when the made-up customer is signed in, so their " +
			"identity and email count as verified; false or left out for an unverified visitor.",
	}
}

/*
sharedTextsFromStoreSocialChannelRequestBodyAccountID returns the texts from
StoreSocialChannelRequestBody.account_id on.
*/
func sharedTextsFromStoreSocialChannelRequestBodyAccountID() map[string]string {
	return map[string]string{
		"StoreSocialChannelRequestBody.account_id": "The id of the Facebook Page or Instagram " +
			"account to connect, taken from the accounts list the Facebook or Instagram sign-in step " +
			"returned together with the grant. Such as 104857600123456 for a Page or 17841400123456789 for " +
			"an Instagram account.",
		"StoreSocialChannelRequestBody.grant": "The grant string the sign-in step returned: POST " +
			"/channels/facebook/signups for a Facebook Page, POST /channels/instagram/signups for an " +
			"Instagram account. It names that sign-in and expires after 15 minutes. Such as " +
			"k3f9x0q2m7v1b8n4c6z5a1s2d3f4g5h6.",
		"TestCaseBody.assertions": "Checks after the test, at most 30 objects with a type: " +
			"procedure_matched, handed_off, action_called or action_not_called (action), action_order " +
			"(action, before), branch_taken (step_id, branch then or else), reply_contains or " +
			"reply_not_contains (text), llm_rubric (rubric). Such as [{\"type\": \"action_called\", " +
			"\"action\": \"aac_abc123\"}].",
		"TestCaseBody.messages": "The customer's scripted messages, oldest first, 1 to 12, each an " +
			"object with text (1 to 4,000 characters) and optional confirm (true first confirms the writes " +
			"waiting from the turn before; text may then be empty). Such as [{\"text\": \"I want a refund " +
			"for order 1042\"}, {\"text\": \"\", \"confirm\": true}].",
		"TestMessageInput.confirm": "true to first confirm the writes left waiting by the turn " +
			"before, as the customer pressing Confirm on the card would; with true the text may be empty. " +
			"false or left out to only send the text.",
		"TestMessageInput.text": "What the made-up customer writes in this turn, 1 to 4000 " +
			"characters, such as I want a refund for order 1042. It may be empty only when confirm is " +
			"true.",
		"TicketFieldBody.ai_hint": "A short note telling the AI how to fill this field, such as " +
			"\"The order number from the customer's message, like A-1042\". At most 500 characters. When " +
			"updating, send null to clear it.",
		"TicketFieldBody.data_type": "What kind of value the field holds: short_text, long_text, " +
			"number (whole), decimal, currency, boolean, date, single_choice, multiple_choice, file, " +
			"member or company, such as \"single_choice\". It is set when the field is added and never " +
			"changes; when updating, a value sent here is ignored.",
		"TicketFieldBody.description": "The description of the ticket field, at most 300 " +
			"characters, such as The order this request is about.",
		"TicketFieldBody.key": "The short name field values are stored under: lowercase letters, " +
			"digits and underscores, starting with a letter, at most 40 characters, unique within the " +
			"type, such as \"order_number\". It is set when the field is added and never changes; when " +
			"updating, a value sent here is ignored.",
	}
}
