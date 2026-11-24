/*
|--------------------------------------------------------------------------
| Written Schema Texts: TimelineEntry To WebhookEndpoint
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromTimelineEntryAndTokenAbility returns the texts from
TimelineEntry on.
*/
func schemaTextsFromTimelineEntryAndTokenAbility() map[string]map[string]string {
	return map[string]map[string]string{
		"TimelineEntry": {
			"type": "What the entry is: message (exchanged with the customer), note (internal, never " +
				"shown to the customer) or event (something that happened, such as an assignment).",
		},
		"TokenAbility": {
			"ability": "Name of an ability group a token can be given, such as conversations:read or " +
				"tickets:write.",
			"allowed":     "True when the caller's role lets them give this ability to a token.",
			"permissions": "The permissions this ability carries, as a list of permission names.",
		},
		"TokenMember": {
			"name": "Name of the member the token acts as.",
		},
		"TokenSummary": {
			"expires_at": "When the token stops working. Null when it never expires.",
			"name":       "The name the token was given when it was made.",
			"prefix":     "The start of the token, by which it is recognized; never the secret itself.",
		},
		"TopArticle": {
			"helpful":   "How many readers marked the article helpful in the reported days.",
			"slug":      "The article's slug, the part of its web address that names it.",
			"title":     "The article's title.",
			"unhelpful": "How many readers marked the article not helpful in the reported days.",
			"views":     "How many times the article was viewed in the reported days.",
		},
		"TopChangelogEntry": {
			"published_at": "When the update was published. Null when it is not published.",
			"reactions":    "How many reactions the update got in the reported days.",
			"slug":         "The update's slug, the part of its web address that names it.",
			"title":        "The update's title.",
			"views":        "How many times the update was viewed in the reported days.",
		},
	}
}

/*
schemaTextsFromTrackedEvent returns the texts from TrackedEvent on.
*/
func schemaTextsFromTrackedEvent() map[string]map[string]string {
	return map[string]map[string]string{
		"TrackedEvent": {
			"contact_id": "Id of the contact the event was recorded for, such as ctc_.... Null when no " +
				"contact was matched.",
			"occurred_at_clamped": "True when the event's time was outside the allowed range and was " +
				"moved into it. Left out otherwise.",
			"status": "What became of the event: accepted, duplicate, contact_not_found, archived, " +
				"event_limit_reached or not_allowed.",
		},
		"TrackedEvents": {
			"accepted": "How many of the events sent were accepted.",
			"events":   "The result of each event sent, in the order sent.",
		},
		"TrackerConversation": {
			"company_id":   "Id of the customer's company, such as cmp_.... Null when they have none.",
			"company_name": "Name of the customer's company. Null when they have none.",
			"company_plan": "The company's plan. Null when it has none.",
			"company_revenue": "The company's monthly revenue, read from its monthly_spend or mrr " +
				"attribute; 0 when neither is a number.",
			"contact_email": "The customer's email address. Null when unknown.",
			"contact_id": "Id of the customer in the conversation, such as ctc_.... Send it wherever a " +
				"contact is asked for.",
			"contact_name": "The customer's name. Null when unknown.",
			"conversation_id": "Id of the linked conversation, such as conv_.... Send it wherever a " +
				"conversation is asked for.",
			"conversation_number": "The conversation's number within the workspace.",
			"last_message_at":     "When the conversation last had a message. Null when it has none.",
			"link_id": "Id of the link joining the conversation to the tracker, such as tktl_.... Use " +
				"it to remove the link.",
			"linked_at": "When the conversation was linked to the tracker.",
			"status":    "The conversation's status: open, snoozed or closed.",
			"subject":   "The conversation's subject. Null when it has none.",
			"ticket_id": "Id of the conversation's own customer ticket, such as tkt_.... Null when the " +
				"conversation is not a ticket.",
			"ticket_reference": "Short reference of the conversation's own ticket, such as T-42. Null " +
				"when the conversation is not a ticket.",
		},
	}
}

