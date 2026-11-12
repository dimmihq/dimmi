/*
|--------------------------------------------------------------------------
| Ticket Fields
|--------------------------------------------------------------------------
|
| Conditions read the conversation's customer ticket: its type, state,
| state group, priority, field values, whether it waits to close the
| loop, and the status groups of the issues linked to it or to the work
| behind it. A conversation that is no ticket has none of these. The
| ticket triggers carry what changed.
|
*/

package workflowspec

import "github.com/dimmihq/dimmi/app/support/audience"

/*
ticketGroups are the state groups conditions compare.
*/
func ticketGroups() []string {
	return []string{"submitted", "in_progress", "waiting_customer", "waiting_team", "resolved"}
}

/*
issueGroups are the status groups of linked issues.
*/
func issueGroups() []string {
	return []string{"backlog", "unstarted", "started", "done", "canceled"}
}

/*
ticketFields are facts of the conversation's ticket.
*/
func ticketFields() []audience.Field {
	f := func(key string, t audience.FieldType, options ...string) audience.Field {
		return audience.Field{
			Key: key, Type: t, Where: audience.Conversation, Trust: audience.Observed, Options: options,
		}
	}

	return []audience.Field{
		f("ticket.type_id", audience.TypeID),
		f("ticket.state_id", audience.TypeID),
		f("ticket.state_group", audience.TypeList, ticketGroups()...),
		f("ticket.priority", audience.TypeList, "none", "low", "medium", "high", "urgent"),
		f("ticket.team_id", audience.TypeID),
		f("ticket.close_loop", audience.TypeBool),
		f("ticket.issue_status_groups", audience.TypeSet, issueGroups()...),
		{
			Key: "ticket.fields.", Prefix: true, Type: audience.TypeAuto, Where: audience.Conversation,
			Trust: audience.Observed,
		},
	}
}

/*
ticketEventFields are facts of the ticket change that started the run.
*/
func ticketEventFields() []audience.Field {
	on := func(key string, t audience.FieldType, options []string, triggers ...string) audience.Field {
		return audience.Field{
			Key: key, Type: t, Where: audience.Conversation, Trust: audience.Observed, Options: options,
			Triggers: triggers,
		}
	}

	return []audience.Field{
		on("trigger.from_group", audience.TypeList, ticketGroups(), TriggerTicketStateChanged),
		on("trigger.to_group", audience.TypeList, ticketGroups(), TriggerTicketStateChanged),
		on("trigger.fields", audience.TypeSet, nil, TriggerTicketFieldChanged),
		on("trigger.link_kind", audience.TypeList, []string{"back_office", "tracker", "follow_up", "related"},
			TriggerTicketLinked),
		on("trigger.issue_status_group", audience.TypeList, issueGroups(), TriggerTicketIssueUpdated),
	}
}
