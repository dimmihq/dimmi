/*
|--------------------------------------------------------------------------
| Written Schema Texts: TicketField To TimelineEntry
|--------------------------------------------------------------------------
|
| What each field of these records means, by schema and field.
|
*/

package mcpserver

/*
schemaTexts134 returns the texts from TicketField on.
*/
func schemaTexts134() map[string]map[string]string {
	return map[string]map[string]string{
		"TicketField": {
			"ai_auto": "True when the AI may fill this field in by itself.",
			"ai_hint": "Guidance telling the AI how to fill this field, up to 500 characters. Null " +
				"when none is set.",
			"archived_at": "When the field was archived: hidden from new work but kept on old tickets. " +
				"Null while it is in use.",
			"currency": "Three-letter uppercase currency code for a currency field, such as USD. Null " +
				"when none is set.",
			"data_type": "The type of value the field holds: short_text, long_text, number, decimal, " +
				"currency, boolean, date, single_choice, multiple_choice, file, member or company.",
			"description": "Help text explaining the field, up to 300 characters. Empty when none is " +
				"set.",
			"editable_customer": "True when the customer may change this field's value.",
			"key": "The field's key within its type, lowercase letters, digits and underscores, such " +
				"as order_number. Use it as the key in a ticket's fields object.",
			"label": "The field's name as people see it, 1 to 80 characters.",
			"options": "The choices of a single_choice or multiple_choice field, as a list of strings. " +
				"Empty for fields without choices.",
			"position":        "The field's place in its type's field order, lower first.",
			"required_create": "True when a teammate must give this field a value to create a ticket.",
			"required_create_customer": "True when a customer must give this field a value to create a " +
				"ticket.",
			"required_resolve": "True when the field must have a value before the ticket can be " +
				"resolved.",
			"type_id": "Id of the ticket type the field belongs to, such as tktt_.... Send it wherever " +
				"a ticket type is asked for.",
			"visible_customer": "True when the customer sees this field on their ticket.",
		},
		"TicketFieldCount": {
			"resolved": "How many of the tickets holding this value are resolved.",
			"tickets":  "How many tickets hold this value.",
			"value": "The field value being counted, as text. Null for the tickets that have no value " +
				"for the field.",
		},
	}
}

/*
schemaTexts135 returns the texts from TicketFieldReport on.
*/
func schemaTexts135() map[string]map[string]string {
	return map[string]map[string]string{
		"TicketFieldReport": {
			"data_type": "The reported field's type: short_text, long_text, number, decimal, currency, " +
				"boolean, date, single_choice, multiple_choice, file, member or company.",
			"field":   "Key of the reported field, such as order_number.",
			"label":   "The reported field's name as people see it.",
			"type_id": "Id of the ticket type the field belongs to, such as tktt_....",
			"values": "One row per value the field holds, each with how many tickets hold it and how " +
				"many of those are resolved.",
		},
		"TicketImpact": {
			"companies":     "How many companies the tracker reaches through its linked conversations.",
			"conversations": "How many conversations are linked to the tracker.",
			"customers":     "How many customers the tracker reaches through its linked conversations.",
			"revenue": "Total monthly revenue of the companies reached, read from each company's " +
				"monthly_spend or mrr attribute; a company with neither counts as 0.",
		},
		"TicketIssue": {
			"deployed_at": "When the issue's fix was seen deployed. Null until then.",
			"done_at":     "When the issue was seen done in its tool. Null until then.",
			"external_id": "The issue's or incident's own id in the provider's tool.",
			"installation_id": "Id of the connected integration the issue is read through, such as " +
				"itg_.... Null when that integration is gone or none is used.",
			"key": "The issue's short key as its tool shows it, such as ENG-123.",
			"provider": "The tool the issue or incident lives in: linear, jira, github, incident_io, " +
				"rootly or pagerduty.",
			"severity": "The issue's or incident's severity as the provider names it. Empty when none " +
				"is known.",
			"status": "The issue's status as its tool names it, as last seen, such as In Progress. " +
				"Empty when not yet read.",
			"status_group": "What dimmi maps the issue's status to: backlog, unstarted, started, done " +
				"or canceled.",
			"sync_error": "Why the last check of the issue failed. Null when it worked.",
			"synced_at":  "When the issue was last checked against its tool. Null when never checked.",
			"team_id":    "The provider's own id of the team the issue belongs to. Empty when not known.",
		},
	}
}

