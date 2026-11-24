/*
|--------------------------------------------------------------------------
| Written Schema Texts: Member To NotificationDelivery
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTextsFromMember returns the texts from Member on.
*/
func schemaTextsFromMember() map[string]map[string]string {
	return map[string]map[string]string{
		"Member": {
			"conversation_access_unassigned": "True when a member with narrowed access also sees " +
				"conversations nobody holds.",
			"conversation_capacity": "The most open conversations the member holds at once, from 1 to " +
				"500. Null for no limit.",
			"permissions": "The permissions the member holds in this workspace, each written " +
				"area:action, such as settings:read or integrations:manage.",
			"role":   "The member's role in the workspace: owner, admin or member.",
			"status": "Whether the membership is in use: active or deactivated.",
		},
		"MemberImport": {
			"created":   "How many addresses that were not yet teammates the import invited.",
			"dry_run":   "True when this was only a trial and nothing was saved.",
			"failed":    "How many rows were refused.",
			"rows":      "The outcome of each row of the file, in order.",
			"unchanged": "How many rows matched a teammate who needed no change.",
			"updated":   "How many existing teammates the import changed.",
		},
		"MemberImportRow": {
			"changes": "The fields this row changed on an existing teammate: any of role, status and " +
				"teams. Empty when none.",
			"email":  "The email address in this row.",
			"result": "What became of the row: created, updated, unchanged or failed.",
			"row":    "The row's line number in the file.",
		},
		"MemberSummary": {
			"avatar_url": "Address of the teammate's picture, or null when they have none.",
			"name":       "The teammate's name.",
		},
		"Membership": {
			"joined_via": "How the member came to the workspace, such as created, invitation or cli. " +
				"Null when not recorded.",
			"last_accessed_at": "When the caller last opened this workspace. Null when they never " +
				"have.",
			"role":   "The caller's role in this workspace: owner, admin or member.",
			"status": "Whether the membership is in use: active or deactivated.",
		},
	}
}

/*
schemaTextsFromMembershipWorkspace returns the texts from
MembershipWorkspace on.
*/
func schemaTextsFromMembershipWorkspace() map[string]map[string]string {
	return map[string]map[string]string{
		"MembershipWorkspace": {
			"deleted_at":  "When the workspace was moved to the trash. Null when it is not deleted.",
			"name":        "The workspace's name.",
			"purge_after": "When a trashed workspace is deleted for good. Null when it is not deleted.",
			"slug":        "The workspace's short name used in addresses, such as acme.",
		},
		"Mention": {
			"name": "The mentioned teammate's or team's name as it was when the note was written.",
			"type": "What the note mentions: member or team.",
		},
		"MentionableMember": {
			"availability": "Whether the teammate is taking new conversations: online or away.",
			"avatar_url":   "Address of the teammate's picture, or null when they have none.",
			"can_see": "True when the teammate is allowed to see this conversation; false means a " +
				"mention would name someone who cannot open it.",
			"name": "The teammate's name.",
		},
		"MentionableTeam": {
			"member_count": "How many teammates are in the team.",
			"name":         "The team's name.",
		},
		"MentionablesBody": {
			"members": "The teammates a note on this conversation may mention, at most 10, matched by " +
				"name.",
			"teams": "The teams a note on this conversation may mention, at most 10. Empty while the " +
				"workspace has team mentions off.",
		},
		"Message": {
			"message": "A plain sentence for a person to read, saying what happened.",
		},
		"MessageAI": {
			"generated": "True when the AI agent wrote this message.",
			"outcome": "How the agent's run ended, such as answered, handoff, unknown, greeted or " +
				"refused. Left out when not recorded.",
		},
	}
}

