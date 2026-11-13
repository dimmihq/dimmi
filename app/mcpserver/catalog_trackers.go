/*
|--------------------------------------------------------------------------
| Tool Catalog: Trackers
|--------------------------------------------------------------------------
|
| The entries for what the AI finds around trackers: the trackers
| suggested for a conversation, and a tracker's summary of what its
| customers report.
|
*/

package mcpserver

/*
trackerDocs returns every tracker entry.
*/
func trackerDocs() map[string]Doc {
	return map[string]Doc{
		"conversations.tracker-suggestions.index": {
			Title:   "List trackers suggested for a conversation",
			Use:     "Reads the open trackers the AI thinks the conversation reports, best first.",
			Returns: "trackers with reference, title and how many conversations each already reaches.",
			Next:    []string{"dimmi_tickets_links_create: link the conversation to the tracker"},
		},
		"tickets.summary.show": {
			Title: "Summarize a tracker",
			Use: "Reads what a tracker's customers report: a short summary the cheap model writes, the " +
				"number of reports and which plans the reporters' companies are on.",
			Returns: "summary, reports and plans with counts.",
		},
	}
}
