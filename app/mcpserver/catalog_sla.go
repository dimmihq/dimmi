/*
|--------------------------------------------------------------------------
| Tool Catalog: Service Levels
|--------------------------------------------------------------------------
|
| The entries for service level policies and the clocks they run. A
| policy names targets in seconds and counts them in a schedule's open
| hours; it applies to new tickets of the types or priorities it names,
| or to a conversation through a workflow's apply SLA step. Changing a
| policy changes how the team is measured, so changes are settings.
|
*/

package mcpserver

/*
slaDocs returns every service level entry.
*/
func slaDocs() map[string]Doc {
	setting := func(title, use string) Doc {
		return Doc{Title: title, Use: use, Risk: new(RiskDanger)}
	}

	return map[string]Doc{
		"sla-policies.index": {
			Title:   "List SLA policies",
			Use:     "Lists the service level policies, archived ones last, with their targets in seconds.",
			Returns: "policies with first reply, next reply, follow-up and resolution targets and breach actions.",
			Next:    []string{"dimmi_tickets_sla_clocks_list: see how a ticket is doing against its policy"},
		},
		"sla-policies.show": {Title: "Get an SLA policy", Use: "Reads one policy."},
		"sla-policies.store": {
			Title: "Create an SLA policy",
			Use: "Adds a policy. Set at least one target in seconds (60 or more). schedule_id counts only open " +
				"hours; ticket_type_ids and priorities make new matching tickets follow it; breach_priority and " +
				"breach_team_id escalate on a breach.",
			Risk: new(RiskDanger),
			Next: []string{"dimmi_workflows_create: apply the policy to conversations with an apply_sla step"},
		},
		"sla-policies.update": setting("Update an SLA policy",
			"Changes a policy; a target of 0 removes it. Running clocks keep their due times."),
		"sla-policies.destroy": setting("Archive an SLA policy",
			"Archives a policy and cancels its running clocks; ended clocks stay for reports."),
		"tickets.sla-clocks.index": {
			Title:   "List a ticket's SLA clocks",
			Use:     "Lists every clock of a ticket, newest first: target, state, started, due, paused and breached.",
			Returns: "clocks in states running, paused, met, breached or canceled.",
		},
		"conversations.sla-clocks.index": {
			Title:   "List a conversation's SLA clocks",
			Use:     "Lists every clock of a conversation and of its customer ticket, newest first.",
			Returns: "clocks in states running, paused, met, breached or canceled.",
		},
	}
}
