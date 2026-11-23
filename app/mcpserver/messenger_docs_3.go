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

import "maps"

/*
messengerDocs15 returns entries from widget.pings.store on.
*/
func messengerDocs15() map[string]Doc {
	return map[string]Doc{
		"widget.pings.store": {
			Title: "Report the visitor's page",
			Use: "Tells dimmi which page the visitor is on now, so the team sees where a question came " +
				"from. Needs a messenger session. It is written down at most once every 10 seconds per " +
				"session; faster pings are accepted and change nothing.",
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
				"visible": "true while the page is in view, false while it is hidden. Optional.",
			},
		},
		"widget.realtime-tokens.store": {
			Title: "Get a realtime token",
			Use: "Issues a short-lived token that opens the visitor's live event stream. Needs a " +
				"messenger session. Ask for a fresh one each time the stream opens or reopens: the token is " +
				"good for about a minute and only for opening a stream, not for calling the API.",
			Returns: "the connect token, when it expires, the stream's address and the visitor's channel.",
		},
	}
}

/*
messengerDocs16 returns entries from widget.session.destroy on.
*/
func messengerDocs16() map[string]Doc {
	return map[string]Doc{
		"widget.session.destroy": {
			Title: "End the session",
			Use: "Signs the visitor out: the session's token stops working at once and its open streams " +
				"close. Send the session token as a bearer token. Ending a session that already ended answers " +
				"the same.",
		},
		"widget.session.show": {
			Title: "Resume the session",
			Use: "Returns the current session when a page loads with a stored token: who the visitor is, " +
				"how many replies wait unread and which conversations are open. Works for anonymous sessions " +
				"only; a signed-in session must send a fresh identity token with the session update instead.",
			Returns: "the session, without its token.",
		},
		"widget.session.update": {
			Title: "Refresh a signed-in session",
			Use: "Proves a signed-in session's customer again with a fresh identity token, which extends " +
				"the session and updates the customer from the token. Send the session token as a bearer " +
				"token; an expired signed-in session is accepted here and revived. A token for anyone else is " +
				"refused and the session stays as it was.",
			Returns: "the session with its new expiry, without its token.",
			Params: map[string]string{
				"user_jwt": "A fresh identity token your backend signed for the same customer the " +
					"session belongs to, 1 to 16384 characters, such as eyJhbGciOi...",
			},
		},
	}
}

/*
messengerDocs17 returns entries from widget.sessions.store on.
*/
func messengerDocs17() map[string]Doc {
	return map[string]Doc{
		"widget.sessions.store": {
			Title: "Start a session",
			Use: "Starts a messenger session the first time a visitor does something, and returns the " +
				"token every later request sends as Authorization: Bearer. Without user_jwt the session is " +
				"anonymous, if the messenger allows that; with one the visitor is signed in once the token " +
				"passes every check. A bad token is refused, never treated as anonymous.",
			Returns: "the session with its token, shown only this once; a signed-in session that was " +
				"extended instead comes back without one.",
			Params: messengerParams1(),
		},
	}
}

/*
messengerDocs18 returns entries from widget.ticket-types.index on.
*/
func messengerDocs18() map[string]Doc {
	return map[string]Doc{
		"widget.ticket-types.index": {
			Title: "List request types",
			Use: "Lists the kinds of request the visitor may open from the messenger. Needs a messenger " +
				"session. Use a type's id as type_id when opening a request.",
			Returns: "the list of request types a customer may open.",
		},
		"widget.tickets.index": {
			Title: "List the visitor's requests",
			Use: "Lists the requests the visitor opened, or their company's. Needs a messenger session. " +
				"Company requests show only to a visitor signed in with a verified identity who belongs to a " +
				"company, and only for request types shared with the company.",
			Returns: "the list of requests as the customer sees them.",
			Params: map[string]string{
				"open": "true to keep only requests not yet resolved, such as open=true. Leaving it " +
					"out lists all of them.",
				"scope": "Whose requests to list: mine for the visitor's own, or company for their " +
					"company's too. Leaving it out means mine.",
			},
		},
		"widget.tickets.show": {
			Title: "Get a request",
			Use: "Returns one of the visitor's requests with the states it went through. Needs a " +
				"messenger session.",
			Returns: "the request with its state history.",
			Params: map[string]string{
				"ticket": "The request's id, such as tkt_6b2n8r, or its reference, such as T-482 or " +
					"482.",
			},
		},
	}
}

