/*
|--------------------------------------------------------------------------
| Tool Catalog: Who And Where
|--------------------------------------------------------------------------
|
| The entries that tell an agent who it acts for and in which workspace:
| the caller, their account, the workspace and its API limits, and the
| workspace's deletion.
|
*/

package mcpserver

/*
callerDocs returns the entries for the caller, their account and the
workspace itself.
*/
func callerDocs() map[string]Doc {
	return map[string]Doc{
		"me.show": {
			Title: "Who am I",
			Use: "Says who this connection acts for: the person, their membership and role in this workspace, and " +
				"what they are allowed to do. Call it early to know which tools will work before trying them.",
			Returns: "the person, the workspace, their role and their permissions.",
			Next:    []string{"dimmi_workspace_get: the workspace's name, plan and settings"},
		},
		"user.show": {
			Title:   "Get my account",
			Use:     "Reads the account of the person this connection acts for: name, email, language and time zone.",
			Returns: "the person's account.",
		},
		"workspace.show": {
			Title: "Get the workspace",
			Use: "Reads the workspace this connection works in: its name, address, time zone and main settings. " +
				"Every tool acts inside this one workspace.",
			Returns: "the workspace.",
			Next: []string{
				"dimmi_inboxes_list: the inboxes conversations arrive in", "dimmi_members_list: the team",
			},
		},
		"workspace.entitlements.show": {
			Title:   "Get the API limits",
			Use:     "Reads how many API requests a minute this workspace may make; zero means no limit.",
			Returns: "the workspace's request limits.",
			Hint:    "When calls are refused for going too fast, wait as the error says.",
		},
		"workspace.destroy": {
			Title: "Delete the workspace",
			Use: "Moves the whole workspace to the trash: its conversations, contacts, help center and settings " +
				"stop working for everyone, and it is purged for good after its time in the trash. Only the owner " +
				"may do this. Never call it unless the person explicitly asks to delete the workspace.",
			Returns: "the workspace in the trash, with when it will be purged.",
			Risk:    new(RiskDanger),
		},
	}
}
