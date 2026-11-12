/*
|--------------------------------------------------------------------------
| Tool Catalog: Inbox, Views And Drafts
|--------------------------------------------------------------------------
|
| The entries for how a teammate arranges their inbox: saved views the
| team shares or one member keeps, personal preferences, the drafts a
| member is still writing, and the snooze choices a workspace offers.
| Drafts are never sent, so they are the safe way to prepare a reply.
|
*/

package mcpserver

import "maps"

/*
viewDefinition explains the conditions a saved view holds.
*/
func viewDefinition() string {
	return "The conditions as JSON: {\"op\":\"and\",\"groups\":[{\"op\":\"or\",\"conditions\":[{\"field\":" +
		"\"priority\",\"op\":\"in\",\"value\":[\"high\",\"urgent\"]}]}]}, op and or or, up to 5 groups and 20 " +
		"conditions. Fields: status, channel, channel_connection, inbox, contact, assignee (me works), team, " +
		"tag, company, priority, unread, waiting, imported, number, message_count, subject, created_at, " +
		"last_message_at, waiting_since, snoozed_until, closed_at. Ops: in and not_in with a list of values " +
		"or ids, exists and not_exists, eq for true or false, eq, gt and lt for numbers, eq, gte and lte for " +
		"priority, contains for subject, before and after with an RFC 3339 time, older_than and newer_than " +
		"with a duration such as P3D or PT2H."
}

/*
viewDocs returns the entries for saved views.
*/
func viewDocs() map[string]Doc {
	return map[string]Doc{
		"conversation-views.index": {
			Title: "List saved views",
			Use: "Lists the saved views you can open, the shared ones and your own personal ones, on one page. " +
				"Pass a view's id as filter[view] to dimmi_conversations_list to see its conversations.",
			Returns: "the views with their name, scope, conditions, sort and status.",
			Next:    []string{"dimmi_conversations_list: list a view's conversations with filter[view]=cvw_..."},
			Params:  map[string]string{"q": "Only views whose name holds this text."},
		},
		"conversation-views.show": {
			Title:   "Get a saved view",
			Use:     "Reads one saved view with its conditions, and warnings for conditions that match nothing.",
			Returns: "the view and its warnings.",
		},
		"conversation-views.store": {
			Title: "Create a saved view",
			Use: "Saves a named list of conversations. A personal view is yours alone (50 at most); a shared " +
				"view shows for the whole team (200 at most) and needs views:manage_shared. The conditions " +
				"are checked as the list will run them.",
			Returns: "the new view.",
			Next:    []string{"dimmi_conversations_list: open it with filter[view] set to its id"},
			Params: map[string]string{
				"scope":         "personal or shared.",
				"name":          "Up to 60 characters.",
				"definition":    viewDefinition(),
				"sort":          "A sort dimmi_conversations_list accepts, such as -priority,waiting_since.",
				"status_filter": "open, snoozed, closed or any: the statuses the view shows.",
			},
		},
		"conversation-views.update": {
			Title: "Update a saved view",
			Use: "Changes a saved view field by field; fields left out stay. Only the owner changes a personal " +
				"view, and shared views need views:manage_shared. An empty icon, sort or status_filter clears it.",
			Returns: "the view as it is now.",
			Params: map[string]string{
				"definition":    viewDefinition(),
				"status_filter": "open, snoozed, closed or any, or empty to clear.",
			},
		},
		"conversation-views.reorder": {
			Title:   "Reorder the shared views",
			Use:     "Sets the order of the shared views for the whole team. Needs views:manage_shared.",
			Returns: "the views in their new order.",
			Params:  map[string]string{"order": "Every shared view's id (cvw_...), in the new order."},
		},
	}
}

/*
preferenceDocs returns the entries for deleting saved views and for
each member's inbox preferences.
*/
func preferenceDocs() map[string]Doc {
	return map[string]Doc{
		"conversation-views.destroy": {
			Title:   "Delete a saved view",
			Use:     "Moves a saved view to the trash and frees its name. A shared view disappears for everyone.",
			Returns: "nothing.",
		},
		"inbox-preferences.show": {
			Title:   "Get your inbox preferences",
			Use:     "Reads how you arranged your inbox: the sidebar, each view's sort, status, layout and columns.",
			Returns: "your preferences, with the customer attributes you pinned.",
		},
		"inbox-preferences.update": {
			Title:   "Update your inbox preferences",
			Use:     "Saves parts of your own inbox preferences; a part sent replaces that part whole, the rest stays.",
			Returns: "your preferences as they are now.",
			Params: map[string]string{
				"views":             "Per view: sort, status, layout and columns, each a value a list accepts.",
				"pinned_attributes": "Customer attribute keys to pin beside conversations, up to 50.",
			},
		},
	}
}

/*
draftContent explains what a draft holds, with the extra arguments a
tool adds.
*/
func draftContent(extra map[string]string) map[string]string {
	params := map[string]string{
		"body": "The draft's text in Markdown. Saving it sends nothing.",
		"client_updated_at": "An RFC 3339 time of this change; use now. A save older than the stored draft " +
			"writes nothing.",
		"attachment_ids": "Up to 10 ids (att_...) of files already uploaded.",
		"recipients":     "Optional {to, cc, bcc, subject, channel_connection_id} for when it is sent.",
	}
	maps.Copy(params, extra)

	return params
}

/*
draftDocs returns the entries for drafts.
*/
func draftDocs() map[string]Doc {
	return map[string]Doc{
		"drafts.index": {
			Title: "List your drafts",
			Use: "Lists your own unsent drafts: replies, notes and new conversations you are writing. Nobody " +
				"sees another member's drafts.",
			Returns: "your drafts with their conversation, mode and text.",
			Params: map[string]string{
				"filter[conversation]": "Only drafts on this conversation, by id (conv_...); a number does not work.",
				"filter[mode]":         "reply, note or new.",
			},
		},
		"drafts.store": {
			Title: "Save a draft",
			Use: "Saves a draft the person can review and send from the dashboard; a draft is never sent and " +
				"only you see it. This is the safe way to prepare a reply without sending it. One draft per " +
				"conversation and mode.",
			Returns: "the draft, flagged stale when a newer stored one won.",
			Next:    []string{"dimmi_drafts_update: change it later"},
			Params: draftContent(map[string]string{
				"conversation_id": "The conversation (conv_..., not a number). Left out for mode new.",
				"mode":            "reply, note or new (a new conversation).",
				"upsert":          "true to update the draft already there instead of being refused.",
			}),
			Hint: "draft_exists means one is there already: pass upsert=true or use dimmi_drafts_update.",
		},
		"drafts.update": {
			Title:   "Update a draft",
			Use:     "Replaces what one of your drafts holds. It is still not sent.",
			Returns: "the draft, flagged stale when a newer stored one won.",
			Params:  draftContent(nil),
		},
		"drafts.destroy": {
			Title:   "Discard a draft",
			Use:     "Discards one of your drafts. Nothing was ever sent.",
			Returns: "nothing.",
			Risk:    new(RiskWrite),
		},
	}
}
