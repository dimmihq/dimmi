/*
|--------------------------------------------------------------------------
| The Messenger API's Operations, Continued
|--------------------------------------------------------------------------
|
| The entries for the messenger API's operations: what the messenger
| and its SDKs call for a customer. They are not tools; the API
| document explains them the same way it explains the rest.
|
*/

package mcpserver

/*
messengerDocsFromWidgetConversationsMessagesStore returns entries from
widget.conversations.messages.store on.
*/
func messengerDocsFromWidgetConversationsMessagesStore() map[string]Doc {
	return map[string]Doc{
		"widget.conversations.messages.store": {
			Title: "Send a message",
			Use: "Writes the visitor's message in a conversation. Needs the messenger session the " +
				"conversation belongs to. A conversation closed long ago, or imported history, cannot take it: " +
				"a new conversation does, and the answer names the old one in redirected_from. A session may " +
				"send 20 messages a minute.",
			Returns: "the stored message, with the conversation's status, whether it reopened and any " +
				"redirect.",
			Params: map[string]string{
				"attachment_ids": "Up to 10 ids of files this session finished uploading, such as " +
					"[\"att_4k2j9x\"]; one that is missing, unfinished or another visitor's refuses the message.",
				"body": "The message text, such as \"Where is my order?\"; at most 64 KB, and required " +
					"unless attachment_ids names a file.",
				"choice": "Id of the button the visitor tapped when the message answers a question " +
					"drawn as buttons, up to 64 characters, so the workflow takes that path. Leave out otherwise.",
				"client_id": "A UUID the client makes up for this message, such as " +
					"6f1c2c1e-8a4b-4d1f-9c3e-2b7a5d9e0f11; sending the same one again returns the stored message " +
					"instead of a second one.",
				"context": "Flat object of details the page passes along, such as {\"plan\": \"pro\"}: " +
					"text, number or true/false values only, at most 30 keys and 4 KB, and null removes a key.",
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
				"page": "The page the visitor is on, as an object with url, title and referrer. " +
					"Optional.",
				"page.referrer": "Address of the page that led here, such as https://google.com/; " +
					"anything that is not an http or https address is dropped.",
				"page.title": "The page's title, such as Pricing; text beyond 300 characters is cut.",
				"page.url": "Full http or https address of the page, such as https://acme.com/pricing; " +
					"kept up to 2048 characters, with query values that look like credentials removed.",
				"website": "Hidden anti-bot form field that real visitors never fill, up to 1000 " +
					"characters; leave it out or empty.",
			},
		},
	}
}

/*
messengerDocsFromWidgetConversationsRatingShow returns entries from
widget.conversations.rating.show on.
*/
func messengerDocsFromWidgetConversationsRatingShow() map[string]Doc {
	return map[string]Doc{
		"widget.conversations.rating.show": {
			Title: "Get the rating request",
			Use: "Returns the latest request asking the visitor to rate the conversation, to draw the " +
				"rating card. Needs the messenger session the conversation belongs to.",
			Returns: "the conversation's latest rating request, or null when none was sent.",
			Params: map[string]string{
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
			},
		},
		"widget.conversations.rating.update": {
			Title: "Rate a conversation",
			Use: "Saves the visitor's score and comment for a conversation that asked how it went. Needs " +
				"the messenger session the conversation belongs to and an open rating request; answers not " +
				"found without one. Send a score, a comment or both; they can be changed for a week after the " +
				"request went out.",
			Returns: "the rating with the visitor's answer.",
			Params: map[string]string{
				"comment": "What the visitor wants to add, up to 2000 characters, such as \"Fast and " +
					"friendly\". An empty text clears the comment; leaving it out keeps it.",
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
				"score": "Whole number from 1 (worst) to 5 (best), such as 4. Leaving it out keeps the " +
					"score already given.",
			},
		},
	}
}

