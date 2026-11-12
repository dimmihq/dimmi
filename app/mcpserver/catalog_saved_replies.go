/*
|--------------------------------------------------------------------------
| Tool Catalog: Saved Replies
|--------------------------------------------------------------------------
|
| The entries for saved replies and macros: text a teammate inserts
| instead of typing it again, shared with the workspace or their own,
| and the actions a macro applies to the conversation it is sent on.
|
*/

package mcpserver

/*
savedReplyActions explains the actions a macro carries.
*/
func savedReplyActions() string {
	return "Macro actions applied to the conversation when the reply is sent: {status: open, closed or snoozed; " +
		"snooze_preset: later_today, tomorrow, next_week or one_week, only when snoozing; assignee_id: mem_...; " +
		"team_id: team_...; tag_ids: up to 10 tag_...; priority: none, low, medium, high or urgent}."
}

/*
savedReplyDocs returns the entries for listing and managing saved
replies.
*/
func savedReplyDocs() map[string]Doc {
	return map[string]Doc{
		"saved-replies.index": {
			Title: "List saved replies",
			Use: "Lists the saved replies you can insert: the workspace's shared ones and your own, by title. " +
				"Use one as the text of a reply instead of writing it again.",
			Returns: "the replies with their title, text, shortcut, whether they are shared and their macro actions.",
			Params:  map[string]string{"q": "Only replies whose title or shortcut contains this text."},
		},
		"saved-replies.show": {
			Title:   "Get a saved reply",
			Use:     "Reads one saved reply with its text and macro actions.",
			Returns: "the reply.",
		},
		"saved-replies.store": {
			Title: "Create a saved reply",
			Use: "Saves text the team inserts instead of typing it again, optionally as a macro that also changes " +
				"the conversation. Personal unless shared is true, which needs permission to manage shared replies.",
			Returns: "the reply as saved.",
			Params: map[string]string{
				"body":     "Markdown. May use {{ contact.first_name | default: \"there\" }} style variables.",
				"shortcut": "A short word to find it by, unique among your replies and among shared ones.",
				"shared":   "true shares it with the whole workspace.",
				"actions":  savedReplyActions(),
			},
		},
		"saved-replies.update": {
			Title:   "Update a saved reply",
			Use:     "Changes a saved reply's title, text, shortcut, macro actions or whether it is shared.",
			Returns: "the reply as it is now.",
			Params: map[string]string{
				"shortcut": "A new shortcut, or null to remove it.",
				"actions":  savedReplyActions() + " Sending actions replaces all of them; {} makes a plain reply.",
			},
		},
		"saved-replies.destroy": {
			Title:   "Delete a saved reply",
			Use:     "Deletes a saved reply for everyone who could use it.",
			Returns: "nothing.",
		},
		"saved-reply-imports.store": {
			Title: "Import saved replies",
			Use: "Adds up to 500 saved replies at once, such as rows of a spreadsheet. All are added or none: " +
				"the first row refused stops the import.",
			Returns: "the replies added, in row order.",
			Params:  map[string]string{"replies": "Rows of {title, body, shortcut, shared}."},
		},
	}
}
