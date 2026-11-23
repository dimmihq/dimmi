/*
|--------------------------------------------------------------------------
| Written Schema Texts: ConnectedApp To CountViewsAllStruct
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTexts36 returns the texts from ConnectedApp on.
*/
func schemaTexts36() map[string]map[string]string {
	return map[string]map[string]string{
		"ConnectedApp": {
			"last_used_at": "When the app last used its access.",
			"name":         "The app's name.",
			"scopes":       "The permissions the person granted the app, such as conversations:read.",
			"workspace_id": "Id of the workspace the app was given access to, starting with ws_. " +
				"Absent when the access names no workspace.",
			"workspace_name": "The name of the workspace the app was given access to. Empty when the " +
				"access names no workspace.",
		},
		"ConnectionRef": {
			"status": "The state of the channel connection behind the address: pending, active, error " +
				"or disconnected.",
		},
		"Contact": {
			"avatar_url": "URL of the contact's picture. Null when they have none.",
			"blocked":    "True when the team has blocked this contact.",
			"custom_attributes": "The contact's custom attributes as an object of names and values, " +
				"such as {\"plan\": \"pro\"}. An empty object when there are none.",
			"email": "The contact's email address. Null when unknown. Not unique: a lead may type any " +
				"address, so check email_verified.",
			"email_verified": "True only when a trusted source proved the email belongs to this " +
				"contact; an address a lead typed stays false.",
			"external_id":   "The contact's id in your own system. Null when none was given.",
			"first_seen_at": "When the contact was first seen. Null when never recorded.",
			"kind": "How well the contact is known: user when your app identified them (verified token " +
				"or API), lead for anyone else.",
			"last_seen_at": "When the contact was last seen. Null when never recorded.",
			"locale": "The contact's language as a locale code, such as en or pt-BR. Null when " +
				"unknown.",
			"location": "Where the contact is, as a JSON object. Null when unknown.",
			"name":     "The contact's name. Null when unknown.",
			"notes":    "Free-text notes the team keeps about the contact. Null when there are none.",
			"phone":    "The contact's phone number. Null when unknown.",
		},
	}
}

/*
schemaTexts37 returns the texts from Contact on.
*/
func schemaTexts37() map[string]map[string]string {
	return map[string]map[string]string{
		"Contact": {
			"tags": "The tags on the contact, each with its id and name. Empty in plain lists that do " +
				"not load tags.",
			"timezone": "The contact's IANA time zone, such as Europe/Berlin, for reading times in " +
				"their local time. Null when unknown.",
		},
		"ContactDuplicate": {
			"reasons": "Why this contact looks like the same person: any of same_email, same_phone, " +
				"same_whatsapp and same_name_company.",
		},
		"ContactErasure": {
			"completed_at": "When the contact's data was wiped. Null until the erasure completes.",
			"contact_id": "Id of the contact being erased, starting with ctc_. The contact is hidden " +
				"as soon as the request is made.",
			"counts": "After the wipe, how many records of each kind were erased, keyed by kind, such " +
				"as {\"messages\": 42, \"attachments\": 3}. An empty object before that.",
			"identities_conflicted": "Given only when an erasure is canceled: the contact's identities " +
				"that could not be given back, each as type:value, such as email:ada@example.com.",
			"scheduled_for": "When the grace period ends and the data is wiped, unless the request is " +
				"canceled or a retention hold keeps it.",
			"status": "Where the request stands: pending (waiting out the grace period), pending_hold " +
				"(kept by a retention hold), completed or canceled.",
		},
		"ContactExport": {
			"contact_id": "Id of the contact whose data is exported, starting with ctc_.",
			"expires_at": "When the export file stops being downloadable, a day after it became ready. " +
				"Null until it is ready.",
			"include_notes": "True when the team's internal notes are part of the export.",
			"size_bytes":    "Size of the export file in bytes. Null until it is built.",
			"status":        "Where the export stands: pending, running, ready, failed or expired.",
			"url": "The API path the ready export downloads from, such as " +
				"/api/v1/contacts/ctc_.../exports/exp_.../download. Null unless status is ready.",
		},
	}
}

