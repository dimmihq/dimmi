/*
|--------------------------------------------------------------------------
| Written Argument Texts: Tickets
|--------------------------------------------------------------------------
|
| The arguments of the tools for tickets, their types, fields and
| states, linked issues, SLA policies and the tools issues are tracked
| in.
|
*/

package mcpserver

/*
ticketTexts returns the texts written for these tools.
*/
func ticketTexts() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, part := range []map[string]map[string]string{
		ticketTextsPart1(), ticketTextsPart2(), ticketTextsPart3(), ticketTextsPart4(), ticketTextsPart5(),
		ticketTextsPart6(), ticketTextsPart7(), ticketTextsPart8(), ticketTextsPart9(),
	} {
		mergeTexts(out, part)
	}

	return out
}

/*
ticketTextsPart1 returns texts of convert_conversation_to_ticket,
create_integration_key_installation and the tools after them.
*/
func ticketTextsPart1() map[string]map[string]string {
	return map[string]map[string]string{
		"convert_conversation_to_ticket": {
			"ai_fill": "true to have the AI read the conversation and propose values for the type's fields. " +
				"They are kept as suggestions, or saved straight into empty fields where the type or field " +
				"fills itself. false or left out skips this.",
			"fields": "A JSON object of field values, keyed by the field keys of the ticket type in type_id. " +
				"Text is a string, numbers a number, boolean true or false, date \"YYYY-MM-DD\", single choice " +
				"one option, multiple choice a list of options, such as {\"order_number\": \"A-1042\", " +
				"\"refund\": true}.",
		},
		"create_integration_key_installation": {
			"integration_provider": "The name of the tool to connect with an API key: incident_io, rootly or " +
				"pagerduty, such as \"pagerduty\".",
		},
		"create_sla_policy": {
			"breach_priority": "The priority a ticket or conversation is raised to when it misses a target of " +
				"this policy: low, medium, high or urgent, such as \"urgent\". It is only raised, never " +
				"lowered. Send \"none\" for no priority change on a breach.",
			"pause_on_waiting_customer": "true to pause the clocks while a ticket is in a state waiting on the " +
				"customer, or a conversation is snoozed; false to keep them running. A new policy starts with " +
				"true.",
			"pause_on_waiting_team": "true to pause the clocks while a ticket is in a state waiting on another " +
				"team; false to keep them running. A new policy starts with false.",
			"priorities": "The ticket priorities this policy applies to, as a list of none, low, medium, high, " +
				"urgent, such as [\"high\", \"urgent\"]. An empty list means any priority. A policy with no " +
				"priorities and no ticket types is never applied on its own.",
		},
		"create_ticket": {
			"priority": "How urgent the ticket is: none, low, medium, high or urgent, such as \"high\". Left " +
				"out, the ticket starts at none.",
		},
	}
}

/*
ticketTextsPart2 returns texts of create_ticket_issue,
create_ticket_link and the tools after them.
*/
func ticketTextsPart2() map[string]map[string]string {
	return map[string]map[string]string{
		"create_ticket_issue": {
			"key": "To link an issue that already exists instead of creating one: its key or id in the tool, " +
				"such as \"ENG-123\" for Linear or Jira, \"acme/api#42\" for GitHub, \"INC-12\" for " +
				"incident.io. Use this or url; leave both out to create a new issue.",
			"project_id": "When creating an issue: the id from the tool's \"projects\" list for the picked " +
				"team, a Linear project or a Jira issue type, such as \"10001\". Left out, the default saved " +
				"in the tool's issue settings is used.",
			"service_id": "When creating an incident: the id of the service it is on, from the tool's " +
				"\"services\" list, such as \"PABC123\". PagerDuty needs one, here or saved in the tool's " +
				"issue settings.",
			"severity_id": "When creating an incident: the id of its severity, from the tool's \"severities\" " +
				"list. Left out, the default saved in the tool's issue settings is used.",
		},
		"create_ticket_link": {
			"kind": "What the link means: back_office (from a customer ticket to back-office work, needs " +
				"ticket_id), tracker (from a tracker ticket to a conversation, needs conversation_id), " +
				"follow_up or related (join any two tickets), such as \"related\".",
		},
		"create_ticket_state": {
			"customer_label": "What the customer sees as the status while a ticket is in this state, such as " +
				"\"With our billing team\". At most 80 characters. Left empty, the state name is used.",
			"group": "The stage this state belongs to: submitted, in_progress, waiting_customer, waiting_team " +
				"or resolved, such as \"in_progress\". It is set when the state is added and cannot be changed " +
				"later.",
			"notify_customer": "true to send the customer a message with the customer label when a ticket they " +
				"can see moves into this state; false to stay quiet. A new state starts with true.",
		},
	}
}

