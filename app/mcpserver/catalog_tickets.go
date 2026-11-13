/*
|--------------------------------------------------------------------------
| Tool Catalog: Tickets
|--------------------------------------------------------------------------
|
| The entries for tickets: finding and reading them, turning a
| conversation into one, creating back-office work and trackers, moving
| state and setting fields with the same required field rules the inbox
| obeys, notes and links, and messaging everyone a tracker reaches. A
| recipe for an agent triaging a bug report: read the conversation,
| list trackers with q, link it to the right one or create one, then
| file the issue.
|
*/

package mcpserver

import "maps"

/*
ticketDocs returns every ticket entry.
*/
func ticketDocs() map[string]Doc {
	out := ticketWorkDocs()
	maps.Copy(out, ticketChangeDocs())
	maps.Copy(out, ticketActivityDocs())
	maps.Copy(out, ticketSetupDocs())

	return out
}

/*
ticketWorkDocs returns the entries for tickets themselves.
*/
func ticketWorkDocs() map[string]Doc {
	return map[string]Doc{
		"tickets.index": {
			Title: "List tickets",
			Use: "Finds tickets by type, kind (customer, back_office, tracker), state, group, " +
				"teammate, team, company, customer, reference, field values, linked issue status, dates or text. " +
				"Use kind=tracker&q=... to find the tracker a bug report belongs to.",
			Returns: "a page of tickets with type, state and customer label, fields, version and linked issues.",
			Next: []string{
				"dimmi_tickets_get: read one with its links and clocks",
				"dimmi_tickets_links_create: attach a conversation to the tracker found",
			},
		},
		"tickets.store": {
			Title: "Create a ticket",
			Use: "Creates a ticket of a type: back-office work, a tracker, or a customer ticket for " +
				"a contact (which starts its conversation). Required fields are enforced: a 422 lists the missing " +
				"keys. Send external_id to make retries safe.",
			Returns: "the ticket with its reference, such as T-482.",
			Next:    []string{"dimmi_tickets_links_create: link the conversations that report it"},
			Params: map[string]string{
				"type_id": "A type id from dimmi_ticket_types_list.",
				"fields":  "Values by field key; the type's fields say each key's data type and options.",
				"link_to": "Links made at once, such as {kind: tracker, conversation_id} for each report.",
			},
		},
	}
}

/*
ticketChangeDocs returns the entries for reading and changing one
ticket.
*/
func ticketChangeDocs() map[string]Doc {
	return map[string]Doc{
		"tickets.show": {
			Title:   "Get a ticket",
			Use:     "Reads one ticket by id or reference (T-482) with its links, running SLA clocks and impact.",
			Returns: "the ticket; trackers include customers, companies and revenue reached.",
		},
		"tickets.update": {
			Title: "Update a ticket",
			Use: "Moves state, sets fields (merged; null removes), title, type, priority, teammate or team, " +
				"and accepts AI suggestions. Send version to refuse the change if someone edited it since. Moving " +
				"to a resolved state needs every field required to resolve.",
			Returns: "the ticket as it now is.",
			Hint:    "A 409 means it changed since you read it: read it again. A 422 lists missing field keys.",
		},
		"tickets.destroy": {
			Title: "Trash a ticket", Use: "Moves a ticket to the trash; restore brings it back.",
			Risk: new(RiskDanger),
		},
		"tickets.restore.store": {Title: "Restore a ticket", Use: "Takes a ticket out of the trash."},
		"tickets.escalations.store": {
			Title: "Escalate to back-office work",
			Use: "Creates back-office work for another team from a customer ticket and moves the customer " +
				"ticket to a state that waits on the team. When the work resolves, the customer ticket comes back.",
			Returns: "the new back-office ticket.",
		},
		"conversations.ticket.store": {
			Title: "Convert a conversation to a ticket",
			Use: "Turns a conversation into a customer ticket of a type. ai_fill proposes field values from the " +
				"transcript, kept as suggestions until accepted.",
			Returns: "the ticket.",
			Next:    []string{"dimmi_tickets_update: accept suggestions or move state"},
		},
	}
}

