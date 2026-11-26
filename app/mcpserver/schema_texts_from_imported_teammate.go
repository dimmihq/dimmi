package mcpserver

/*
schemaTextsFromImportedTeammate returns the texts of the fields about
teammates brought over from another tool, who wait for an admin to
enable them.
*/
func schemaTextsFromImportedTeammate() map[string]map[string]string {
	return map[string]map[string]string{
		"AwaitingAccess": {
			"name": "The name of the workspace the caller waits to be enabled in.",
		},
		"Me": {
			"awaiting_access": "The workspaces the caller was brought into from another tool and " +
				"waits for an admin to enable them in. Null for personal access tokens.",
		},
		"WorkspaceMember": {
			"awaiting_enable": "True when the member was brought over from another tool and waits " +
				"for an admin to enable them. Set status to active to enable them.",
			"enable_invited_at": "When enabling mailed the member an invitation, because they had " +
				"not signed up yet. Null before that.",
			"enable_team_ids": "Ids of the teams enabling puts the member in, each starting with " +
				"team_. Empty for anyone not waiting to be enabled.",
			"imported_from": "The tool the member was brought over from, such as intercom. Null for " +
				"anyone else.",
		},
	}
}