/*
ticketTextsPart3 returns texts of create_ticket_type,
create_ticket_type_field.
*/
func ticketTextsPart3() map[string]map[string]string {
	return map[string]map[string]string{
		"create_ticket_type": {
			"ai_auto_fill": "true to let the AI save its proposed values straight into every empty field of " +
				"new tickets of this type; false to keep them as suggestions a teammate accepts, except for " +
				"fields that have ai_auto on.",
			"auto_resolve_message": "The message sent in the ticket's conversation when a waiting ticket of " +
				"this type is resolved on its own after auto_resolve_days, such as \"We closed this as we did " +
				"not hear back.\" At most 2000 characters.",
			"company_sharing": "true to let verified customers also see tickets of this type opened by others " +
				"in their own company; false to show each customer only their own.",
			"customer_visible": "true to let customers open tickets of this type and see them; false to keep " +
				"the type for the team only. A new type of kind customer starts with true, other kinds with " +
				"false.",
			"kind": "What the type is for: customer (a customer's request tied to a conversation), back_office " +
				"(work for another team) or tracker (gathers many reports of one problem), such as " +
				"\"customer\". Set when the type is added; it cannot be changed later.",
			"move_tickets": "When state_ids drops a state that still holds tickets of this type, where those " +
				"tickets go: a list of objects with the dropped state and one of the kept states, such as " +
				"[{\"from_state_id\": \"tkts_...\", \"to_state_id\": \"tkts_...\"}].",
		},
		"create_ticket_type_field": {
			"ai_auto": "true to let the AI save its proposed value straight into this field on a new ticket " +
				"when the field is empty; false to keep it as a suggestion a teammate accepts.",
			"ai_hint": "A short note telling the AI how to fill this field, such as \"The order number from " +
				"the customer's message, like A-1042\". At most 500 characters.",
			"data_type": "What kind of value the field holds: short_text, long_text, number (whole), decimal, " +
				"currency, boolean, date, single_choice, multiple_choice, file, member or company, such as " +
				"\"single_choice\". Set when the field is added; it cannot be changed later.",
			"editable_customer": "true to let the customer fill or change this field on their own tickets; " +
				"false for team only. Turning it on also makes the field visible to the customer.",
		},
	}
}

/*
ticketTextsPart4 returns texts of create_ticket_type_field,
delete_ticket_issue and the tools after them.
*/
func ticketTextsPart4() map[string]map[string]string {
	return map[string]map[string]string{
		"create_ticket_type_field": {
			"key": "The short name field values are stored under: lowercase letters, digits and underscores, " +
				"starting with a letter, at most 40 characters, such as \"order_number\". Unique within the " +
				"type; set when the field is added and cannot be changed later.",
			"label": "The name of the field shown on forms, 1 to 80 characters, such as \"Order number\".",
			"options": "The choices of a single_choice or multiple_choice field, as a list of texts of 1 to " +
				"100 characters, at most 200, such as [\"Refund\", \"Exchange\", \"Other\"]. Choice fields " +
				"need at least one. Sending it replaces the whole list.",
			"required_create": "true if the team, the API and automation must fill this field to create a " +
				"ticket; false if it may be left empty.",
			"required_create_customer": "true if a customer must fill this field when they open a ticket " +
				"themselves; false if they may leave it empty.",
			"required_resolve": "true if this field must have a value before a ticket can move to a resolved " +
				"state; false if not.",
			"show_when": "Show this field only when another field of the same type has a value: a JSON object " +
				"with that field's key and, optionally, the values that count, such as {\"field\": \"reason\", " +
				"\"equals\": [\"Refund\"]}. Without equals any value shows it.",
			"visible_customer": "true to show this field and its value to the customer on their tickets; false " +
				"to keep it for the team. It is always true while editable_customer is true.",
		},
		"delete_ticket_issue": {
			"remove_from_tool": "true to also take back what dimmi wrote on the issue in its tool, such as the " +
				"customer impact and the link back to the ticket; false or left out leaves them there. The " +
				"issue itself stays in the tool either way.",
		},
		"delete_ticket_state": {
			"move_to": "Required. The id of another live state of the same group that this state's tickets, " +
				"and the types using it, move to, such as \"tkts_01hzy3v8k2m4n6p8q0r2s4t6v8\".",
		},
	}
}

