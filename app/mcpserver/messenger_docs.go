/*
|--------------------------------------------------------------------------
| The Messenger API's Operations
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
messengerDocs returns these entries, by operation.
*/
func messengerDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		messengerDocs1(), messengerDocs2(), messengerDocs3(), messengerDocs4(), messengerDocs5(),
		messengerDocs6(), messengerDocs7(), messengerDocs8(), messengerDocs9(), messengerDocs10(),
		messengerDocs11(), messengerDocs12(), messengerDocs13(), messengerDocs14(), messengerDocs15(),
		messengerDocs16(), messengerDocs17(), messengerDocs18(), messengerDocs19(), messengerDocs20(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
messengerDocs1 returns entries from widget.apps.article-views.store on.
*/
func messengerDocs1() map[string]Doc {
	return map[string]Doc{
		"widget.apps.article-views.store": {
			Title: "Count an article view",
			Use: "Tells dimmi that someone opened a help article inside the messenger, so the view is " +
				"counted for the day. Needs no session: the messenger's app id goes in the path. Answers not " +
				"found when the messenger links to no help center; an article the help center does not show is " +
				"skipped without an error.",
			Params: map[string]string{
				"article": "Slug of the article that was opened: 1 to 200 lowercase letters, digits " +
					"and hyphens, such as reset-your-password.",
			},
		},
		"widget.apps.availability.show": {
			Title: "Check team availability",
			Use: "Tells whether someone on the team is there now and how soon they usually reply. Needs " +
				"no session, only the messenger's app id, and the answer may be cached for 30 seconds.",
			Returns: "the team's availability: online, within_hours, reply_time with its wording, team " +
				"faces and the server time.",
			Params: map[string]string{
				"app": "The messenger's app id from its install snippet, such as wgt_7h2k9d. An " +
					"unknown, malformed or deleted id answers not found.",
				"locale": "Language tag for the reply time wording, such as fr or pt-BR. Falls back to " +
					"the base language, then English.",
			},
		},
		"widget.apps.changelog.show": {
			Title: "Read the changelog tab",
			Use: "Returns the newest published changelog entries of the help center this messenger links " +
				"to, newest first, at most 20. Needs no session, only the app id in the path. The list is " +
				"empty when the messenger has no help center or its changelog tab is off.",
			Returns: "the full changelog's address and up to 20 entries with slug, title, summary, label, " +
				"publish time and body blocks.",
		},
	}
}

/*
messengerDocs2 returns entries from widget.apps.config.show on.
*/
func messengerDocs2() map[string]Doc {
	return map[string]Doc{
		"widget.apps.config.show": {
			Title: "Load the messenger config",
			Use: "Returns everything a messenger needs to draw itself: appearance, texts in the visitor's " +
				"language, tabs and settings. Call it first on every page load; it needs no session and is " +
				"cacheable for a minute.",
			Returns: "the messenger's config, or an empty 304 answer when If-None-Match matches the " +
				"current version.",
			Params: map[string]string{
				"If-None-Match": "The Etag header of an earlier answer, sent back as is. When it still " +
					"matches, the answer is 304 with no body.",
				"app": "The messenger's app id from its install snippet, such as wgt_7h2k9d. An " +
					"unknown, malformed or deleted id answers not found.",
				"locale": "Language tag the visitor prefers, such as de or pt-BR. A messenger set to " +
					"one fixed language ignores it, and English is the default.",
				"v": "Any value, such as the config version 42. It is not read; it only keeps a CDN " +
					"from answering with an older copy.",
			},
		},
		"widget.apps.help.show": {
			Title: "Read the help tab",
			Use: "Returns the collections and published articles anyone may read in the help center this " +
				"messenger links to, so help shows before the visitor says anything. Needs no session, only " +
				"the app id in the path. Everything is empty when the messenger links to no help center.",
			Returns: "the help center's address, its collections, up to 200 articles with body blocks, " +
				"and up to 5 popular article slugs.",
		},
		"widget.apps.ticket-types.index": {
			Title: "List request types",
			Use: "Lists the kinds of request a visitor may open from this messenger, so the home screen " +
				"can offer them before the visitor writes. Needs no session, only the app id in the path.",
			Returns: "the list of request types a customer may open.",
		},
	}
}

/*
messengerDocs3 returns entries from widget.article-ratings.store on.
*/
func messengerDocs3() map[string]Doc {
	return map[string]Doc{
		"widget.article-ratings.store": {
			Title: "Rate a help article",
			Use: "Records whether a help article helped the visitor reading it in the messenger. Needs a " +
				"messenger session and a help center linked to the messenger. Answering again changes the " +
				"earlier answer instead of adding another.",
			Params: map[string]string{
				"article": "Slug of the article being rated: 1 to 200 lowercase letters, digits and " +
					"hyphens, such as reset-your-password.",
				"helpful": "true when the article helped the visitor, false when it did not.",
			},
		},
		"widget.article-views.store": {
			Title: "Count an article view",
			Use: "Tells dimmi the visitor opened a help article inside the messenger, so the view is " +
				"counted for the day; nothing about the visitor is kept with it. Needs a messenger session and " +
				"a help center linked to the messenger.",
			Params: map[string]string{
				"article": "Slug of the article that was opened: 1 to 200 lowercase letters, digits " +
					"and hyphens, such as reset-your-password.",
			},
		},
		"widget.attachments.store": {
			Title: "Announce a file upload",
			Use: "First of three upload steps: say which file is coming and get back where to send its " +
				"bytes. Needs a messenger session and a messenger with attachments switched on. Then send the " +
				"bytes, mark the upload uploaded, and pass its id in attachment_ids.",
			Returns: "the pending upload with its id, status and where to send the bytes.",
			Params: map[string]string{
				"content_type": "The file's media type, 1 to 255 characters, such as image/png. Types " +
					"dimmi blocks are refused.",
				"filename": "The file's name with its extension, 1 to 255 characters, such as " +
					"invoice.pdf. Blocked extensions are refused.",
				"size_bytes": "The file's exact size in bytes, at least 1, such as 48213. A size above " +
					"the messenger's attachment limit is refused.",
			},
		},
	}
}

/*
messengerDocs4 returns entries from widget.attachments.update on.
*/
func messengerDocs4() map[string]Doc {
	return map[string]Doc{
		"widget.attachments.update": {
			Title: "Finish a file upload",
			Use: "Last upload step: say the bytes arrived so dimmi checks them and marks the file " +
				"uploaded. Needs the same messenger session that announced the file, and the bytes already " +
				"sent.",
			Returns: "the finished attachment, ready to name in attachment_ids.",
			Params: map[string]string{
				"attachment": "Id of the upload this session announced, such as att_4k2j9x; any other " +
					"id answers not found.",
				"status": "The state to move the upload to. Only uploaded is accepted; any other value " +
					"is refused.",
			},
		},
		"widget.contact.show": {
			Title: "Read the visitor's profile",
			Use: "Returns what dimmi holds about the visitor behind the session. Needs a messenger " +
				"session; the session decides whose profile it is, so no id is sent.",
			Returns: "the visitor's profile.",
		},
		"widget.contact.update": {
			Title: "Update the visitor's profile",
			Use: "Lets a visitor say who they are: a name, an email to reply to, a phone number, a few " +
				"attributes. Needs a messenger session. A signed-in customer's name, email and phone come from " +
				"their app and are refused here; they may only write the attributes the workspace lets the " +
				"messenger write.",
			Returns: "the visitor's profile as it now is.",
			Params: map[string]string{
				"custom_attributes": "Object of attribute values to merge into the profile, such as " +
					"{\"plan\": \"pro\"}, at most 20 keys at once; values must fit attributes the workspace has " +
					"defined.",
				"email": "A bare email address to reply to, such as ada@example.com, at most 254 " +
					"characters. It is stored as typed by the visitor, not as proven.",
				"name": "The visitor's name, 1 to 120 characters, such as Ada Lovelace.",
				"phone": "Phone number in international format, a plus sign and 7 to 15 digits, such " +
					"as +4930123456. Spaces are removed.",
			},
		},
	}
}

/*
messengerDocs5 returns entries from widget.conversations.action-calls.show on.
*/
func messengerDocs5() map[string]Doc {
	return map[string]Doc{
		"widget.conversations.action-calls.show": {
			Title: "Get an action confirmation",
			Use: "Returns where an action the AI agent proposed in the conversation stands, to draw or " +
				"refresh its confirmation card. Needs the messenger session the conversation belongs to.",
			Returns: "the action call as the visitor sees it.",
			Params: map[string]string{
				"call": "Id of an action the agent proposed in this conversation, such as aax_5t8w2q; " +
					"any other id answers not found.",
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
			},
		},
		"widget.conversations.action-calls.update": {
			Title: "Confirm or cancel an action",
			Use: "Records the visitor's yes or no to an action the AI agent proposed in the conversation. " +
				"Needs the messenger session the conversation belongs to. Sending the same answer twice " +
				"returns where the call stands and never runs anything twice.",
			Returns: "the action call with its new state.",
			Params: map[string]string{
				"call": "Id of an action the agent proposed in this conversation, such as aax_5t8w2q; " +
					"any other id answers not found.",
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
				"status": "The visitor's answer: confirmed to go ahead, or canceled to drop the " +
					"action. Required.",
			},
		},
	}
}

/*
messengerDocs6 returns entries from widget.conversations.ai.handoff on.
*/
func messengerDocs6() map[string]Doc {
	return map[string]Doc{
		"widget.conversations.ai.handoff": {
			Title: "Ask for a person",
			Use: "Hands the visitor's conversation from the AI agent to the team: the agent writes its " +
				"handoff line and the team is told. Needs the messenger session the conversation belongs to. A " +
				"conversation already handed off stays as it is, and a session may ask 5 times a minute.",
			Params: map[string]string{
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
			},
		},
		"widget.conversations.ai.helped": {
			Title: "Mark the AI answer helpful",
			Use: "Records that the AI agent's newest answer in the conversation helped the visitor. Needs " +
				"the messenger session the conversation belongs to. Sending it again changes nothing.",
			Params: map[string]string{
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
			},
		},
		"widget.conversations.index": {
			Title: "List the visitor's conversations",
			Use: "Lists the visitor's conversations on this messenger, latest activity first, a page at a " +
				"time: 20 by default, 50 at most. Needs a messenger session. filter[status] narrows the list " +
				"to open or closed conversations.",
			Returns: "a page of the visitor's conversations with a cursor for the next page.",
		},
	}
}

/*
messengerDocs7 returns entries from widget.conversations.messages.index on.
*/
func messengerDocs7() map[string]Doc {
	return map[string]Doc{
		"widget.conversations.messages.index": {
			Title: "List a conversation's messages",
			Use: "Reads one conversation's messages a page at a time, oldest first within each page. " +
				"Needs the messenger session the conversation belongs to. Use before to scroll back in " +
				"history, or after to catch up from a message the client already holds.",
			Returns: "a page of messages, with a cursor for older ones when more remain.",
			Params: map[string]string{
				"after": "Id of a message of this conversation the client already holds, such as " +
					"msg_9d3f7a, to read the ones after it.",
				"before": "The cursor a previous page returned, sent back as is, to read the messages " +
					"older than that page.",
				"conversation": "Id of one of the visitor's own conversations on this messenger, such " +
					"as conv_8f3k2m; any other id answers not found.",
				"limit": "How many messages to return, 1 to 100, such as 50. Leaving it out, or 0, " +
					"returns 30.",
			},
		},
	}
}
