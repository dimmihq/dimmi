/*
|--------------------------------------------------------------------------
| The Messenger API's Words: UpdateWidgetActionCallRequestBody To WidgetTicketType
|--------------------------------------------------------------------------
|
| What each field of these messenger records means, by schema and field.
|
*/

package mcpserver

/*
messengerTexts8 returns the texts from UpdateWidgetActionCallRequestBody on.
*/
func messengerTexts8() map[string]map[string]string {
	return map[string]map[string]string{
		"UpdateWidgetActionCallRequestBody": {
			"status": "The visitor's answer on the confirmation card: confirmed to let the action run, " +
				"canceled to stop it.",
		},
		"UpdateWidgetAttachmentRequestBody": {
			"status": "The state to move the upload to. Only uploaded is accepted: send it once the " +
				"file's bytes have been sent.",
		},
		"UpdateWidgetContactBody": {
			"custom_attributes": "Attributes the visitor tells about themselves, keyed by attribute " +
				"name. They are stored as unverified; a signed-in customer may set only the ones the workspace " +
				"allows.",
			"email": "The email address the visitor gives to be replied at, such as jane@example.com. " +
				"Only an anonymous visitor may set it, and it is not treated as proven.",
			"name": "The name the visitor gives. Only an anonymous visitor may set it; a signed-in " +
				"customer's name comes from their app.",
			"phone": "The phone number the visitor gives, up to 64 characters. Only an anonymous " +
				"visitor may set it.",
		},
		"UpdateWidgetReadStateRequestBody": {
			"last_read_message_id": "The id of the latest message the visitor has seen in this " +
				"conversation, such as msg_abc123. The team's replies up to it are marked read.",
		},
		"UpdateWidgetSessionBody": {
			"user_jwt": "A fresh identity token your server signed for the session's signed-in " +
				"customer, to prove them again and keep the session going.",
		},
		"UpdatesOutputBody": {
			"data": "The changes since the cursor, each an event name, an id and a payload. Empty when " +
				"nothing changed. A change returned twice keeps the same id, so keep each id once.",
		},
	}
}

/*
messengerTexts9 returns the texts from WidgetActionCall on.
*/
func messengerTexts9() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetActionCall": {
			"expires_at": "When the visitor's chance to confirm runs out, as an RFC 3339 timestamp. " +
				"Null when the call has no deadline.",
			"state": "Where the action stands: awaiting_verification, awaiting_confirmation, " +
				"pending_approval, approved, declined, expired, canceled, running, succeeded or failed.",
			"status": "The same value as state, under the name the confirmation card reads.",
			"summary": "A plain description of what the action will do, to show the visitor before " +
				"they confirm.",
			"title": "The action's short name, shown as the heading of the confirmation card.",
		},
		"WidgetArticleCard": {
			"slug": "The help article's url-safe name, such as getting-started. Match it against the " +
				"help tab's articles to open the article in place.",
			"summary": "The article's short description. Absent when it has none.",
			"title":   "The article's title, shown on the card.",
			"url":     "The article's address on the help center. Absent when there is none.",
		},
		"WidgetAttachment": {
			"blocked": "True only when the malware scan refused the file; its url is then null. Absent " +
				"otherwise.",
			"content_type": "The file's media type, such as image/png.",
			"filename":     "The file's name as uploaded, such as invoice.pdf.",
			"height": "The image's height in pixels. Null when it is not recorded, as for files that " +
				"are not images.",
			"size_bytes": "The file's size in bytes.",
			"url": "A short-lived link to download the file. Null while the file is still being " +
				"checked, when it was blocked, or when no link can be made.",
			"width": "The image's width in pixels. Null when it is not recorded, as for files that are " +
				"not images.",
		},
		"WidgetBroadcast": {
			"display": "How to draw the broadcast's message: message for an ordinary chat message, " +
				"post for a card with a title.",
			"title": "The heading a post card leads with. Absent when the broadcast has none.",
		},
	}
}