/*
schemaTexts38 returns the texts from ContactIdentity on.
*/
func schemaTexts38() map[string]map[string]string {
	return map[string]map[string]string{
		"ContactIdentity": {
			"type": "The kind of identity: email, phone, widget_visitor, external_id, slack_user, " +
				"whatsapp_user, facebook_user or instagram_user.",
			"value": "The identity itself on that channel, such as an email address, a phone number or " +
				"the channel's own user id.",
			"verified": "True when a trusted source proved this identity belongs to the contact.",
		},
		"ContactNote": {
			"body": "The note's text.",
			"company_id": "Id of the company the note is about, starting with cmp_. Null when the note " +
				"is about a contact.",
			"contact_id": "Id of the contact the note is about, starting with ctc_. Null when the note " +
				"is about a company.",
			"editable": "True when the member reading may rewrite or delete the note: its author may, " +
				"and anyone may when it has no author.",
			"pinned": "True when the note is pinned.",
		},
		"ContactNoteAuthor": {
			"name": "The display name of the teammate who wrote the note.",
		},
		"ContactSummary": {
			"avatar_url": "URL of the contact's picture. Null when they have none.",
			"email":      "The contact's email address. Null when unknown.",
			"kind": "How well the contact is known: user when your app identified them, lead for " +
				"anyone else.",
			"name": "The contact's name. Null when unknown.",
			"timezone": "The contact's IANA time zone, such as Europe/Berlin, for reading times in " +
				"their local time. Null when unknown.",
		},
		"ContentGap": {
			"last_date": "The last day someone searched these words and found nothing, as a date such " +
				"as 2026-11-01.",
			"query":    "The words readers searched for that found no article.",
			"searches": "How many times this search came up empty over the period.",
		},
	}
}

/*
schemaTexts39 returns the texts from ContextCompany on.
*/
func schemaTexts39() map[string]map[string]string {
	return map[string]map[string]string{
		"ContextCompany": {
			"custom_attributes": "The company's custom attributes as an object of names and values. An " +
				"empty object when it has none.",
			"domain": "The company's web domain, such as acme.com. Null when unknown.",
			"name":   "The company's name.",
		},
		"Conversation": {
			"ai_handoff_reason": "Why the AI agent stepped aside, such as customer_requested, unknown, " +
				"max_replies, frustration, safety or human_replied. Null when it has not handed off.",
			"ai_pending_approvals": "How many of the AI agent's actions in this conversation are " +
				"waiting for a teammate to approve them.",
			"ai_reply_count": "How many replies the AI agent has sent in this conversation.",
			"ai_state": "Who may speak next: off (the AI agent is not in it), active (the agent " +
				"answers) or handed_off (only humans from now on).",
			"closed_at": "When the conversation was closed. Null when it has never been closed.",
			"first_customer_message_at": "When the customer first wrote in this conversation. Null " +
				"when they have not written yet.",
			"first_reply_at": "When the customer got their first reply. Null when nobody has replied " +
				"yet.",
			"last_activity_at": "When anything last happened in the conversation. Null when nothing " +
				"has yet.",
			"last_message_at": "When the latest message was written. Null when there are no messages.",
			"message_count":   "How many messages the conversation holds.",
			"number": "The human reference number, such as 1042 for #1042. Unique within the workspace " +
				"and never reused.",
			"priority": "How urgent the conversation is: none, low, medium, high or urgent.",
			"snoozed_until": "When a snoozed conversation wakes and reopens. Null unless status is " +
				"snoozed.",
			"spam": "True when the conversation is marked as spam. A flag beside status, so unmarking " +
				"it restores where it stood.",
			"status": "Where the conversation stands: open (needs the team), snoozed (until " +
				"snoozed_until) or closed.",
			"subject": "The conversation's subject line. Null when it has none.",
			"tags":    "The tags on the conversation, each with its id and name. Empty when it has none.",
		},
	}
}

/*
schemaTexts40 returns the texts from Conversation on.
*/
func schemaTexts40() map[string]map[string]string {
	return map[string]map[string]string{
		"Conversation": {
			"unread": "True when the conversation has something the calling member has not read. " +
				"Judged for the caller, not for the whole team.",
			"viewers": "The teammates looking at the conversation right now, each with id, name and " +
				"avatar. Present only when the list asked for viewers.",
			"waiting_since": "Since when the customer has waited for an answer: their latest message " +
				"is newer than the latest reply. Null when nobody waits or it is closed.",
		},
		"ConversationBulkResult": {
			"changed": "The fields that changed, such as status, assignee_id, team_id, spam, add_tags, " +
				"remove_tags or unread, or deleted when it was trashed. Empty when nothing changed.",
			"error": "The code of the error that stopped this conversation's change, such as " +
				"not_found. Absent when it went through.",
			"ok": "True when the change went through for this conversation, false when error says why " +
				"it did not.",
		},
		"ConversationContext": {
			"duplicates_count": "How many other contacts look like the same person as this " +
				"conversation's customer.",
			"identities": "The ways the customer appears on channels, such as an email address or a " +
				"WhatsApp number, each with its type, value and whether it is verified.",
			"source": "The page, browser and device the conversation started from, as the channel " +
				"recorded it. An empty object when nothing was recorded.",
		},
		"ConversationCounts": {
			"computed_at": "When these counts were worked out.",
		},
		"ConversationRating": {
			"asked_at": "When the customer was asked to rate the conversation. Null when the ask has " +
				"not gone out.",
			"channel": "Where the customer was asked: widget or email.",
			"comment": "What the customer wrote with their rating. Null when they left no comment.",
			"conversation_id": "Id of the conversation that was rated, starting with conv_. Send it " +
				"wherever a conversation is asked for.",
		},
	}
}