/*
messengerDocs19 returns entries from widget.tickets.store on.
*/
func messengerDocs19() map[string]Doc {
	return map[string]Doc{
		"widget.tickets.store": {
			Title: "Open a request",
			Use: "Submits a request form: dimmi checks it, starts a conversation with the title and " +
				"description as the visitor's first message, and puts a ticket on it. Needs a messenger " +
				"session. Replies then go through that conversation's messages; the same client_id sent twice " +
				"makes one request.",
			Returns: "the request as the customer sees it.",
			Params: map[string]string{
				"attachment_ids": "Up to 10 ids of files this session finished uploading, such as " +
					"[\"att_4k2j9x\"], attached to the request's first message.",
				"client_id": "A UUID the client makes up for this form, such as " +
					"6f1c2c1e-8a4b-4d1f-9c3e-2b7a5d9e0f11. Required; sending it again returns the first request.",
				"description": "The details of the request, up to 10000 characters, such as \"The " +
					"March invoice shows the wrong amount\". Optional.",
				"fields": "Object of values for the type's fields a customer may fill, keyed by field " +
					"key, such as {\"order_number\": \"A-1042\"}. A file field holds a list of attachment ids.",
				"page": "The page the visitor is on, as an object with url, title and referrer. " +
					"Optional.",
				"page.referrer": "Address of the page that led here, such as https://google.com/; " +
					"anything that is not an http or https address is dropped.",
				"page.title": "The page's title, such as Pricing; text beyond 300 characters is cut.",
				"page.url": "Full http or https address of the page, such as https://acme.com/pricing; " +
					"kept up to 2048 characters, with query values that look like credentials removed.",
				"title": "Short summary of the request, 1 to 200 characters, such as \"Refund for " +
					"order A-1042\". Required.",
				"type_id": "Id of a request type customers may open, taken from the request types " +
					"list, such as tktt_3m8v1c. Required.",
			},
		},
	}
}

/*
messengerDocs20 returns entries from widget.updates.index on.
*/
func messengerDocs20() map[string]Doc {
	return map[string]Doc{
		"widget.updates.index": {
			Title: "Poll for updates",
			Use: "Polling fallback for clients whose live stream is blocked: returns what changed in the " +
				"visitor's conversations since the last poll, at most 100 changes. Needs a messenger session. " +
				"Each answer carries the cursor for the next poll and how long to wait; when has_more is true, " +
				"poll again at once.",
			Returns: "the changes since the cursor, with the next cursor, the wait before the next poll " +
				"and whether more remain.",
			Params: map[string]string{
				"since": "The cursor from the previous answer's meta, sent back as is. Leave it out on " +
					"the first poll.",
			},
		},
	}
}

/*
messengerParams1 explains the parameters and fields of
widget.sessions.store.
*/
func messengerParams1() map[string]string {
	out := map[string]string{
		"app_id": "The messenger's app id from its install snippet, such as wgt_7h2k9d. An " +
			"unknown, malformed or deleted id answers not found.",
		"context": "Flat object of details the page passes along, such as {\"plan\": \"pro\"}: " +
			"text, number or true/false values only, at most 30 keys and 4 KB, and null removes a key.",
		"locale": "The visitor's language tag, such as en or pt-BR, up to 64 characters. Text " +
			"that is no language tag is ignored.",
		"page": "The page the visitor is on, as an object with url, title and referrer. " +
			"Optional.",
		"page.referrer": "Address of the page that led here, such as https://google.com/; " +
			"anything that is not an http or https address is dropped.",
		"page.title": "The page's title, such as Pricing; text beyond 300 characters is cut.",
		"page.url": "Full http or https address of the page, such as https://acme.com/pricing; " +
			"kept up to 2048 characters, with query values that look like credentials removed.",
		"pages": "Up to 10 pages the visitor saw before the session started, oldest first, " +
			"each an object with url, title and referrer.",
		"pages.referrer": "Address of the page that led here, such as https://google.com/; " +
			"anything that is not an http or https address is dropped.",
		"pages.title": "The page's title, such as Pricing; text beyond 300 characters is cut.",
	}
	maps.Copy(out, messengerParams1More())

	return out
}

/*
messengerParams1More explains the rest of them.
*/
func messengerParams1More() map[string]string {
	return map[string]string{
		"pages.url": "Full http or https address of the page, such as " +
			"https://acme.com/pricing; kept up to 2048 characters, with query values that look like " +
			"credentials removed.",
		"previous_token": "The session token the client held before signing in, such as " +
			"dmi_ws_ followed by 43 characters. With user_jwt, that anonymous session's visitor becomes " +
			"the customer and the old session ends.",
		"sdk": "The client library making the call, as an object with name and version. " +
			"Optional.",
		"sdk.name":    "Name of the client library, up to 64 characters, such as @dimmi/browser.",
		"sdk.version": "Version of the client library, up to 64 characters, such as 1.4.0.",
		"test_token": "Optional text of up to 128 characters. Starting a session does not read " +
			"it.",
		"timezone": "The visitor's IANA time zone name, such as Europe/Berlin. An unknown name " +
			"is ignored.",
		"user_jwt": "Identity token your backend signed for the signed-in visitor, up to 16384 " +
			"characters, such as eyJhbGciOi.... Leave it out for an anonymous session.",
		"website": "Hidden anti-bot form field that real visitors never fill; leave it out or " +
			"empty. A filled one marks the session as a bot and what it starts lands as spam.",
	}
}
