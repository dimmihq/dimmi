/*
|--------------------------------------------------------------------------
| Tool Catalog: Linked Issues
|--------------------------------------------------------------------------
|
| The entries for the engineering work behind tickets: issues and
| incidents linked from Linear, Jira, GitHub, incident.io, Rootly and
| PagerDuty, the lists each connected tool offers, its search and sync
| settings, the drafts that close the loop, and the workspace's ticket
| switches. A recipe for filing a bug from a tracker: list the
| workspace's integrations, read the tool's teams, create the issue on
| the tracker, then watch its status come back on the ticket.
|
*/

package mcpserver

import "maps"

/*
ticketIssueDocs returns every linked issue entry.
*/
func ticketIssueDocs() map[string]Doc {
	out := ticketIssueLinkDocs()
	maps.Copy(out, ticketIssueToolDocs())

	return out
}

/*
ticketIssueLinkDocs returns the entries for links and drafts.
*/
func ticketIssueLinkDocs() map[string]Doc {
	return map[string]Doc{
		"tickets.issues.index": {
			Title: "List a ticket's linked issues",
			Use: "Lists the issues and engineering incidents linked to a ticket, with each one's status as " +
				"last seen.",
			Returns: "the links with provider, key, URL, status, status group, warning and sync error.",
		},
		"tickets.issues.store": {
			Title: "Create or link an issue",
			Use: "Creates an issue in a connected tool (installation_id, team_id, project_id, title) with the " +
				"customer impact and a link back, or links an existing one by url or key. With an incident tool " +
				"it declares the incident there (service_id for PagerDuty, severity_id).",
			Returns: "the link.",
			Next:    []string{"dimmi_integration_installations_issue_teams_list: pick the team first"},
			Params: map[string]string{
				"installation_id": "An installation id from dimmi_integrations_list.",
				"team_id":         "A team id from dimmi_integration_installations_issue_teams_list.",
				"url":             "A pasted issue URL; the tool is found from it.",
			},
		},
		"ticket-issues.destroy": {
			Title: "Unlink an issue", Use: "Removes the link; the issue stays in its tool.", Risk: new(RiskDanger),
		},
		"tickets.update-drafts.index": {
			Title: "List close-the-loop drafts",
			Use: "Lists the messages drafted for everyone a tracker links: when its issue is done or an " +
				"engineer commented @customers. Open drafts count who they reach " +
				"and who would be held, by reason.",
			Returns: "the drafts with status (waiting_deploy, ready, sent, discarded) and counts.",
		},
		"ticket-update-drafts.update": {
			Title:   "Send or discard a draft",
			Use:     "Sends a draft to everyone linked, optionally with an edited body, or discards it.",
			Returns: "the draft as decided.",
			Risk:    new(RiskReach),
		},
	}
}

/*
ticketIssueToolDocs returns the entries for connected tools and the
workspace's switches.
*/
func ticketIssueToolDocs() map[string]Doc {
	return map[string]Doc{
		"integration-installations.issue-teams.index": {
			Title:   "List a tool's teams",
			Use:     "Lists Linear teams, Jira projects or GitHub repositories of a connected tool.",
			Returns: "options with id and name.",
		},
		"integration-installations.issue-options.index": {
			Title: "List a tool's options",
			Use: "Lists one of a tool's lists: teams, projects (within a team), states, severities, services, " +
				"users, fields or environments (within a repository).",
			Returns: "options with id, name, status group and severity rank.",
		},
		"integration-installations.issues.index": {
			Title: "Search a tool's issues", Use: "Finds issues or incidents to link by words or key.",
			Returns: "issues with key, URL, title and status.",
		},
		"integration-installations.issue-settings.show": {
			Title:   "Get a tool's sync settings",
			Use:     "Reads how a tool's statuses map to ticket states, its defaults and where it delivers.",
			Returns: "the settings with webhook_url.",
		},
		"integration-installations.issue-settings.update": {
			Title: "Update a tool's sync settings",
			Use: "Maps a tool's status ids to ticket states and sets defaults, the deployment that proves a fix " +
				"shipped and the signing secret of a key-connected tool.",
			Returns: "the settings.",
			Risk:    new(RiskSensitive),
		},
		"integrations.key-installations.store": {
			Title:   "Connect a tool with an API key",
			Use:     "Connects incident.io, Rootly or PagerDuty with an API key, checked with the tool first.",
			Returns: "the installation.",
			Risk:    new(RiskSensitive),
		},
		"ticket-settings.show": {
			Title: "Get ticket switches", Use: "Reads whether duplicate conversations are linked to a tracker by " +
				"themselves and the spike threshold.", Returns: "the switches.",
		},
		"ticket-settings.update": {
			Title: "Update ticket switches", Use: "Changes the workspace's ticket switches.", Returns: "the switches.",
			Risk: new(RiskSensitive),
		},
	}
}