/*
schemaTextsFromMessageAI returns the texts from MessageAI on.
*/
func schemaTextsFromMessageAI() map[string]map[string]string {
	return map[string]map[string]string{
		"MessageAI": {
			"run_id": "Id of the agent run that wrote the message, starting with air_. Shown to the " +
				"team only; left out for customers.",
			"sources": "The sources the answer cites, each with a title and address; empty when it " +
				"cites none.",
		},
		"MessageAIAction": {
			"name":  "The key of the action the agent ran.",
			"title": "The action's name as people read it.",
		},
		"MessageAIActionCall": {
			"arguments": "The values the agent passed to the action. Shown to the team only; left out " +
				"for customers.",
			"decision_reason": "Why a teammate approved or declined the call, in their words. Shown to " +
				"the team only; left out when none was given.",
			"expires_at": "When a call waiting for a decision lapses. Shown to the team only; left out " +
				"when it does not expire.",
			"result_summary": "What the action came to, in words. Shown to the team only; left out " +
				"until there is a result.",
			"state": "Where the call stands: awaiting_verification, awaiting_confirmation, " +
				"pending_approval, approved, declined, expired, canceled, running, succeeded or failed.",
			"summary": "One line saying what the change is, written from its arguments.",
		},
		"MessageAIConfirmation": {
			"action_call_id": "Id of the action call waiting for the customer's yes, starting with " +
				"aax_. Send it to confirm or cancel the change.",
			"details": "The lines of the confirmation card, each a label and a value; empty when there " +
				"are none.",
			"expires_at": "When the confirmation lapses if the customer does not answer. Null when it " +
				"does not expire.",
			"status": "Where the call stood when the card was written, such as awaiting_confirmation, " +
				"succeeded, canceled or expired.",
			"text": "The summary of the change the customer is asked to confirm, written from its " +
				"arguments.",
		},
	}
}

/*
schemaTextsFromMessageAIConfirmation returns the texts from
MessageAIConfirmation on.
*/
func schemaTextsFromMessageAIConfirmation() map[string]map[string]string {
	return map[string]map[string]string{
		"MessageAIConfirmation": {
			"title": "The card's title: the name of the change.",
		},
		"MessageAIDetail": {
			"label": "Name of one line on the confirmation card the customer is asked to approve, such " +
				"as \"Amount\".",
			"value": "The value shown beside that line's label on the confirmation card.",
		},
		"MessageAIPerson": {
			"name": "Display name of the teammate who approved or declined the action call.",
		},
		"MessageAISource": {
			"excerpt": "A line or two of what the cited source says: a help article's description, or the " +
				"start of the passage the answer drew on. Left out for a source customers may not read.",
			"slug": "Slug of the published help article the source is, for opening it in a help center " +
				"or the messenger. Left out when the source is not a public help article.",
			"title": "Title of a source the AI answer cites, such as a help article's title.",
			"url":   "Web address of the cited source, for checking the answer against it.",
		},
		"MessageBroadcast": {
			"name": "Name of the broadcast this message went out with.",
		},
		"MessageOriginal": {
			"html": "Sanitized HTML of the received message. Null when the mail had no HTML part.",
			"html_too_large": "True when the mail's HTML was bigger than what was kept; the raw email " +
				"still has all of it.",
			"raw_available": "True when the caller can download the raw email of this message.",
			"text":          "Full plain text of the received message, longer than the body the timeline shows.",
			"text_truncated": "True when even this full text was cut because the mail held more than " +
				"was kept; the raw email still has all of it.",
		},
		"MessageSummary": {
			"excerpt": "Start of the message as plain text, cut to 140 characters with a trailing … " +
				"when longer. Empty when the message has no body.",
			"type": "Kind of timeline entry: message, note or event.",
		},
	}
}

/*
schemaTextsFromMessengerAudiences returns the texts from MessengerAudiences on.
*/
func schemaTextsFromMessengerAudiences() map[string]map[string]string {
	return map[string]map[string]string{
		"MessengerAudiences": {
			"rules": "The messenger's audience rules as stored, a JSON object with launcher (who sees " +
				"the launcher), start (who may start a conversation) and tabs (who sees each tab).",
		},
		"MetaStruct": {
			"next_cursor": "Cursor of the next page; send it back as the cursor to read on. Null on " +
				"the last page.",
			"per_page":    "Page size: the most rows one page of this list holds.",
			"prev_cursor": "Cursor of the previous page. Null on the first page.",
			"total": "How many rows match in all, across every page. Left out when the endpoint did " +
				"not count.",
		},
		"MetaStruct1": {
			"secret": "The new webhook endpoint's signing secret. Shown only this once, in the answer " +
				"that created the endpoint, so store it now.",
		},
		"MetaStruct2": {
			"next_cursor": "Cursor of the next page of conversations; send it back as the cursor to " +
				"read on. Null on the last page.",
		},
		"MetaStruct3": {
			"changed": "Names of the conversation fields this request changed, in the order they were " +
				"applied, such as [\"status\", \"add_tags\"]. Empty when nothing changed.",
		},
		"MetaStruct5": {
			"stale":    "True when the stored draft was newer than the one sent, so nothing was written.",
			"upserted": "True when an existing draft was updated rather than a new one created.",
		},
		"MetaStruct6": {
			"next_cursor": "Cursor of the next page of notifications; send it back as the cursor to " +
				"read on. Null on the last page.",
			"unread_count": "How many of the caller's notifications are not marked read.",
			"unseen_count": "How many of the caller's notifications they have not yet seen in the " +
				"bell.",
		},
	}
}

