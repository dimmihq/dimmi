/*
|--------------------------------------------------------------------------
| Tool Catalog: Inbox, Tags, Snoozes And Inboxes
|--------------------------------------------------------------------------
|
| The entries for the vocabulary of tags, the snooze choices, the
| inboxes conversations land in with who may open them and how they
| assign, and what the AI agent and Operator did on a conversation.
|
*/

package mcpserver

/*
snoozeDocs returns the entries for snooze choices and lists.
*/
func snoozeDocs() map[string]Doc {
	return map[string]Doc{
		"snooze-options.index": {
			Title: "List snooze choices",
			Use: "Lists the snooze choices for one conversation, from its team's list, the workspace's or the " +
				"built-in presets, each with the moment it would end in your time zone.",
			Returns: "the choices with their key, label and end time.",
			Next:    []string{"dimmi_conversations_update: snooze with status=snoozed and snooze_option=<key>"},
		},
		"snooze-lists.index": {
			Title:   "List snooze lists",
			Use:     "Reads the snooze choices admins set for the workspace and for each team, and the built-in ones.",
			Returns: "the workspace's list, each team's list by team id, and the defaults.",
		},
		"snooze-lists.update": {
			Title: "Set a snooze list",
			Use: "Replaces the snooze choices of the workspace or of one team, whose list applies to the " +
				"conversations it holds. Sending no options removes the list, so it inherits again.",
			Returns: "the snooze lists as they are now.",
			Params: map[string]string{
				"scope": "workspace, or a team's id (team_...).",
				"options": "Up to 10 choices, each {key, label, kind}: kind preset with preset (later_today, " +
					"tomorrow, next_week or one_week); in with minutes (5 to 525600); day_at with days_ahead " +
					"(1 to 365) and at (\"09:00\"); weekday_at with weekday (1 Monday to 7 Sunday) and at.",
			},
		},
	}
}

/*
tagDocs returns the entries for the workspace's tags.
*/
func tagDocs() map[string]Doc {
	return map[string]Doc{
		"tags.index": {
			Title:   "List tags",
			Use:     "Lists the workspace's tags by name, to find a tag's id for filters and tagging.",
			Returns: "the tags with their id, name, color and icon.",
			Params: map[string]string{
				"q":       "Only tags whose name starts with this text.",
				"include": "counts to add how many records carry each tag.",
			},
		},
		"tags.store": {
			Title: "Create a tag",
			Use: "Adds a tag to the workspace. Often unnecessary: tagging a conversation by a new name creates " +
				"the tag.",
			Returns: "the tag.",
			Params: map[string]string{
				"name":  "Up to 100 characters; names are unique whatever their case.",
				"color": "A hex color such as #3b82f6.",
				"icon":  "An icon name such as bug, question, lightbulb, rocket, refund-2 or star.",
			},
		},
		"tags.update": {
			Title: "Update a tag",
			Use: "Renames, recolors or changes the icon of a tag. Every conversation, contact and company " +
				"carrying it changes with it.",
			Returns: "the tag as it is now.",
			Params:  map[string]string{"color": "A hex color such as #3b82f6, or null to clear it."},
		},
		"tags.destroy": {
			Title:   "Delete a tag",
			Use:     "Deletes a tag and takes it off every conversation, contact and company in one step.",
			Returns: "nothing.",
		},
	}
}

/*
inboxSettings explains the settings of an inbox.
*/
func inboxSettings() string {
	return "A merge patch: keys sent replace theirs, null resets a key to its default. reopen_window_days: " +
		"how long after closing a customer's message reopens it, 0 for never. assign_on_reply: a reply takes " +
		"a conversation nobody holds. color. assignment: {mode: manual, round_robin or balanced; " +
		"include_away; overflow_team_id; queue_order: oldest_waiting or priority_then_oldest; " +
		"default_capacity; capacity_by_channel}."
}