/*
schemaTextsFromTrackerConversation returns the texts from
TrackerConversation on.
*/
func schemaTextsFromTrackerConversation() map[string]map[string]string {
	return map[string]map[string]string{
		"TrackerConversation": {
			"ticket_state_id": "Id of the state the conversation's own ticket is in, such as tkts_.... " +
				"Null when the conversation is not a ticket.",
		},
		"TrackerConversationsOutputBody": {
			"data": "The conversations linked to the tracker, each with its customer, company, plan " +
				"and revenue.",
		},
		"TrackerSuggestion": {
			"linked_count": "How many conversations are already linked to the suggested tracker.",
			"reference":    "The suggested tracker's short reference, such as T-42.",
			"state_group": "The group of the suggested tracker's current state: submitted, " +
				"in_progress, waiting_customer, waiting_team or resolved.",
			"ticket_id": "Id of the suggested tracker ticket, such as tkt_.... Send it to link the " +
				"conversation to that tracker.",
			"title": "The suggested tracker's title.",
		},
		"TwoFactorReset": {
			"expires_at": "Until when the mailed reset link works.",
			"member_id":  "Id of the member the reset link was mailed to, such as mem_....",
			"status": "Where the reset stands: pending once the link is mailed, completed once it has " +
				"been used.",
		},
		"TypingIndicator": {
			"expires_at": "When the customer's screen stops showing the typing sign unless typing is " +
				"reported again.",
			"state": "The typing state that was accepted: start or stop.",
		},
		"UpsertMeta": {
			"possible_duplicates": "Other contacts that may be the same person, such as a lead who " +
				"only typed the same email address, for a teammate to judge. Left out when there are none.",
			"upserted": "Whether the request added the record or changed an existing one: created or " +
				"updated.",
		},
	}
}

/*
schemaTextsFromUser returns the texts from User on.
*/
func schemaTextsFromUser() map[string]map[string]string {
	return map[string]map[string]string{
		"User": {
			"avatar_url":     "Web address of the person's profile picture. Null when they have none.",
			"email":          "The email address the person signs in with.",
			"email_verified": "True when the person has proven they own their email address.",
			"has_password": "True when the account has a password set; false when the person only " +
				"signs in another way.",
			"locale": "The language the person picked, such as en or fr. Null while they follow their " +
				"workspace's language.",
			"name": "The person's full name.",
			"preferences": "The person's own settings with defaults filled in, keyed by name, such as " +
				"{\"send_with_enter\": false}.",
			"timezone": "The time zone the person picked, as an IANA name such as Europe/Paris. Null " +
				"while they follow their workspace's time zone.",
			"two_factor_enabled": "True when the account has a second sign-in factor turned on.",
		},
		"ViewPreference": {
			"columns": "The columns the member's table layout of this view shows, in order, at most " +
				"40. Left out for the default.",
			"layout": "How the member likes this view laid out: list or table. Empty or left out for " +
				"the default.",
			"sort": "The order the member likes this view sorted in. Empty or left out for the " +
				"default.",
			"status": "The conversation status the member likes this view to show. Empty or left out " +
				"for the default.",
		},
		"ViewPreferenceBody": {
			"columns": "The columns the member's table layout of this view shows, in order, at most " +
				"40. Left out for the default.",
			"layout": "How the member likes this view laid out: list or table. Empty or left out for " +
				"the default.",
			"sort": "The order the member likes this view sorted in. Empty or left out for the " +
				"default.",
			"status": "The conversation status the member likes this view to show. Empty or left out " +
				"for the default.",
		},
	}
}

/*
schemaTextsFromViewWarning returns the texts from ViewWarning on.
*/
func schemaTextsFromViewWarning() map[string]map[string]string {
	return map[string]map[string]string{
		"ViewWarning": {
			"code": "Why the condition is flagged, such as reference_missing for a condition naming a " +
				"record that was deleted.",
			"path": "Where in the view's definition the flagged condition sits.",
		},
		"ViewerPayload": {
			"avatar_url": "Web address of the viewing teammate's picture. Null when not given.",
			"name":       "Name of the teammate currently viewing the conversation.",
		},
		"Warning": {
			"code": "Which kind of likely mistake was found in the audience rules: negative_or, " +
				"impossible_and or page_rule_is_public.",
			"message": "A plain sentence explaining the problem to a person.",
			"path":    "Where in the audience's rules the problem sits, such as rules.",
		},
		"WebhookAttempt": {
			"duration_ms": "How long the request took, in milliseconds. Null when not measured.",
			"error_code": "Why the attempt failed, such as http_error, redirect_not_followed or a " +
				"network failure class. Null when it succeeded.",
			"error_message": "Details of the failure in words, up to 500 characters. Null when there " +
				"are none.",
			"hint": "Advice for fixing the endpoint, given only when it answered 401 or 403 (check the " +
				"signature against the raw body). Left out otherwise.",
			"number": "Which try of the delivery this was, counting from 1.",
			"request_headers": "The HTTP headers dimmi sent with the request, as an object of name to " +
				"value.",
			"requested_at": "When the request was sent.",
			"response_body": "The body the endpoint answered with, as text. Null when it gave none or " +
				"never answered.",
			"response_headers": "The HTTP headers the endpoint answered with, as an object. Null when " +
				"it never answered.",
			"response_status": "HTTP status the endpoint answered with, such as 200 or 500. Null when " +
				"it never answered.",
			"trigger": "What caused the attempt: auto (dimmi's own send or retry), manual (someone " +
				"asked to resend) or test (a test send).",
		},
	}
}