/*
messengerTexts10 returns the texts from WidgetChoice on.
*/
func messengerTexts10() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetChoice": {
			"label": "The text to show on the answer's button.",
		},
		"WidgetContact": {
			"email": "The visitor's email address as they or their app gave it. Null when none is " +
				"known.",
			"kind": "user for a customer the host app signed in, lead for a visitor it has not.",
			"name": "The visitor's name as they or their app gave it. Null when none is known.",
		},
		"WidgetConversation": {
			"ai_state": "The AI agent's part in the conversation: off, active while it is answering, " +
				"or handed_off once it passed the conversation to the team.",
			"customer_status": "What to tell the customer: closed, waiting_on_you when the team or the " +
				"AI agent spoke last, or with_team otherwise.",
			"imported": "True when the conversation was imported from another tool. Absent otherwise.",
			"last_message_at": "When the latest message was sent, as an RFC 3339 timestamp. Null when " +
				"the conversation has no message yet.",
			"number": "The conversation's number within the workspace, such as 1042.",
			"seen_by_team_at": "When a teammate last read up to the customer's latest message. Absent " +
				"when nobody has, or when the workspace hides it from customers.",
			"status":  "open or closed. A conversation the team snoozed shows as open.",
			"subject": "The conversation's subject line. Null when it has none.",
			"unread_count": "How many of the team's replies in this conversation the visitor has not " +
				"read.",
			"widget_id": "The id of the messenger the conversation started on, such as wgt_abc123. " +
				"Null for a conversation that started on another channel.",
		},
		"WidgetEventSent": {
			"metadata": "Details about the event, as an object of your own keys and values, such as " +
				"{\"plan\": \"pro\"}. Optional.",
			"name": "The name of what the visitor did, up to 64 characters, such as checkout_started.",
		},
	}
}

/*
messengerTexts11 returns the texts from WidgetIdentity on.
*/
func messengerTexts11() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetIdentity": {
			"error": "Why the visitor's identity was not verified. Null when there is nothing to " +
				"report.",
			"status": "verified when the session is a signed-in customer's, anonymous otherwise.",
			"warnings": "Notes about the identity check that did not stop it. Empty when there are " +
				"none.",
		},
		"WidgetLastMessage": {
			"excerpt": "A short plain-text preview of the conversation's latest message, for a list " +
				"row. Empty when that message was deleted.",
		},
		"WidgetMessage": {
			"attachments": "The files on the message. Empty when it has none or the message was " +
				"deleted.",
			"body": "The message's text. Null when the message was deleted, or when it carries no " +
				"text.",
			"choices": "The answers a workflow's question offers as buttons, each an id and a label. " +
				"Send the tapped id as choice in the reply. Absent on any other message.",
			"choices_only": "True when the question can be answered only by tapping a choice, so hide " +
				"the text box until one is tapped. Absent otherwise.",
			"client_id": "The client_id the visitor's client sent with this message, to match it with " +
				"the copy shown while sending. Null for messages sent without one.",
			"conversation_id": "The id of the conversation the message belongs to, such as " +
				"conv_abc123.",
			"deleted": "True when the message was deleted: keep its place in the thread and show no " +
				"body or files. Absent otherwise.",
			"read_at": "When the message was read by the other side, as an RFC 3339 timestamp. Null " +
				"while it is unread.",
		},
		"WidgetMessageBody": {
			"attachment_ids": "The ids of up to 10 uploaded files to send with the message, such as " +
				"att_abc123, from the visitor's own finished uploads.",
			"body": "The message's text. Optional when the message carries files or a choice.",
			"choice": "The id of the choice button the visitor tapped, when answering a question drawn " +
				"as buttons. The workflow follows that path whatever the body says.",
		},
	}
}

/*
messengerTexts12 returns the texts from WidgetMessageBody on.
*/
func messengerTexts12() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetMessageBody": {
			"client_id": "An id your client makes up for this message, up to 64 characters, such as a " +
				"UUID. Sending again with the same value does not create a second message.",
			"context": "Extra facts the page passes along about the visit, as a flat object of text, " +
				"number or true/false values. At most 30 keys; a null value removes a key sent earlier.",
			"website": "The hidden anti-bot field named by the config's honeypot_field. Always send it " +
				"empty or leave it out.",
		},
		"WidgetPageBody": {
			"referrer": "The address the visitor came to this page from, as the browser reports it. " +
				"Optional.",
			"title": "The page's title, up to 1000 characters. Optional.",
			"url": "The full address of the page the visitor was on, such as " +
				"https://example.com/pricing.",
		},
		"WidgetProfile": {
			"custom_attributes": "The contact's custom attributes as an object keyed by attribute " +
				"name.",
			"email": "The visitor's email address as they or their app gave it. Null when none is " +
				"known.",
			"email_verified": "True when the email address is confirmed as the visitor's own, false " +
				"when it is only what they typed.",
			"kind":   "user for a customer the host app signed in, lead for a visitor it has not.",
			"locale": "The visitor's language, such as en-US. Null when none is known.",
			"name":   "The visitor's name as they or their app gave it. Null when none is known.",
			"phone": "The visitor's phone number as they or their app gave it. Null when none is " +
				"known.",
		},
		"WidgetSDKBody": {
			"name": "The name of the client library making the call, up to 64 characters, such as " +
				"@dimmi/browser.",
			"version": "The client library's version, such as 1.4.0.",
		},
	}
}