/*
schemaTextsFromMetaStruct7On returns the texts from MetaStruct7 on.
*/
func schemaTextsFromMetaStruct7On() map[string]map[string]string {
	return map[string]map[string]string{
		"MetaStruct7": {
			"next_cursor": "Cursor of the next page of delivery log lines; send it back as the cursor " +
				"to read on. Null on the last page.",
		},
		"Named": {
			"name": "Display name of the inbox, team or member this short form stands for.",
		},
		"Node": {
			"children": "Items under this one in the knowledge source's tree, each shaped the same " +
				"way. Empty for a document.",
			"kind": "What the item is at the connected service, such as collection, folder, page, " +
				"article or file.",
			"title": "Name of the item as the connected service shows it.",
		},
		"NoteContact": {
			"email": "Email address of the signed-in reader who left the note. Null when the contact " +
				"has none.",
			"name": "Name of the signed-in reader who left the note. Null when the contact has none.",
		},
		"Notification": {
			"action_call_id": "Id of the AI action call the notification asks about (starts with " +
				"aax_). Null when it is not about one.",
			"actor_name": "Name of who caused the notification, such as the customer or teammate. " +
				"Empty when there is none.",
			"body": "Text of the notification under its title, such as a preview of what was said.",
			"conversation_id": "Id of the conversation the notification is about (starts with conv_); " +
				"send it wherever a conversation is asked for. Null when it is not about one.",
			"conversation_number": "Number of the conversation the notification is about. Left out " +
				"when it is not about one.",
			"count": "How many alerts this one bell row stands for.",
			"link":  "Where in the app the notification opens.",
			"read":  "True once the member marked it read, or read the conversation it is about.",
		},
	}
}

/*
schemaTextsFromNotification returns the texts from Notification on.
*/
func schemaTextsFromNotification() map[string]map[string]string {
	return map[string]map[string]string{
		"Notification": {
			"resolved": "How the action call it asks about ended once decided, an action call state " +
				"such as approved or declined. Left out until then and for other notifications.",
			"seen":  "True once the member has seen it in the bell.",
			"title": "Headline of the notification.",
			"trigger": "What the notification is about: new_conversation, conversation_assigned, " +
				"customer_replied, mentioned, ai_handoff_requested, ai_action_approval_requested, " +
				"snooze_expired, team_joined or operator_routine_finished.",
		},
		"NotificationCell": {
			"channel": "The channel this cell is for: in_app, email, push or slack.",
			"default": "Whether this trigger is on for this channel when the member has made no " +
				"choice.",
			"enabled": "Whether the member currently gets this trigger on this channel.",
			"locked":  "True when this trigger cannot be turned off on this channel.",
			"trigger": "Key of the trigger this cell is for, such as customer_replied; matches a key " +
				"in triggers.",
		},
		"NotificationCounts": {
			"unread_count": "How many of the caller's notifications are not marked read.",
			"unseen_count": "How many of the caller's notifications they have not yet seen in the " +
				"bell.",
		},
		"NotificationDeliveriesOutputBody": {
			"data": "One page of the delivery log: each line is one alert on one channel, with how it " +
				"ended and why.",
		},
		"NotificationDelivery": {
			"attempts": "How many times dimmi tried this delivery.",
			"channel":  "Channel the alert went on: in_app, email, push or slack.",
			"delivery_id": "Id shared by every channel's line of the same alert (a UUID), for grouping " +
				"them.",
			"detail": "More about a failure, such as the error text, at most 300 characters. Null when " +
				"there is nothing to add.",
			"link": "Where in the app the alert opens.",
		},
	}
}