/*
messengerDocsFromWidgetConversationsReadStateUpdate returns entries from
widget.conversations.read-state.update on.
*/
func messengerDocsFromWidgetConversationsReadStateUpdate() map[string]Doc {
	return map[string]Doc{
		"widget.conversations.read-state.update": {
			Title: "Mark messages as read",
			Use: "Says how far the visitor read, so the team's replies up to that message count as read. " +
				"Needs the messenger session the conversation belongs to. The pointer only moves forward: an " +
				"older message changes nothing.",
			Returns: "the read state with the count of replies still unread.",
			Params: map[string]string{
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
				"last_read_message_id": "Id of the newest message the visitor has seen in this " +
					"conversation, such as msg_9d3f7a. A message of another conversation is refused.",
			},
		},
		"widget.conversations.show": {
			Title:   "Get a conversation",
			Use:     "Returns one of the visitor's conversations. Needs the messenger session it belongs to.",
			Returns: "the conversation.",
			Params: map[string]string{
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
			},
		},
	}
}

/*
messengerDocsFromWidgetConversationsStore returns entries from
widget.conversations.store on.
*/
func messengerDocsFromWidgetConversationsStore() map[string]Doc {
	return map[string]Doc{
		"widget.conversations.store": {
			Title: "Start a conversation",
			Use: "Starts a new conversation with the visitor's first message. Needs a messenger session; " +
				"the messenger's rules may refuse the visitor, and a messenger that asks for an email while " +
				"the team is away refuses a visitor without one. A session may start 5 conversations an hour.",
			Returns: "the new conversation, with the stored message beside it.",
			Params: map[string]string{
				"attachment_ids": "Up to 10 ids of files this session finished uploading, such as " +
					"[\"att_4k2j9x\"]; one that is missing, unfinished or another visitor's refuses the message.",
				"body": "The message text, such as \"Where is my order?\"; at most 64 KB, and required " +
					"unless attachment_ids names a file.",
				"choice": "Id of the button the visitor tapped when the message answers a question " +
					"drawn as buttons, up to 64 characters, so the workflow takes that path. Leave out otherwise.",
				"client_id": "A UUID the client makes up for this message, such as " +
					"6f1c2c1e-8a4b-4d1f-9c3e-2b7a5d9e0f11; sending the same one again returns the stored message " +
					"instead of a second one.",
				"context": "Flat object of details the page passes along, such as {\"plan\": \"pro\"}: " +
					"text, number or true/false values only, at most 30 keys and 4 KB, and null removes a key.",
				"page": "The page the visitor is on, as an object with url, title and referrer. " +
					"Optional.",
				"page.referrer": "Address of the page that led here, such as https://google.com/; " +
					"anything that is not an http or https address is dropped.",
				"page.title": "The page's title, such as Pricing; text beyond 300 characters is cut.",
				"page.url": "Full http or https address of the page, such as https://acme.com/pricing; " +
					"kept up to 2048 characters, with query values that look like credentials removed.",
				"website": "Hidden anti-bot form field that real visitors never fill, up to 1000 " +
					"characters; leave it out or empty.",
			},
		},
	}
}

/*
messengerDocsFromWidgetConversationsTypingIndicatorsStore returns entries from
widget.conversations.typing-indicators.store on.
*/
func messengerDocsFromWidgetConversationsTypingIndicatorsStore() map[string]Doc {
	return map[string]Doc{
		"widget.conversations.typing-indicators.store": {
			Title: "Send a typing notice",
			Use: "Tells the team the visitor started or stopped typing in the conversation. Needs the " +
				"messenger session the conversation belongs to. Nothing is stored, and repeated starts are " +
				"thinned so teammates are not flooded.",
			Params: map[string]string{
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
				"state": "started when the visitor begins typing, stopped when they stop or send.",
			},
		},
		"widget.email-verifications.checks.store": {
			Title: "Check an email code",
			Use: "Takes the code the visitor typed. A good one marks the session's email proven for a day " +
				"and the visitor's address verified. Needs the messenger session that asked for the code. Five " +
				"wrong tries lock the code, and an expired code needs a new one.",
			Returns: "the masked address and when it was verified.",
			Params: map[string]string{
				"code": "The six-digit code from the email, such as 482913; spaces around it are " +
					"ignored.",
			},
		},
		"widget.email-verifications.store": {
			Title: "Send an email code",
			Use: "Mails a six-digit code to an address so an anonymous visitor can prove they own it. " +
				"Needs an anonymous messenger session; a signed-in session is refused. The code works for 10 " +
				"minutes, replaces any earlier one, and another may be sent after 30 seconds, up to 5 an hour " +
				"per session.",
			Returns: "the masked address, when the code expires and when another may be sent.",
			Params: map[string]string{
				"email": "The bare email address to send the code to, 3 to 254 characters, such as " +
					"ada@example.com.",
			},
		},
	}
}

