/*
|--------------------------------------------------------------------------
| Tool Catalog: Ticket Reports
|--------------------------------------------------------------------------
|
| The entries for the ticket reports: time spent in each state per
| type, tickets per type and per value of a field, service level
| attainment per policy and target, and the trackers ranked by the
| customers, companies and revenue they reach. Each reads from and to,
| whole days in the workspace's time zone, the last thirty by default.
|
*/

package mcpserver

/*
ticketReportRange describes the range every ticket report reads.
*/
func ticketReportRange() map[string]string {
	return map[string]string{
		"from": "First day, 2006-01-02. Defaults to 30 days ago.",
		"to":   "Last day included, 2006-01-02. Defaults to today.",
	}
}

/*
ticketReportDocs returns every ticket report entry.
*/
func ticketReportDocs() map[string]Doc {
	return map[string]Doc{
		"reports.tickets.time-in-state.index": {
			Title: "Report time in state",
			Use: "Shows how long tickets created in the range spent in each state, per type, to find where work " +
				"waits. Time resolved is left out; a state still held counts up to now.",
			Returns: "per type and state: tickets, mean, median and 90th percentile seconds.",
			Params:  ticketReportRange(),
		},
		"reports.tickets.by-type.index": {
			Title:   "Report tickets by type",
			Use:     "Counts each type's tickets created and resolved in the range, and open now.",
			Returns: "per type: created, resolved and open.",
			Next:    []string{"dimmi_reports_tickets_by_field_list: split one type by a field"},
			Params:  ticketReportRange(),
		},
		"reports.tickets.by-field.index": {
			Title: "Report tickets by field",
			Use: "Splits a type's tickets created in the range by the values of one field, such as refund " +
				"reasons or bug severity. Each choice of a multiple choice field counts once.",
			Returns: "the field and, per value (null for none), tickets and how many are resolved.",
			Params: map[string]string{
				"type_id": "A type id from dimmi_ticket_types_list.",
				"field":   "The key of one of the type's fields.",
				"from":    "First day, 2006-01-02.",
				"to":      "Last day included, 2006-01-02.",
			},
		},
		"reports.tickets.sla.index": {
			Title: "Report service level attainment",
			Use: "Shows, per SLA policy and target, the clocks that ended in the range met or breached and the " +
				"share met.",
			Returns: "per policy and target: met, breached, hit rate in percent and mean seconds to meet.",
			Params:  ticketReportRange(),
		},
		"reports.tickets.trackers.index": {
			Title: "Report tracker impact",
			Use: "Ranks the trackers and incidents open in the range, or resolved in it, by the monthly revenue " +
				"of the companies whose conversations they gather, then by conversations.",
			Returns: "trackers with conversations, those linked in the range, customers, companies and revenue.",
			Next:    []string{"dimmi_tickets_get: read the tracker", "dimmi_tickets_updates_create: message them"},
			Params:  ticketReportRange(),
		},
	}
}
