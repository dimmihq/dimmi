/*
|--------------------------------------------------------------------------
| Tool Catalog: Inbox, Conversations
|--------------------------------------------------------------------------
|
| The entries for one conversation: reading it and its customer,
| starting one, changing its status, holder, priority and tags, and the
| reader's own read state. None of these changes reaches the customer,
| except starting a conversation as a teammate.
|
*/

package mcpserver

/*
conversationDocs returns the entries for reading and starting a
conversation.
*/
func conversationDocs() map[string]Doc {
	return map[string]Doc{
		"conversations.show": {
			Title: "Get a conversation",
			Use: "Reads one conversation by id (conv_...) or number (1042 for #1042): status, priority, subject, " +
				"customer, channel, inbox, assignee, team, tags, AI agent state and the latest message. It " +
				"does not hold the thread; read that with dimmi_conversations_messages_list.",
			Returns: "the conversation.",
			Next: []string{
				"dimmi_conversations_messages_list: read the thread",
				"dimmi_conversations_context_get: see who the customer is",
			},
		},
		"conversations.context.show": {
			Title: "Get a conversation's customer context",
			Use: "Reads what the panel beside a conversation shows about its customer in one call: the contact " +
				"with custom attributes, their company, the ways they reach you, where this conversation " +
				"started, their recent conversations and how many possible duplicates they have.",
			Returns: "the contact, company, identities, source, past conversations and duplicates count.",
			Next:    []string{"dimmi_contacts_get: read the contact in full"},
		},
		"conversations.store": {
			Title: "Start a conversation",
			Use: "Starts a conversation with its first message. As a teammate (author member, the default) you " +
				"reach out first on a channel that can start threads, such as an email address, and the " +
				"customer receives the message. As author contact you record what a customer wrote, on an " +
				"inbox's API channel, and it waits for the team. Use only when asked.",
			Returns: "the new conversation with its id and number.",
			Next:    []string{"dimmi_conversations_messages_list: confirm the first message and its delivery"},
			Risk:    new(RiskReach),
			Params: map[string]string{
				"body": "The first message in Markdown. As a teammate, the customer receives exactly this text.",
				"contact": "The customer: {id} (ctc_...), or {email} or {external_id} with an optional name, " +
					"added when new.",
				"author": "member (default): you write first and the customer receives it. contact: you record " +
					"what the customer wrote.",
				"channel_connection_id": "The channel to send on (chn_..., an email address's " +
					"channel_connection from dimmi_email_addresses_list). Required with author member.",
				"inbox_id":    "With author contact: the inbox (inbox_...) whose API channel takes the message.",
				"assignee_id": "mem_..., me, or null for nobody. A teammate's conversation is theirs by default.",
			},
			Hint: "As a teammate, pass channel_connection_id of a channel that can start threads, such as email.",
		},
	}
}

/*
conversationChangeDocs returns the entry for changing a conversation.
*/
func conversationChangeDocs() map[string]Doc {
	return map[string]Doc{
		"conversations.update": {
			Title: "Update a conversation",
			Use: "Changes several things in one step: status (close, reopen, snooze), assignee, team, " +
				"priority, subject, inbox, tags, spam and your unread mark. Fields left out stay; if one fails " +
				"nothing changes. Nothing here reaches the customer: to answer them use " +
				"dimmi_conversations_replies_create.",
			Returns: "the conversation as it is now, and meta.changed with the fields that changed.",
			Next:    []string{"dimmi_conversations_neighbors_list: move on to the next conversation in the queue"},
			Params: map[string]string{
				"status": "open, snoozed or closed. A closed conversation cannot be snoozed: reopen it first.",
				"snoozed_until": "With status=snoozed, exactly one of snoozed_until, snooze_preset or " +
					"snooze_option. An RFC 3339 time, from a minute to a year ahead.",
				"snooze_preset": "later_today (three hours on), tomorrow or next_week (at nine) or one_week " +
					"(seven days on, at nine), in your time zone.",
				"snooze_option": "The key of one of this conversation's snooze choices, from " +
					"dimmi_snooze_options_list.",
				"note":        "Only with status=closed: a note for the team, kept with the closing.",
				"assignee_id": "A teammate (mem_..., from dimmi_members_list), me, or null to hand it to nobody.",
				"team_id":     "A team (team_..., from dimmi_teams_list), or null for no team.",
				"priority":    "none, low, medium, high or urgent.",
				"inbox_id":    "Moves the conversation to this inbox (inbox_...).",
				"unread":      "true marks it unread for you alone; false marks it read.",
				"spam":        "true moves it to the spam view and out of every other; false brings it back.",
				"add_tags":    "Tags by id (tag_...) or name; a name no tag has yet becomes a new tag. Up to 50.",
				"remove_tags": "Tag ids (tag_...) to take off.",
			},
			Hint: "Snoozing needs exactly one of snoozed_until, snooze_preset or snooze_option.",
		},
	}
}

