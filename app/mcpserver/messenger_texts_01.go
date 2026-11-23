/*
|--------------------------------------------------------------------------
| The Messenger API's Words: AvailabilityResource To UpdateItem
|--------------------------------------------------------------------------
|
| What each field of these messenger records means, by schema and field.
|
*/

package mcpserver

/*
messengerTexts1 returns the texts from AvailabilityResource on.
*/
func messengerTexts1() map[string]map[string]string {
	return map[string]map[string]string{
		"AvailabilityResource": {
			"online": "True when a teammate who answers this messenger's inbox is set to available and " +
				"connected right now.",
			"reply_time": "How fast the team usually replies: minutes, hours or day. When the " +
				"messenger's setting is automatic, it is minutes while someone is online and hours otherwise.",
			"reply_time_text": "The reply time as words to show the visitor, in the language asked for " +
				"with the locale query parameter.",
			"server_time": "The server's clock when it answered, as an RFC 3339 timestamp.",
			"team": "Up to three teammates to show the visitor, each a name and an avatar link. Only " +
				"teammates with an avatar are listed; empty when none have one.",
			"within_hours": "Whether the team is inside its working hours. Currently always true.",
		},
		"Block": {
			"items": "The entries of a list or steps block as plain text, one per item. Absent on " +
				"every other kind of block.",
			"kind": "What the block is: h2 (heading), p (paragraph), list (bulleted), steps " +
				"(numbered), code, note (quote or callout) or image.",
			"text": "The block's plain text: the heading, paragraph or note wording, the source of a " +
				"code block, or the alt text of an image. Absent on list and steps blocks.",
			"tokens": "A code block's source split into runs for syntax coloring, in order. Absent " +
				"when the code's language is unknown or the block is not code.",
		},
		"ChangelogEntry": {
			"body": "The entry's content as blocks to draw in order, as it was last published.",
			"label": "The name of the entry's first label, such as New. Empty when the entry has no " +
				"label.",
		},
		"ChangelogIndex": {
			"entries": "The latest published changelog entries, at most 20. Empty when the messenger " +
				"has no help center or its What's new tab is off.",
			"url": "The address of the full changelog page on the help center. Empty when there is " +
				"none.",
		},
	}
}

/*
messengerTexts2 returns the texts from CheckWidgetEmailCodeRequestBody on.
*/
func messengerTexts2() map[string]map[string]string {
	return map[string]map[string]string{
		"CheckWidgetEmailCodeRequestBody": {
			"code": "The code the visitor received by email and typed in, 6 to 12 characters. It stops " +
				"working 10 minutes after it was sent.",
		},
		"Config": {
			"app_id": "The messenger's id, such as wgt_abc123. Send it as app_id when starting a " +
				"session.",
			"config_version": "A number that goes up each time the messenger's settings change. " +
				"Compare it with the session's config_version to know when to fetch the config again.",
			"customer_status": "Whether the messenger shows each conversation's customer status. " +
				"Currently always true.",
			"delay_notice": "Whether the messenger's delay notice is on. Currently always true.",
			"dir": "The text direction of the resolved language: rtl for Arabic, Hebrew, Persian and " +
				"Urdu, ltr for every other.",
			"home_links": "The links to show on the home screen, each with a title, a url and an icon, " +
				"in the order set. Empty when none are set.",
			"honeypot_field": "The name of the hidden form field real visitors never fill, currently " +
				"website. Send it empty; a session started with it filled is treated as a bot.",
			"locale": "The language the texts were resolved to, such as en or fr: the messenger's " +
				"fixed language, or else the closest match to the locale asked for, falling back to en.",
			"locales": "Every language dimmi ships messenger texts in, the fallback language first.",
			"open_to": "The tab the messenger opens on. Currently always home.",
			"show_availability": "Whether to show the team's availability and reply time to the " +
				"visitor.",
			"show_team_avatars": "Whether to show teammates' avatars to the visitor.",
			"tabs": "The tabs that are on, in display order: home and messages always, then help and " +
				"changelog when turned on and a help center is linked.",
			"texts": "Every wording the messenger shows, keyed by text name, in the resolved language " +
				"with the team's own wording where they set one.",
		},
		"EmailCodeSent": {
			"email": "The address the code was sent to, masked to its first letter and domain, such as " +
				"j***@example.com.",
		},
	}
}