/*
schemaTexts136 returns the texts from TicketIssue on.
*/
func schemaTexts136() map[string]map[string]string {
	return map[string]map[string]string{
		"TicketIssue": {
			"team_name": "Name of the team the issue belongs to in its tool. Empty when not known.",
			"ticket_id": "Id of the ticket the issue is linked to, such as tkt_....",
			"title":     "The issue's title as last seen in its tool. Empty when not yet read.",
			"url":       "Web address of the issue in its tool.",
			"warning": "A problem a person should know about, such as a Linear team the app cannot " +
				"see. Null when there is none.",
		},
		"TicketLink": {
			"conversation_id": "Id of the conversation the link points to, such as conv_.... Null when " +
				"the link points to another ticket instead.",
			"from_ticket_id": "Id of the ticket the link starts from, such as tkt_....",
			"kind": "What the link means: back_office (work behind a customer ticket), tracker (a " +
				"report of a tracked problem), follow_up or related.",
			"to_ticket_id": "Id of the ticket the link points to, such as tkt_.... Null when the link " +
				"points to a conversation instead.",
		},
		"TicketPlanCount": {
			"count": "How many of the tracker's reports came from customers on this plan.",
			"plan":  "The plan's name. Empty for customers without a plan.",
		},
		"TicketSLAAttainment": {
			"breached": "How many clocks of this target ended in the reported range after their " +
				"deadline.",
			"hit_rate": "Percent of ended clocks that met the target, from 0 to 100. Null when none " +
				"ended in the range.",
			"mean_met_seconds": "Average time the met clocks took, in seconds.",
			"met":              "How many clocks of this target ended in the reported range within their deadline.",
			"policy_id":        "Id of the service level policy, such as sla_....",
			"policy_name":      "Name of the service level policy.",
			"running":          "How many clocks of this target are running right now.",
			"target":           "Which promise of the policy this row is about.",
		},
	}
}

/*
schemaTexts137 returns the texts from TicketSettings on.
*/
func schemaTexts137() map[string]map[string]string {
	return map[string]map[string]string{
		"TicketSettings": {
			"auto_link_duplicates": "True when dimmi links conversations that report an already " +
				"tracked problem to its tracker by itself.",
		},
		"TicketState": {
			"archived_at": "When the state was archived: hidden from new work but kept on old tickets. " +
				"Null while it is in use.",
			"customer_label": "What the customer sees when their ticket is in this state, such as " +
				"We're on it.",
			"group": "The fixed group that says what the state means: submitted, in_progress, " +
				"waiting_customer, waiting_team or resolved.",
			"name":            "The state's name as the team sees it, 1 to 60 characters.",
			"notify_customer": "True when moving a ticket into this state tells the customer.",
			"position":        "The state's place in the workspace's order of states, lower first.",
			"purpose": "Marks a state dimmi itself moves tickets to: close_loop, being_worked_on, " +
				"wont_fix or investigating. Null for ordinary states.",
		},
		"TicketStateRef": {
			"customer_label": "What the customer sees when their ticket is in this state.",
			"group": "The fixed group that says what the state means: submitted, in_progress, " +
				"waiting_customer, waiting_team or resolved.",
			"name": "The state's name as the team sees it.",
		},
		"TicketStateTime": {
			"group": "The state's fixed group: submitted, in_progress, waiting_customer, waiting_team " +
				"or resolved.",
			"mean_seconds":   "Average time tickets spent in this state, in seconds.",
			"median_seconds": "Median time tickets spent in this state, in seconds.",
			"p90_seconds":    "The time 90% of tickets stayed within in this state, in seconds.",
			"state_id":       "Id of the ticket state, such as tkts_....",
			"state_name":     "The state's name as the team sees it.",
			"tickets":        "How many tickets of this type were in this state.",
			"type_id":        "Id of the ticket type, such as tktt_....",
			"type_name":      "The ticket type's name.",
		},
	}
}

/*
schemaTexts138 returns the texts from TicketSummary on.
*/
func schemaTexts138() map[string]map[string]string {
	return map[string]map[string]string{
		"TicketSummary": {
			"generated_at": "When the summary was written. Null when none has been written yet.",
			"plans": "How the tracker's reports split by the reporting customers' plan, one row per " +
				"plan.",
			"reports": "How many conversations report the problem this tracker follows.",
			"summary": "AI-written summary of what the tracker's customers report, up to 4000 " +
				"characters. Empty when none has been written yet.",
			"ticket_id": "Id of the tracker ticket being summarized, such as tkt_....",
		},
		"TicketTrackerImpact": {
			"companies":     "How many companies the tracker reaches.",
			"conversations": "How many conversations are linked to the tracker in all.",
			"conversations_in_range": "How many conversations were linked to the tracker within the " +
				"reported date range.",
			"customers": "How many customers the tracker reaches.",
			"kind":      "Always tracker.",
			"reference": "The tracker's short reference, such as T-42.",
			"revenue": "Total monthly revenue of the companies reached, read from each company's " +
				"monthly_spend or mrr attribute.",
			"state_group": "The group of the tracker's current state: submitted, in_progress, " +
				"waiting_customer, waiting_team or resolved.",
			"title": "The tracker's title.",
		},
		"TicketType": {
			"ai_auto_fill": "True when the AI fills in fields on this type's tickets.",
			"archived_at": "When the type was archived: no new tickets can use it, old ones keep it. " +
				"Null while it is in use.",
			"auto_resolve_days": "Days a ticket waits on the customer before it resolves itself, 1 to " +
				"90. Null when tickets never resolve themselves.",
			"auto_resolve_message": "Message sent to the customer in the ticket's conversation when " +
				"the ticket resolves itself, up to 2000 characters. Null when none is sent.",
			"company_sharing": "True when everyone at the requester's company sees tickets of this " +
				"type.",
			"customer_visible": "True when the messenger and customer portal show this type.",
			"default_resolved_state_id": "Id of the resolved state tickets of this type move to by " +
				"default, such as tkts_.... Null when none is set.",
		},
	}
}