/*
conversationStateDocs returns the entries for a conversation's tags
and the reader's read state.
*/
func conversationStateDocs() map[string]Doc {
	return map[string]Doc{
		"conversations.tags.store": {
			Title:   "Tag a conversation",
			Use:     "Adds tags to a conversation, by id or by name; a name no tag has yet becomes a new tag.",
			Returns: "the conversation with its tags as they are now.",
			Params: map[string]string{
				"tags":       "Tag ids (tag_...) or names, up to 50. Names match whatever their case.",
				"message_id": "Optional message (msg_...) that prompted the tags.",
			},
		},
		"conversations.tags.destroy": {
			Title:   "Remove a tag from a conversation",
			Use:     "Takes one tag off a conversation. The tag itself stays in the workspace.",
			Returns: "the conversation with its tags as they are now.",
			Risk:    new(RiskWrite),
		},
		"conversations.read-state.show": {
			Title:   "Get your read state",
			Use:     "Reads how far you have read a conversation. Read state is yours alone.",
			Returns: "the last entry you read, when, and whether the conversation is unread for you.",
		},
		"conversations.read-state.update": {
			Title: "Mark a conversation read or unread",
			Use: "Marks a conversation read for you, up to the latest entry or the one you name, or unread so " +
				"the newest entry from someone else shows as new. Only you see your read state.",
			Returns: "your read state as it is now.",
			Params: map[string]string{
				"last_read_message_id": "Read up to this entry (msg_...). Leave everything out to read to the end.",
				"unread":               "true to mark it unread. Not together with last_read_message_id.",
			},
		},
	}
}

/*
conversationTrashDocs returns the entries for the trash and for what
stops email to a customer.
*/
func conversationTrashDocs() map[string]Doc {
	return map[string]Doc{
		"conversations.destroy": {
			Title:   "Move a conversation to the trash",
			Use:     "Takes a conversation out of every list and count. Its messages stay, so it can be put back.",
			Returns: "nothing.",
			Next:    []string{"dimmi_conversations_restore_create: put it back"},
			Risk:    new(RiskWrite),
		},
		"conversations.restore.store": {
			Title:   "Restore a conversation from the trash",
			Use:     "Puts a trashed conversation back where it was.",
			Returns: "the conversation as it is now.",
		},
		"email-suppressions.index": {
			Title:   "List what stops email to a customer",
			Use:     "Lists the bounces, complaints and blocks that stop mail to a contact's address.",
			Returns: "each suppression with its reason, the kind of mail it stops and when it lapses.",
			Params:  map[string]string{"filter[contact]": "The contact (ctc_...) whose address to read."},
			Next:    []string{"dimmi_email_suppressions_delete: let mail reach the address again"},
		},
		"email-suppressions.destroy": {
			Title:   "Lift an email suppression",
			Use:     "Lets mail reach a suppressed address again. Lifting a complaint takes an owner.",
			Returns: "nothing.",
			Risk:    new(RiskWrite),
		},
	}
}