/*
ticketTextsPart5 returns texts of escalate_ticket,
list_integration_installation_issue_options and the tools after them.
*/
func ticketTextsPart5() map[string]map[string]string {
	return map[string]map[string]string{
		"escalate_ticket": {
			"fields": "A JSON object of field values, keyed by the field keys of the back-office ticket type " +
				"in type_id. Text is a string, numbers a number, boolean true or false, date \"YYYY-MM-DD\", " +
				"single choice one option, multiple choice a list of options, such as {\"order_number\": " +
				"\"A-1042\", \"refund\": true}.",
		},
		"list_integration_installation_issue_options": {
			"list": "Which list to pull from the tool: teams, projects, states, severities, services, users, " +
				"fields or environments, such as \"states\". Left out, teams. Tools offer only some: Linear " +
				"and Jira have teams, projects and states; GitHub has teams (repositories), environments and " +
				"states.",
			"parent": "The id the list sits within, taken from an earlier list: a team id for the projects and " +
				"states of Linear or Jira, or a repository like \"acme/api\" for GitHub environments. Leave " +
				"out for lists with no parent, such as teams.",
		},
		"list_tickets": {
			"created_after": "Only tickets created at or after this time, as an RFC 3339 date and time with a " +
				"time zone, such as \"2026-01-15T00:00:00Z\".",
			"created_before": "Only tickets created before this time, as an RFC 3339 date and time with a time " +
				"zone, such as \"2026-02-01T00:00:00Z\".",
			"fields": "A JSON object, written as text, of field values tickets must match exactly, keyed by " +
				"field key, such as {\"plan\": \"pro\", \"refund\": true}.",
			"issue_status": "Only tickets with a linked issue whose status is in this group: backlog, " +
				"unstarted, started, done or canceled, such as \"started\".",
			"priority": "Only tickets of this priority: none, low, medium, high or urgent, such as \"urgent\".",
			"resolved_after": "Only tickets resolved at or after this time, as an RFC 3339 date and time with " +
				"a time zone, such as \"2026-01-15T00:00:00Z\".",
			"resolved_before": "Only tickets resolved before this time, as an RFC 3339 date and time with a " +
				"time zone, such as \"2026-02-01T00:00:00Z\".",
		},
		"update_integration_installation_issue_settings": {
			"deploy_environment": "The GitHub environment whose successful deployments count as a fix shipped, " +
				"by name from the \"environments\" list of the repository, such as \"production\". Empty means " +
				"any environment of the deploy repository.",
		},
	}
}

/*
ticketTextsPart6 returns texts of
update_integration_installation_issue_settings, update_sla_policy and
the tools after them.
*/
func ticketTextsPart6() map[string]map[string]string {
	return map[string]map[string]string{
		"update_integration_installation_issue_settings": {
			"deploy_repository": "The GitHub repository whose successful deployments mark done issues as " +
				"shipped, as owner/name from the tool's \"teams\" list, such as \"acme/api\". Empty turns this " +
				"off.",
			"project_id": "The default for new issues when none is picked: an id from the tool's \"projects\" " +
				"list for the team, a Linear project or a Jira issue type, such as \"10001\". Empty clears it.",
			"service_id": "The default service new incidents are raised on when none is picked: an id from the " +
				"tool's \"services\" list, such as \"PABC123\". Empty clears it.",
			"severity_id": "The default severity new incidents are declared with when none is picked: an id " +
				"from the tool's \"severities\" list. Empty clears it.",
			"states": "A JSON object mapping the tool's status ids, from its \"states\" list, to the dimmi " +
				"ticket state a ticket moves to, such as {\"started\": \"tkts_01hzy3v8k2m4n6p8q0r2s4t6v8\"}. " +
				"It replaces the whole mapping; an empty value drops that status.",
		},
		"update_sla_policy": {
			"breach_priority": "The priority a ticket or conversation is raised to when it misses a target of " +
				"this policy: low, medium, high or urgent, such as \"urgent\". It is only raised, never " +
				"lowered. Send \"none\" for no priority change on a breach.",
			"pause_on_waiting_customer": "true to pause the clocks while a ticket is in a state waiting on the " +
				"customer, or a conversation is snoozed; false to keep them running.",
			"pause_on_waiting_team": "true to pause the clocks while a ticket is in a state waiting on another " +
				"team; false to keep them running.",
			"priorities": "The ticket priorities this policy applies to, as a list of none, low, medium, high, " +
				"urgent, such as [\"high\", \"urgent\"]. An empty list means any priority. A policy with no " +
				"priorities and no ticket types is never applied on its own. Sending it replaces the list.",
		},
		"update_ticket": {
			"accept_suggestions": "The keys of fields whose AI-suggested value should be saved as the field's " +
				"value, as a list such as [\"order_number\", \"reason\"]. Keys with no suggestion are skipped.",
		},
	}
}