/*
ticketActivityDocs returns the entries for what happens around tickets.
*/
func ticketActivityDocs() map[string]Doc {
	return map[string]Doc{
		"tickets.events.index": {
			Title: "List a ticket's history", Use: "Reads every change to a ticket, oldest first.",
			Returns: "state, field, type, link, issue, assignment and note entries with who and when.",
		},
		"ticket-events.index": {
			Title: "Export the ticket log",
			Use:   "Follows every ticket change in the workspace after a cursor or since a time, deletes included.",
			Risk:  new(RiskSensitive),
		},
		"tickets.notes.store": {
			Title: "Add a ticket note", Use: "Leaves an internal note; a customer ticket's note also lands in its " +
				"conversation.",
		},
		"tickets.links.index": {Title: "List a ticket's links", Use: "Reads back-office, tracker, follow-up links."},
		"tickets.links.store": {
			Title: "Link a ticket",
			Use: "Links a tracker to a conversation (kind tracker), a customer ticket to back-office " +
				"work, or any two tickets (related, follow_up). Linking twice changes nothing.",
		},
		"ticket-links.destroy": {Title: "Remove a ticket link", Use: "Removes a link.", Risk: new(RiskDanger)},
		"tickets.conversations.index": {
			Title:   "List a tracker's conversations",
			Use:     "Lists the conversations a tracker reaches, with customer, company, plan and revenue.",
			Returns: "the conversations and totals under meta.impact.",
		},
		"tickets.updates.store": {
			Title: "Message everyone a tracker reaches",
			Use: "Sends one message to every linked customer on their own channel, optionally resolving them. " +
				"Always call with dry_run: true first to see the count and who is held back, then confirm.",
			Risk: new(RiskReach),
		},
		"tickets.updates.index": {
			Title: "List a tracker's messages", Use: "Reads past messages with each recipient's result.",
		},
		"conversations.tickets.index": {
			Title: "Get a conversation's tickets", Use: "Reads a conversation's own ticket and the trackers it joined.",
		},
		"conversations.ticket-suggestions.index": {
			Title: "Suggest ticket fields", Use: "Asks the AI for a type's field values from a conversation.",
		},
	}
}

/*
ticketSetupDocs returns the entries for types, fields and states.
*/
func ticketSetupDocs() map[string]Doc {
	setup := func(title, use string) Doc { return Doc{Title: title, Use: use, Risk: new(RiskDanger)} }

	return map[string]Doc{
		"ticket-types.index": {
			Title: "List ticket types", Use: "Lists types with their fields, allowed states and ticket counts.",
		},
		"ticket-types.show":  {Title: "Get a ticket type", Use: "Reads one type with its fields and states."},
		"ticket-types.store": setup("Create a ticket type", "Adds a type: customer, back_office or tracker."),
		"ticket-types.update": setup("Update a ticket type",
			"Changes a type's settings and the states it uses; its kind stays."),
		"ticket-types.destroy":      setup("Archive a ticket type", "Hides a type from new tickets; old ones keep it."),
		"ticket-types.fields.index": {Title: "List a type's fields", Use: "Lists a type's fields in order."},
		"ticket-types.fields.store": setup("Add a ticket field",
			"Adds a field such as order_number (short_text) or amount (currency); key and data type never change."),
		"ticket-types.fields.update-many": setup("Reorder ticket fields", "Sets the order of a type's fields."),
		"ticket-fields.update":            setup("Update a ticket field", "Changes a field's settings."),
		"ticket-fields.destroy":           setup("Archive a ticket field", "Hides a field; stored values stay."),
		"ticket-states.index": {
			Title: "List ticket states", Use: "Lists states in their five groups with the label customers see.",
		},
		"ticket-states.store":       setup("Add a ticket state", "Adds a state to a group."),
		"ticket-states.update-many": setup("Reorder ticket states", "Sets the order of the states."),
		"ticket-states.update":      setup("Update a ticket state", "Renames a state or changes its customer label."),
		"ticket-states.destroy": setup("Remove a ticket state",
			"Removes a state, moving its tickets to move_to, a state of the same group."),
	}
}