/*
messengerTexts3 returns the texts from EmailCodeSent on.
*/
func messengerTexts3() map[string]map[string]string {
	return map[string]map[string]string{
		"EmailCodeSent": {
			"expires_at": "When the code stops working, 10 minutes after it was sent, as an RFC 3339 " +
				"timestamp.",
			"resend_after": "The earliest time another code may be asked for, 30 seconds after this " +
				"one, as an RFC 3339 timestamp.",
		},
		"EmailProven": {
			"email": "The address the visitor proved, masked to its first letter and domain, such as " +
				"j***@example.com.",
			"verified_at": "When the visitor proved the address, as an RFC 3339 timestamp. The proof " +
				"lasts 24 hours from then.",
		},
		"HelpArticle": {
			"body": "The article's content as blocks to draw in order, so the messenger can open it " +
				"without another request.",
			"slug": "The article's url-safe name, such as getting-started. Use it to report a view or " +
				"rating, or to ask for a help pass.",
			"summary": "The article's short description. Empty when it has none.",
			"title":   "The article's title.",
			"updated": "When the published version of the article was made, as an RFC 3339 timestamp.",
		},
		"HelpCollection": {
			"articles": "The slugs of the articles in this collection, matching entries of the " +
				"articles list. Empty when it holds none the visitor may read.",
			"description": "The collection's description. Empty when it has none.",
			"slug":        "The collection's url-safe name, such as billing.",
			"title":       "The collection's name.",
		},
		"HelpIndex": {
			"articles": "The help center's articles the visitor may read, with their content, at most " +
				"200. Empty when the messenger links to no help center.",
			"collections": "The help center's collections, each listing the slugs of its articles. " +
				"Empty when the messenger links to no help center.",
		},
	}
}

/*
messengerTexts4 returns the texts from HelpIndex on.
*/
func messengerTexts4() map[string]map[string]string {
	return map[string]map[string]string{
		"HelpIndex": {
			"popular": "The slugs of up to five articles to feature first, taken from the start of the " +
				"articles list.",
			"url": "The address the help center's pages start at. Empty when the messenger links to no " +
				"help center.",
		},
		"HelpPass": {
			"url": "The address to open the article or the help center's home at. When the messenger " +
				"knows the visitor, it signs them in on arrival, so open it soon and do not store it.",
		},
		"PostedMessageMeta": {
			"conversation_status": "The status of the conversation the message landed in, after it was " +
				"sent, such as open.",
			"redirected_from": "The id of the conversation the message was sent to, such as " +
				"conv_abc123, when a new conversation took it instead. Null when it landed where it was sent.",
			"reopened":    "True when the message reopened a closed conversation.",
			"server_time": "The server's clock when it answered, as an RFC 3339 timestamp.",
		},
		"ReadStateResource": {
			"conversation_id": "The id of the conversation that was marked read, such as conv_abc123.",
			"last_read_message_id": "The id of the last message the visitor has read, as sent in the " +
				"request, such as msg_abc123.",
			"unread_count": "How many of the team's replies in this conversation the visitor still has " +
				"not read.",
		},
		"Realtime": {
			"channel": "The stream channel that carries this visitor's updates, such as " +
				"widget_contact:ctc_abc123.",
			"token_url": "The address to POST to for a short-lived connect token before opening the " +
				"stream.",
			"url": "The address of the live update stream (server-sent events) to connect to with a " +
				"connect token.",
		},
	}
}

/*
messengerTexts5 returns the texts from RealtimeTokenResource on.
*/
func messengerTexts5() map[string]map[string]string {
	return map[string]map[string]string{
		"RealtimeTokenResource": {
			"channel": "The stream channel that carries this visitor's updates, such as " +
				"widget_contact:ctc_abc123.",
			"expires_at": "When the connect token stops working, as an RFC 3339 timestamp. Ask for a " +
				"new one before reconnecting after that.",
			"token": "A short-lived signed token that opens this session's live update stream. It is " +
				"good for the stream only, not for API calls.",
			"url": "The address of the live update stream (server-sent events) to connect to with the " +
				"token.",
		},
		"SendWidgetEmailCodeRequestBody": {
			"email": "The email address to send the verification code to, 3 to 254 characters, such as " +
				"jane@example.com.",
		},
		"StoreWidgetArticleRatingRequestBody": {
			"article": "The slug of the help article being rated, such as getting-started: lowercase " +
				"letters, digits and hyphens.",
			"helpful": "True when the visitor said the article helped, false when it did not.",
		},
		"StoreWidgetArticleViewRequestBody": {
			"article": "The slug of the help article the visitor opened, such as getting-started. A " +
				"slug the help center does not show is ignored without an error.",
		},
		"StoreWidgetAttachmentRequestBody": {
			"content_type": "The file's media type, such as image/png. The upload must then be sent " +
				"with this same Content-Type.",
			"filename": "The file's name as the visitor sees it, such as invoice.pdf, 1 to 255 " +
				"characters.",
			"size_bytes": "The file's exact size in bytes, at least 1. The upload must send exactly " +
				"this many bytes.",
		},
		"StoreWidgetEventsRequestBody": {
			"events": "The events to record for the visitor, 1 to 20 per request, in the order they " +
				"happened. The answer lists a status for each in the same order.",
		},
	}
}

