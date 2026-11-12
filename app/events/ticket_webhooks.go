/*
|--------------------------------------------------------------------------
| Ticket Webhooks
|--------------------------------------------------------------------------
|
| The subscribable events of tickets, trackers and incidents. Each
| carries the ticket as the REST API shows it, what changed from and to,
| and who changed it. Incidents fire the same events with their kind,
| plus their public posts and the spikes the detector suggests.
|
*/

package events

/*
ticketWebhooks lists the events about tickets.
*/
func ticketWebhooks() []WebhookEventType {
	t := func(name, description string) WebhookEventType {
		return WebhookEventType{Type: "ticket." + name, Object: "ticket", Group: "Tickets", Description: description}
	}

	return []WebhookEventType{
		t("created", "A ticket was created or a conversation converted."),
		t("updated", "A ticket's title, description or field values changed."),
		t("state_changed", "A ticket moved to another state."),
		t("resolved", "A ticket entered a resolved state."),
		t("reopened", "A resolved ticket left the resolved states."),
		t("assigned", "A ticket's teammate or team changed."),
		t("type_changed", "A ticket moved to another type."),
		t("linked", "A ticket was linked to another ticket or a conversation."),
		t("unlinked", "A ticket link was removed."),
		t("issue_linked", "A Linear, Jira or GitHub issue or an engineering incident was linked."),
		t("issue_updated", "A linked issue's status changed."),
		{
			Type: "ticket.note_created", Object: "ticket", Group: "Tickets", NotesOptIn: true,
			Description: "A teammate left an internal note on a ticket.",
		},
		t("update_sent", "A tracker's message to its linked customers finished sending."),
		t("public_update_posted", "An incident posted to its public update log."),
		t("sla_warning", "A ticket's service level target is about to be missed."),
		t("sla_breached", "A ticket missed a service level target."),
		t("deleted", "A ticket moved to the trash."),
		t("restored", "A ticket came back from the trash."),
		{
			Type: "incident.suggested", Object: "incident_suggestion", Group: "Tickets",
			Description: "A spike of conversations about one thing suggests an incident.",
		},
	}
}
