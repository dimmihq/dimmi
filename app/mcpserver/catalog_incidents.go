/*
|--------------------------------------------------------------------------
| Tool Catalog: Incidents
|--------------------------------------------------------------------------
|
| The entries for what the AI finds around incidents and trackers:
| spikes of conversations suggested as incidents, which an agent
| accepts (declaring the incident with the conversations linked) or
| dismisses for an hour; the trackers suggested for a conversation; and
| a tracker's summary of what its customers report.
|
*/

package mcpserver

/*
incidentDocs returns every incident entry.
*/
func incidentDocs() map[string]Doc {
	return map[string]Doc{
		"incident-suggestions.index": {
			Title: "List incident suggestions",
			Use: "Lists the open suggestions the spike detector made: groups of new conversations about one " +
				"thing, at or above the workspace's threshold in 15 minutes.",
			Returns: "suggestions with title, summary, area, conversation ids and count.",
			Next:    []string{"dimmi_incident_suggestions_update: accept to declare the incident, or dismiss"},
		},
		"incident-suggestions.update": {
			Title: "Accept or dismiss an incident suggestion",
			Use: "status=accepted declares an incident with the suggestion's title and area and links its " +
				"conversations; status=dismissed mutes the group for an hour.",
			Returns: "the suggestion; ticket_id names the declared incident.",
			Next:    []string{"dimmi_tickets_public_updates_create: post the first public update"},
			Params:  map[string]string{"status": "accepted or dismissed."},
		},
		"conversations.tracker-suggestions.index": {
			Title:   "List trackers suggested for a conversation",
			Use:     "Reads the open trackers the AI thinks the conversation reports, best first.",
			Returns: "trackers with reference, title and how many conversations each already reaches.",
			Next:    []string{"dimmi_tickets_links_create: link the conversation to the tracker"},
		},
		"tickets.summary.show": {
			Title: "Summarize a tracker",
			Use: "Reads what a tracker's or incident's customers report: a short summary the cheap model " +
				"writes, the number of reports and which plans the reporters' companies are on.",
			Returns: "summary, reports and plans with counts.",
		},
	}
}