/*
messengerTexts6 returns the texts from StoreWidgetHelpPassRequestBody on.
*/
func messengerTexts6() map[string]map[string]string {
	return map[string]map[string]string{
		"StoreWidgetHelpPassRequestBody": {
			"article": "The slug of the help article to open, such as getting-started. Leave it out or " +
				"empty to open the help center's home.",
		},
		"StoreWidgetOpenerRequestBody": {
			"context": "Extra facts the page passes along about the visit, as a flat object of text, " +
				"number or true/false values. At most 30 keys; a null value removes a key sent earlier.",
		},
		"StoreWidgetPingRequestBody": {
			"context": "Extra facts the page passes along about the visit, as a flat object of text, " +
				"number or true/false values. At most 30 keys; a null value removes a key sent earlier.",
			"visible": "Whether the page is currently visible to the visitor. Optional.",
		},
		"StoreWidgetSessionBody": {
			"app_id": "The id of the messenger to start the session on, such as wgt_abc123, as given " +
				"in the install snippet and the config.",
			"context": "Extra facts the page passes along about the visit, as a flat object of text, " +
				"number or true/false values. At most 30 keys.",
			"locale": "The visitor's language as the browser reports it, such as en-US.",
			"pages": "Up to 10 pages the visitor was on before the session started, oldest first, each " +
				"with a url and optionally a title and referrer.",
			"previous_token": "The token of the session this browser held before, sent when the " +
				"visitor signs in so that session is handed over to the signed-in one.",
			"test_token": "A test token for the session. Optional; leave it out for real visitors.",
			"timezone":   "The visitor's time zone as an IANA name, such as Europe/Paris.",
			"user_jwt": "The identity token your server signed for the signed-in customer. A token " +
				"that fails any check refuses the session; leave it out to start an anonymous one.",
			"website": "The hidden anti-bot field named by the config's honeypot_field. Always send it " +
				"empty or leave it out; a filled value marks the session as a bot and its conversations as " +
				"spam.",
		},
	}
}

/*
messengerTexts7 returns the texts from StoreWidgetTicketRequestBody on.
*/
func messengerTexts7() map[string]map[string]string {
	return map[string]map[string]string{
		"StoreWidgetTicketRequestBody": {
			"attachment_ids": "The ids of up to 10 uploaded files to attach to the request, such as " +
				"att_abc123, from the visitor's own finished uploads.",
			"client_id": "An id your client makes up for this submission, up to 64 characters, such as " +
				"a UUID. Sending the form again with the same value creates only one request.",
			"description": "The visitor's longer explanation of the request, up to 10000 characters. " +
				"Optional.",
			"fields": "Values for the type's fields the customer may fill, keyed by field key. A file " +
				"field takes ids of the visitor's uploads.",
			"title": "A short title for the request, up to 200 characters.",
			"type_id": "The id of the request type being opened, such as tktt_abc123, from the list of " +
				"types the visitor may open.",
		},
		"StoreWidgetTypingRequestBody": {
			"state": "started when the visitor begins typing in the conversation, stopped when they " +
				"stop. The team sees it live.",
		},
		"TeamFace": {
			"avatar_url": "A link to the teammate's avatar image that works for one hour, so fetch " +
				"availability again rather than storing it.",
			"name": "The teammate's name as shown to the visitor.",
		},
		"Token": {
			"class": "What the run of code is, for coloring: keyword, string, number or comment. " +
				"Absent for plain code that takes no color.",
			"text": "The run's exact text. Joining every token's text in order gives the code block's " +
				"source.",
		},
		"UpdateItem": {
			"event": "What changed: message.created, message.updated, message.deleted, " +
				"conversation.updated or conversation.closed. The names match the events the live stream " +
				"carries.",
			"payload": "The changed record: a message event carries conversation_id and message, a " +
				"conversation event carries conversation_id, widget_id and conversation.",
		},
	}
}