/*
messengerDocsFromWidgetEventsStore returns entries from widget.events.store on.
*/
func messengerDocsFromWidgetEventsStore() map[string]Doc {
	return map[string]Doc{
		"widget.events.store": {
			Title: "Track visitor events",
			Use: "Records what the visitor did on the page, up to 20 events at once, on the session's " +
				"contact. Needs a messenger session. Only event names the workspace opened to the messenger " +
				"are recorded, unless it lets the messenger send any; the server sets each event's time. A " +
				"session may send 30 batches a minute.",
			Returns: "what became of each event sent, such as accepted, duplicate or not_allowed.",
			Params: map[string]string{
				"events": "List of 1 to 20 events to record, each with a name and optionally metadata " +
					"and an id.",
				"events.id": "An id the client makes up for the event, up to 64 characters, such as " +
					"evt-1042. A retry with the same id within 48 hours is counted once.",
				"events.metadata": "Flat object of details about the event, such as {\"plan\": " +
					"\"pro\", \"seats\": 5}: at most 20 keys of text, numbers or true/false, 2 KB in all.",
				"events.name": "The event's name: 1 to 64 lowercase letters, digits and _ . : -, such " +
					"as checkout.completed.",
			},
		},
		"widget.help-passes.store": {
			Title: "Open the help center",
			Use: "Returns the address to open an article, or the help center's home, in the full help " +
				"center. Needs a messenger session and a help center linked to the messenger. A visitor the " +
				"messenger knows gets an address that signs them in on arrival, good once and for a minute; " +
				"anyone else gets the plain address.",
			Returns: "the address to open.",
			Params: map[string]string{
				"article": "Slug of the article to open: up to 200 lowercase letters, digits and " +
					"hyphens, such as reset-your-password. Leave it out for the help center's home.",
			},
		},
	}
}

/*
messengerDocsFromWidgetHelpShow returns entries from widget.help.show on.
*/
func messengerDocsFromWidgetHelpShow() map[string]Doc {
	return map[string]Doc{
		"widget.help.show": {
			Title: "Read the visitor's help tab",
			Use: "Returns the help center's collections and articles as this visitor may read them: what " +
				"their audiences open when the messenger knows them (signed in, or with a proven email), and " +
				"what anyone may read otherwise. Needs a messenger session. Everything is empty when the " +
				"messenger links to no help center.",
			Returns: "the help center's address, its collections, up to 200 articles with body blocks, " +
				"and up to 5 popular article slugs.",
		},
		"widget.openers.store": {
			Title: "Open an empty conversation",
			Use: "Starts a conversation before the visitor has written anything, so a workflow can greet " +
				"them the moment the chat opens. Needs a messenger session. The same limit as a first message " +
				"applies, 5 new conversations an hour per session, and the messenger's rules may refuse the " +
				"visitor.",
			Returns: "the new, empty conversation.",
			Params: map[string]string{
				"context": "Flat object of details the page passes along, such as {\"plan\": \"pro\"}: " +
					"text, number or true/false values only, at most 30 keys and 4 KB, and null removes a key.",
				"page": "The page the visitor is on, as an object with url, title and referrer. " +
					"Optional.",
				"page.referrer": "Address of the page that led here, such as https://google.com/; " +
					"anything that is not an http or https address is dropped.",
				"page.title": "The page's title, such as Pricing; text beyond 300 characters is cut.",
				"page.url": "Full http or https address of the page, such as https://acme.com/pricing; " +
					"kept up to 2048 characters, with query values that look like credentials removed.",
			},
		},
	}
}