/*
ticketTextsPart7 returns texts of update_ticket, update_ticket_field.
*/
func ticketTextsPart7() map[string]map[string]string {
	return map[string]map[string]string{
		"update_ticket": {
			"fields": "A JSON object of field values to change, keyed by the field keys of the ticket's type; " +
				"other fields stay. A null removes a value. Dates are \"YYYY-MM-DD\", choices must be among " +
				"the field's options, such as {\"order_number\": \"A-1042\", \"reason\": null}.",
			"priority": "How urgent the ticket is: none, low, medium, high or urgent, such as \"high\".",
			"version": "The version number of the ticket you last read, such as 7. When sent, the change is " +
				"refused if the ticket has changed since. Leave out to change it regardless.",
		},
		"update_ticket_field": {
			"ai_auto": "true to let the AI save its proposed value straight into this field on a new ticket " +
				"when the field is empty; false to keep it as a suggestion a teammate accepts.",
			"ai_hint": "A short note telling the AI how to fill this field, such as \"The order number from " +
				"the customer's message, like A-1042\". At most 500 characters. Send null to clear it.",
			"data_type": "The field's data type, such as \"short_text\". It is set when the field is added and " +
				"never changes; a value sent here is ignored.",
			"editable_customer": "true to let the customer fill or change this field on their own tickets; " +
				"false for team only. Turning it on also makes the field visible to the customer.",
			"key": "The field's key, such as \"order_number\". It is set when the field is added and never " +
				"changes; a value sent here is ignored.",
			"label": "The name of the field shown on forms, 1 to 80 characters, such as \"Order number\".",
			"options": "The choices of a single_choice or multiple_choice field, as a list of texts of 1 to " +
				"100 characters, at most 200, such as [\"Refund\", \"Exchange\", \"Other\"]. Choice fields " +
				"need at least one. Sending it replaces the whole list.",
			"required_create": "true if the team, the API and automation must fill this field to create a " +
				"ticket; false if it may be left empty.",
			"required_create_customer": "true if a customer must fill this field when they open a ticket " +
				"themselves; false if they may leave it empty.",
		},
	}
}

/*
ticketTextsPart8 returns texts of update_ticket_field,
update_ticket_settings and the tools after them.
*/
func ticketTextsPart8() map[string]map[string]string {
	return map[string]map[string]string{
		"update_ticket_field": {
			"required_resolve": "true if this field must have a value before a ticket can move to a resolved " +
				"state; false if not.",
			"show_when": "Show this field only when another field of the same type has a value: a JSON object " +
				"with that field's key and, optionally, the values that count, such as {\"field\": \"reason\", " +
				"\"equals\": [\"Refund\"]}. Without equals any value shows it. Send null to always show it.",
			"visible_customer": "true to show this field and its value to the customer on their tickets; false " +
				"to keep it for the team. It is always true while editable_customer is true.",
		},
		"update_ticket_settings": {
			"auto_link_duplicates": "true to link a new conversation to the tracker ticket it best matches " +
				"without asking; false to only suggest the match for a teammate to accept.",
		},
		"update_ticket_state": {
			"customer_label": "What the customer sees as the status while a ticket is in this state, such as " +
				"\"With our billing team\". At most 80 characters. Left empty, the state name is used.",
			"group": "The stage the state belongs to, such as \"in_progress\". It is set when the state is " +
				"added and never changes; a value sent here is ignored.",
			"notify_customer": "true to send the customer a message with the customer label when a ticket they " +
				"can see moves into this state; false to stay quiet. A new state starts with true.",
		},
		"update_ticket_type": {
			"ai_auto_fill": "true to let the AI save its proposed values straight into every empty field of " +
				"new tickets of this type; false to keep them as suggestions a teammate accepts, except for " +
				"fields that have ai_auto on.",
			"auto_resolve_message": "The message sent in the ticket's conversation when a waiting ticket of " +
				"this type is resolved on its own after auto_resolve_days, such as \"We closed this as we did " +
				"not hear back.\" At most 2000 characters. Send null to send no message.",
			"company_sharing": "true to let verified customers also see tickets of this type opened by others " +
				"in their own company; false to show each customer only their own.",
		},
	}
}

/*
ticketTextsPart9 returns texts of update_ticket_type.
*/
func ticketTextsPart9() map[string]map[string]string {
	return map[string]map[string]string{
		"update_ticket_type": {
			"customer_visible": "true to let customers open tickets of this type and see them; false to keep " +
				"the type for the team only.",
			"kind": "The type's kind: customer, back_office or tracker. It is set when the type is added and " +
				"never changes; a value sent here is ignored.",
			"move_tickets": "When state_ids drops a state that still holds tickets of this type, where those " +
				"tickets go: a list of objects with the dropped state and one of the kept states, such as " +
				"[{\"from_state_id\": \"tkts_...\", \"to_state_id\": \"tkts_...\"}].",
		},
	}
}