/*
schemaTextsFromWebhookDelivery returns the texts from WebhookDelivery on.
*/
func schemaTextsFromWebhookDelivery() map[string]map[string]string {
	return map[string]map[string]string{
		"WebhookDelivery": {
			"attempt_count": "How many times dimmi has tried to send this delivery.",
			"endpoint_id": "Id of the webhook endpoint the event is sent to, such as whk_.... Send it " +
				"wherever a webhook endpoint is asked for.",
			"event_id": "Id of the event being delivered, such as evt_.... The same event sent to " +
				"several endpoints shares it.",
			"event_type": "Dotted name of the event delivered, such as conversation.created or " +
				"contact.updated; webhook.test for a plain test send.",
			"last_attempt_at": "When the newest attempt was made. Null before the first attempt.",
			"last_error_code": "Error code of the newest failed attempt, such as http_error or " +
				"redirect_not_followed, or a network failure class. Null when it did not fail.",
			"last_response_status": "HTTP status the endpoint answered the newest attempt with, such " +
				"as 200 or 500. Null when it never answered.",
			"next_attempt_at": "When the next attempt is due. Null when none is planned.",
			"payload_bytes":   "Size of the payload sent, in bytes.",
			"sequence": "A number that only goes up across events; of two deliveries about the same " +
				"object, the higher one carries the newer copy.",
			"status":       "Where the delivery stands: pending, retrying, succeeded, failed or canceled.",
			"succeeded_at": "When the endpoint accepted the delivery. Null until it does.",
			"test":         "True when this is a test send rather than a real event.",
			"thin": "True when the full object was too large to send, so the payload carries only a " +
				"reference to it and must be fetched.",
		},
		"WebhookDeliveryDetail": {
			"attempt_count": "How many times dimmi has tried to send this delivery.",
			"attempts": "Every request made for this delivery, oldest first, each with how the " +
				"endpoint answered.",
			"endpoint_id": "Id of the webhook endpoint the event is sent to, such as whk_.... Send it " +
				"wherever a webhook endpoint is asked for.",
			"event_id": "Id of the event being delivered, such as evt_.... The same event sent to " +
				"several endpoints shares it.",
			"event_type": "Dotted name of the event delivered, such as conversation.created or " +
				"contact.updated; webhook.test for a plain test send.",
			"last_attempt_at": "When the newest attempt was made. Null before the first attempt.",
		},
	}
}

/*
schemaTextsFromWebhookDeliveryDetail returns the texts from
WebhookDeliveryDetail on.
*/
func schemaTextsFromWebhookDeliveryDetail() map[string]map[string]string {
	return map[string]map[string]string{
		"WebhookDeliveryDetail": {
			"last_error_code": "Error code of the newest failed attempt, such as http_error or " +
				"redirect_not_followed, or a network failure class. Null when it did not fail.",
			"last_response_status": "HTTP status the endpoint answered the newest attempt with, such " +
				"as 200 or 500. Null when it never answered.",
			"next_attempt_at": "When the next attempt is due. Null when none is planned.",
			"payload":         "The exact JSON payload this delivery sends. Null when it is no longer stored.",
			"payload_bytes":   "Size of the payload sent, in bytes.",
			"sequence": "A number that only goes up across events; of two deliveries about the same " +
				"object, the higher one carries the newer copy.",
			"status":       "Where the delivery stands: pending, retrying, succeeded, failed or canceled.",
			"succeeded_at": "When the endpoint accepted the delivery. Null until it does.",
			"test":         "True when this is a test send rather than a real event.",
			"thin": "True when the full object was too large to send, so the payload carries only a " +
				"reference to it and must be fetched.",
		},
		"WebhookEndpoint": {
			"api_version": "The API version the endpoint's payloads are shaped for, as a date such as " +
				"2026-11-01.",
			"description": "A note saying what the endpoint is for, up to 255 characters. Empty when " +
				"none is set.",
			"disabled_at": "When the endpoint was turned off. Null while it is on.",
			"disabled_reason": "Why the endpoint is off: manual (someone turned it off), auto_failure " +
				"(it kept failing) or gone (it answered 410). Null while it is on.",
			"enabled": "True when dimmi sends events to the endpoint; false when it is turned off and " +
				"nothing is sent.",
			"event_types": "The events the endpoint subscribes to: exact names such as " +
				"conversation.created, an area wildcard, or * for every event.",
			"failing_since": "Since when the endpoint has answered nothing but failures. Null while it " +
				"is healthy.",
			"health": "Whether deliveries are getting through: healthy or failing.",
			"inbox_ids": "Ids of the inboxes whose events the endpoint receives, such as inbox_.... " +
				"Null means every inbox.",
		},
	}
}