/*
schemaTexts41 returns the texts from ConversationRating on.
*/
func schemaTexts41() map[string]map[string]string {
	return map[string]map[string]string{
		"ConversationRating": {
			"face":     "The emoji face the customer picked, from 😠 for a score of 1 to 😍 for 5.",
			"label":    "The word for the score: Terrible, Bad, OK, Good or Amazing.",
			"rated_at": "When the customer gave their rating.",
			"score":    "The rating from 1 (worst) to 5 (best).",
		},
		"ConversationRatingSettings": {
			"delay_minutes": "How many minutes after a conversation closes the customer is asked to " +
				"rate it.",
			"email":                "True when ratings are asked for by email.",
			"enabled":              "True when the workspace asks customers to rate closed conversations at all.",
			"include_agent_closed": "True when conversations the AI agent closed ask for a rating too.",
			"messenger":            "True when ratings are asked for in the messenger.",
		},
		"ConversationRatingsOutputBody": {
			"data": "The conversation's answered ratings; asks the customer has not answered are left " +
				"out.",
		},
		"ConversationReadState": {
			"conversation_id": "Id of the conversation, starting with conv_.",
			"last_read_at":    "When the caller last read the conversation. Null when they never have.",
			"last_read_message_id": "Id of the last message the caller has read, starting with msg_. " +
				"Null when they have read none.",
			"unread": "True when the conversation holds something the caller has not read.",
		},
		"ConversationRef": {
			"number": "The human reference number, such as 1042 for #1042. Unique within the " +
				"workspace.",
			"status":  "Where the conversation stands: open, snoozed or closed.",
			"subject": "The conversation's subject line. Null when it has none.",
		},
		"ConversationView": {
			"definition": "The conditions that decide which conversations the view lists, as a JSON " +
				"object. The list is worked out each time the view opens.",
		},
	}
}

/*
schemaTexts42 returns the texts from ConversationView on.
*/
func schemaTexts42() map[string]map[string]string {
	return map[string]map[string]string{
		"ConversationView": {
			"icon": "The icon shown beside the view. Null when none is set.",
			"name": "The view's name, 1 to 60 characters. Unique among shared views, or among one " +
				"member's personal views.",
			"owner_id": "Id of the member who keeps a personal view, starting with mem_. Null for a " +
				"shared view.",
			"position": "The view's place in the order the team gave shared views; lower comes first.",
			"scope":    "Who sees the view: shared with the whole workspace, or personal to its owner.",
			"sort": "The order the view lists conversations in, such as waiting_since, -created_at, " +
				"-priority or snoozed_until. Null for the default order.",
			"status_filter": "The status the view shows: open, snoozed, closed or any. Null when the " +
				"view sets none.",
			"warnings": "Problems with the view's conditions, each with the condition's path and a " +
				"code such as reference_missing for a deleted record. Empty when all is well.",
		},
		"CountViews": {
			"custom": "Open conversation counts for saved views, keyed by view id (starting with " +
				"cvw_), each as {\"open\": n}. Counts stop at 999.",
			"inboxes": "Open conversation counts per inbox, keyed by inbox id (starting with inbox_), " +
				"each as {\"open\": n}. Counts stop at 999.",
			"members": "Open conversation counts per assigned member, keyed by member id (starting " +
				"with mem_), each as {\"open\": n}. Counts stop at 999.",
			"teams": "Open conversation counts per team, keyed by team id (starting with team_), each " +
				"as {\"open\": n}. Counts stop at 999.",
		},
		"CountViewsAllStruct": {
			"capped": "True when there are more than 999 open conversations, so open shows the cap of " +
				"999 rather than the real number.",
			"open": "How many conversations are open across everything the caller can see, up to 999.",
		},
	}
}