/*
inboxSetupDocs returns the entries for listing and managing inboxes.
*/
func inboxSetupDocs() map[string]Doc {
	return map[string]Doc{
		"inboxes.index": {
			Title:   "List inboxes",
			Use:     "Lists the inboxes you can open. Conversations land in an inbox from the channels it connects.",
			Returns: "the inboxes with their access, default team and settings.",
			Next:    []string{"dimmi_conversations_list: list one inbox with filter[view]=inbox:inbox_..."},
		},
		"inboxes.show": {
			Title:   "Get an inbox",
			Use:     "Reads one inbox: its name, access, default team and assignment settings.",
			Returns: "the inbox.",
		},
		"inboxes.store": {
			Title:   "Create an inbox",
			Use:     "Adds an inbox, with its own API channel so conversations can be started in it right away.",
			Returns: "the new inbox.",
			Params:  map[string]string{"name": "Up to 80 characters."},
		},
		"inboxes.update": {
			Title:   "Update an inbox",
			Use:     "Changes an inbox's name, default team or settings, such as how it assigns new conversations.",
			Returns: "the inbox as it is now.",
			Params: map[string]string{
				"default_team_id": "The team (team_...) new conversations go to, or null for none.",
				"settings":        inboxSettings(),
			},
		},
		"inboxes.destroy": {
			Title: "Delete an inbox",
			Use: "Deletes an inbox, moving its conversations and channels to another inbox in the same step. " +
				"The default inbox cannot be deleted.",
			Returns: "nothing.",
			Params: map[string]string{
				"move_to": "The inbox (inbox_...) that takes its conversations and channels; needed when it has any.",
			},
		},
	}
}

/*
inboxAccessDocs returns the entries for who may open an inbox and how
it assigns.
*/
func inboxAccessDocs() map[string]Doc {
	return map[string]Doc{
		"inboxes.access.update": {
			Title: "Set who can open an inbox",
			Use: "Opens an inbox to everyone, or restricts it to the teams and teammates listed, replacing the " +
				"whole list. Owners and admins open every inbox whatever the list says.",
			Returns: "the inbox with its access.",
			Params: map[string]string{
				"mode":       "everyone or restricted.",
				"team_ids":   "With restricted: the teams (team_...) that may open it.",
				"member_ids": "With restricted: the teammates (mem_...) that may open it.",
			},
		},
		"inboxes.assignment-status.show": {
			Title: "Explain an inbox's automatic assignment",
			Use: "Explains why conversations wait for automatic assignment: who is in the inbox's pool, how " +
				"much each holds against their limits, why each may not take the next one, and the waiting " +
				"conversations by reason. Changes nothing.",
			Returns: "the pool member by member and the waiting conversations by reason.",
			Params:  map[string]string{"channel_type": "widget, email or api: the channel the pool is judged for."},
		},
	}
}

/*
inboxAIDocs returns the entries for what the AI agent and Operator did
on a conversation.
*/
func inboxAIDocs() map[string]Doc {
	conversation := "Id of the conversation, starting with conv_. A number does not work here; find the id " +
		"with dimmi_conversations_get."

	return map[string]Doc{
		"conversations.ai.runs.index": {
			Title: "List the AI agent's runs on a conversation",
			Use: "Lists each time the AI agent worked on a conversation: whether it answered, handed off to the " +
				"team or skipped, why, and what it cost. Use it to explain what the AI agent did.",
			Returns: "the runs with status, outcome, skip or handoff reason, answer message and token use.",
			Next:    []string{"dimmi_conversations_ai_runs_get: read one run's steps"},
			Params:  map[string]string{"conversation": conversation},
		},
		"conversations.ai.runs.show": {
			Title:   "Get an AI agent run",
			Use:     "Reads one AI agent run with its steps, the trace of what it did and why it ended as it did.",
			Returns: "the run and its steps; stored payloads show only when you may read them.",
			Params:  map[string]string{"conversation": conversation},
		},
		"conversations.operator.brief.store": {
			Title: "Brief me on a conversation",
			Use: "Asks Operator, the team's own agent, to sum up a conversation for you on your Operator thread " +
				"about it. With attach=true the brief is also kept on the conversation as a note for the team. " +
				"Each brief spends AI usage.",
			Returns: "the Operator thread with the brief as its newest message.",
			Next:    []string{"dimmi_operator_threads_messages_create: ask Operator a follow-up"},
			Params: map[string]string{
				"conversation": conversation,
				"thread_id":    "An Operator thread (opt_...) about this conversation; your latest one when left out.",
				"attach":       "true to also keep the brief on the conversation as a note.",
			},
		},
	}
}