/*
messengerTexts13 returns the texts from WidgetSession on.
*/
func messengerTexts13() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetSession": {
			"config_version": "The messenger's current settings version. When it is higher than the " +
				"config you hold, fetch the config again.",
			"expires_at": "When the session stops working, as an RFC 3339 timestamp. Start a new " +
				"session after that.",
			"kind": "anonymous for a visitor nobody signed in, identified for a customer verified with " +
				"an identity token.",
			"open_conversation_ids": "The ids of the visitor's conversations that are not closed, such " +
				"as conv_abc123. Empty when there are none.",
			"server_time": "The server's clock when it answered, as an RFC 3339 timestamp.",
			"test":        "True for a test session, whose conversations are marked as test ones.",
			"token": "The session's secret, starting with dmi_ws_, to send as the bearer token on " +
				"later calls. Present only on the answer that created the session, so store it then.",
			"unread_count": "How many of the team's replies the visitor has not read, across their " +
				"conversations on this messenger.",
		},
		"WidgetTicket": {
			"conversation_id": "The id of the conversation where the request is discussed, such as " +
				"conv_abc123. Null when it has none.",
			"description": "The longer explanation given when the request was opened. Empty when none " +
				"was given.",
			"fields": "The request's fields a customer may see, each with its current value when it " +
				"has one.",
			"history": "The states the request went through, each with its customer-facing label, its " +
				"group and when it was entered.",
			"mine": "True when this visitor opened the request, false for a colleague's request their " +
				"company shares.",
			"reference": "The request's short reference as people quote it, such as T-482. It also " +
				"works in place of the id when fetching the request.",
			"resolved_at": "When the request was resolved, as an RFC 3339 timestamp. Null while it is " +
				"still open.",
			"title": "The request's short title.",
		},
	}
}

/*
messengerTexts14 returns the texts from WidgetTicketBlock on.
*/
func messengerTexts14() map[string]map[string]string {
	return map[string]map[string]string{
		"WidgetTicketBlock": {
			"reference": "The request's short reference as people quote it, such as T-482.",
			"title":     "The request's short title.",
		},
		"WidgetTicketField": {
			"currency": "The three-letter currency code of a currency field, such as USD. Absent on " +
				"other fields.",
			"data_type": "The kind of value: short_text, long_text, number, decimal, currency, " +
				"boolean, date, single_choice, multiple_choice, file, member or company.",
			"description": "Help text explaining the field to the customer. Absent when it has none.",
			"editable":    "True when the customer may change this field's value.",
			"key": "The field's stable name, used as the key in the fields object when opening a " +
				"request, such as order_number.",
			"label": "The field's name as shown to the customer.",
			"options": "The values to pick from on a single_choice or multiple_choice field. Empty for " +
				"other fields.",
			"required": "True when the customer must fill this field to open a request.",
			"value": "The field's current value on a request, shaped by its data_type. Absent on a " +
				"type's form and when the request has no value for it.",
		},
		"WidgetTicketState": {
			"at": "When the request entered this state, as an RFC 3339 timestamp. Present only in a " +
				"request's history.",
			"group": "The stage the state belongs to, which orders a progress strip: submitted, " +
				"in_progress, waiting_customer, waiting_team or resolved.",
			"label": "The state's name as the customer reads it, never the team's internal name for " +
				"it.",
		},
		"WidgetTicketType": {
			"company_sharing": "True when requests of this type are shared with the customer's " +
				"colleagues at the same company.",
			"description": "A short explanation of what this type of request is for. May be empty.",
			"fields": "The form fields a customer fills when opening this type, in order. Empty when " +
				"the type appears inside a request, whose own fields list holds them.",
			"icon": "The name of the icon the team picked for this type.",
		},
	}
}