/*
schemaTexts139 returns the texts from TicketType on.
*/
func schemaTexts139() map[string]map[string]string {
	return map[string]map[string]string{
		"TicketType": {
			"default_state_id": "Id of the state new tickets of this type start in, such as tkts_.... " +
				"Null when none is set.",
			"description": "What this type is for, up to 500 characters. Empty when none is set.",
			"fields":      "The type's fields, each with its key, data type and when it is required.",
			"icon":        "Name of the type's icon, lowercase with dashes, such as ticket.",
			"inbox_id": "Id of the inbox tickets of this type go to by default, such as inbox_.... " +
				"Null when none is set.",
			"kind": "What tickets of this type are: customer (a customer's request), back_office (work " +
				"for another team) or tracker (many reports of one problem).",
			"name":         "The type's name, 1 to 60 characters.",
			"open_tickets": "How many tickets of this type are open now.",
			"position":     "The type's place in the workspace's order of types, lower first.",
			"reopen_days":  "Days a resolved ticket can still be reopened, 0 to 90.",
			"state_ids": "Ids of the states this type may use, in its order, such as tkts_.... A " +
				"ticket of this type can only move to one of these.",
			"team_id": "Id of the team tickets of this type go to by default, such as team_.... Null " +
				"when none is set.",
			"tickets_by_state": "Count of this type's tickets in each state, trashed ones included, " +
				"keyed by state id, such as {\"tkts_...\": 12}.",
			"total_tickets": "How many tickets of this type exist in all.",
		},
		"TicketTypeCount": {
			"created":  "How many tickets of this type were created in the reported range.",
			"kind":     "What tickets of this type are: customer, back_office or tracker.",
			"name":     "The ticket type's name.",
			"open":     "How many tickets of this type are open now.",
			"resolved": "How many tickets of this type were resolved in the reported range.",
			"type_id":  "Id of the ticket type, such as tktt_....",
		},
		"TicketTypeRef": {
			"icon": "Name of the type's icon, lowercase with dashes, such as ticket.",
			"kind": "What the ticket is: customer, back_office or tracker.",
			"name": "The ticket type's name.",
		},
	}
}

/*
schemaTexts140 returns the texts from TimelineEntry on.
*/
func schemaTexts140() map[string]map[string]string {
	return map[string]map[string]string{
		"TimelineEntry": {
			"attachments": "Files sent with the entry. Empty when there are none or the entry was " +
				"deleted.",
			"body": "The entry's text, read as body_format says. Null once the entry is deleted.",
			"body_format": "How to read body: markdown, or plain for text a customer typed in a mail " +
				"client, which is never markup.",
			"conversation_id": "Id of the conversation the entry belongs to, such as conv_....",
			"deleted_at": "When the entry was deleted; it keeps its place with its text gone. Null " +
				"when not deleted.",
			"delivery_error": "Why sending the message to the customer failed. Null when it did not " +
				"fail.",
			"delivery_status": "How far a message to the customer got: pending, sent, delivered, read " +
				"or failed; not_applicable for entries that are not delivered anywhere.",
			"edited_at": "When the entry's text was last edited. Null when never edited.",
			"event": "For an entry of type event, an object saying what happened, such as an " +
				"assignment. Null for messages and notes.",
			"has_original": "True when an email's whole text, quoted thread included, is kept and can " +
				"be asked for beyond what body shows.",
			"mentions": "The teammates and teams a note names, with their names. Empty for anything " +
				"but a note.",
			"reactions": "A note's emoji reactions, each with who reacted. Empty for anything but a " +
				"note.",
			"read_at": "When the customer saw a teammate's or AI agent's reply. Null when not seen or " +
				"when the channel does not report it; email never does.",
			"redactions": "The redaction rules that changed the text before it was stored, each with " +
				"its rule and how many matches it replaced. Left out when none did.",
			"scheduled_send_at": "When a reply still waiting to leave will be sent; its writer may " +
				"undo it until then. Null otherwise.",
			"sent_at": "When the message was sent. Null when it has not been sent.",
			"source_deleted_at": "When the author deleted the message where they wrote it, such as in " +
				"Slack; its text is kept. Null when they did not.",
			"substantive": "True for a real message exchanged with the customer; false for notes, " +
				"events and automatic messages.",
			"truncated": "True when an email's body was cut at the size limit.",
		},
	}
}
