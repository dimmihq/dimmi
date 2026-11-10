/*
|--------------------------------------------------------------------------
| Tool Catalog: Inbox
|--------------------------------------------------------------------------
|
| The entries for conversations, their messages, replies, notes, drafts,
| views, snoozes, tags and inboxes. Triage is the job agents do most, so
| these entries say plainly what a customer sees and what only the team
| sees, how conversations are named, and which filters a list takes.
|
*/

package mcpserver

import "maps"

/*
inboxDocs returns the entries for conversations, their messages,
replies, notes, drafts, views, snoozes, tags and inboxes.
*/
func inboxDocs() map[string]Doc {
	out := map[string]Doc{}
	for _, part := range []map[string]Doc{
		conversationListDocs(), conversationDocs(), conversationChangeDocs(), conversationStateDocs(),
		timelineDocs(), writingDocs(), messageChangeDocs(), collaborationDocs(), viewDocs(), preferenceDocs(),
		draftDocs(), snoozeDocs(), tagDocs(), inboxSetupDocs(), inboxAccessDocs(), inboxAIDocs(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
conversationListFilters explains the filters the conversation list and
its neighbors share.
*/
func conversationListFilters() map[string]string {
	return map[string]string{
		"filter[view]": "Which list: mine (the default: assigned to you), unassigned, all, mentions (notes that " +
			"mention you), spam, deleted (the trash), member:mem_... for one teammate's, team:team_..., " +
			"inbox:inbox_..., or a saved view's id (cvw_..., from dimmi_conversation_views_list). Spam and " +
			"deleted conversations show only in their own views.",
		"filter[status]": "open (the default), snoozed or closed, several separated by commas, or any for every " +
			"status. A saved view brings its own status unless you pass one.",
		"filter[assignee]": "A teammate's id (mem_...), me, or none for conversations nobody holds. It wins over " +
			"the holder the view implies.",
		"filter[team]":               "A team's id (team_...), or none for conversations no team holds.",
		"filter[inbox]":              "An inbox's id (inbox_...).",
		"filter[contact]":            "A contact's id (ctc_...): only that customer's conversations.",
		"filter[company]":            "A company's id (cmp_...).",
		"filter[channel]":            "widget (the messenger), email or api.",
		"filter[channel_connection]": "Channel connection ids (chn_...), such as one email address's.",
		"filter[priority]":           "none, low, medium, high or urgent, several separated by commas.",
		"filter[tag]":                "Tag ids (tag_...) the conversation must all carry, up to 5.",
		"filter[tag_any]":            "Tag ids of which the conversation carries at least one, up to 20.",
		"filter[created_after]":      "An RFC 3339 time, such as 2026-11-01T00:00:00Z: started at or after it.",
		"filter[created_before]":     "An RFC 3339 time: started before it.",
		"filter[waiting]":            "true for conversations waiting for a reply from the team.",
		"filter[unread]":             "true for conversations with entries you have not read.",
		"filter[assignment_waiting]": "true for conversations automatic assignment could not place yet.",
		"filter[ai]": "handed_off (the AI agent passed it to the team), active (the AI agent is still " +
			"answering and has replied) or any (the AI agent took part at all).",
		"q": "Search words, \"a phrase\", -word to leave out, and is:open, is:closed, is:snoozed, is:unread, " +
			"is:unassigned, is:mine, is:priority, priority:high or priority:>=medium. A number such as #1042 " +
			"finds that conversation whatever its status.",
		"sort": "-last_message_at (the default), last_message_at, waiting_since (longest waiting first), " +
			"-created_at, created_at, -priority, -priority,waiting_since, or snoozed_until when listing only " +
			"snoozed conversations (then the default).",
		"include": "viewers to add which teammates are looking at each conversation right now.",
	}
}

/*
conversationListDocs returns the entries for listing conversations,
stepping through a list and counting the views.
*/
func conversationListDocs() map[string]Doc {
	return map[string]Doc{
		"conversations.index": {
			Title: "List conversations",
			Use: "Lists a page of one inbox view. With no filters it shows only your own open conversations " +
				"(filter[view]=mine), so pass filter[view]=all to see the whole inbox. Each row has an id " +
				"(conv_...) and a #number; either works in every conversation tool. Use filters rather than " +
				"reading conversations one by one.",
			Returns: "a page of conversations, newest activity first by default, each with status, priority, " +
				"assignee, team, tags, AI agent state and latest message, and next_cursor for more.",
			Next: []string{
				"dimmi_conversations_messages_list: read a conversation's thread",
				"dimmi_conversations_update: assign, tag, prioritize, snooze or close",
			},
			Params: conversationListFilters(),
			Hint:   "A cursor works only with the filters and sort it came from: drop cursor after changing them.",
		},
		"conversations.neighbors.index": {
			Title: "Find the next and previous conversation",
			Use: "Finds the conversations just before and after one in a list, given the same filters and sort " +
				"as dimmi_conversations_list. Use it to work through a queue one at a time; it still works once " +
				"the conversation left the list, such as right after you closed it.",
			Returns: "prev and next, each a short conversation (id, number, subject, status) or null.",
			Next:    []string{"dimmi_conversations_get: open the next conversation"},
			Params:  conversationListFilters(),
		},
		"conversation-counts.index": {
			Title: "Count conversations per view",
			Use: "Counts the conversations in each inbox view: yours (open, unread, snoozed), unassigned, all, " +
				"mentions, spam, and each inbox, team, teammate and pinned saved view. Counts stop at a cap. " +
				"Use it for a quick picture of the workload before listing.",
			Returns: "the counts by view, with the time they were worked out.",
			Next:    []string{"dimmi_conversations_list: list the view that needs attention"},
		},
		"search.index": {
			Title: "Search conversations and contacts",
			Use: "Quick search across every status (never spam or the trash): a few conversations whose subject " +
				"or messages match, each with an excerpt, and a few contacts by name, email or id. A #number " +
				"or conv_ id answers that conversation as jump. For a full filtered list use " +
				"dimmi_conversations_list with q.",
			Returns: "jump (an exact hit or null), conversations with the matching message, and contacts.",
			Next:    []string{"dimmi_conversations_get: open a hit", "dimmi_contacts_get: open a contact"},
			Params: map[string]string{
				"q":     "Words, \"a phrase\", -word to leave out, a #number or an id.",
				"limit": "How many results of each kind to return.",
			},
		},
	}
}
