/*
|--------------------------------------------------------------------------
| Arguments
|--------------------------------------------------------------------------
|
| Most tools take the id of something, and an agent that does not know
| where ids come from guesses. Every argument named after a record says
| what its ids look like and which tool lists them, and the list
| arguments every list tool shares say how paging and filtering work.
|
*/

package mcpserver

import (
	"maps"
	"strings"
)

/*
idGuide is what is known about an argument holding a record's id: what
the record is, the prefix its ids start with, and the tool that finds
them. Text is the whole explanation when it is written by hand.
*/
type idGuide struct {
	what   string
	prefix string
	finder string
	text   string
}

/*
idArgument describes an argument holding a record's id.
*/
func idArgument(what, prefix, finder string) idGuide {
	return idGuide{what: what, prefix: prefix, finder: finder}
}

/*
String explains the argument: what the record is, what its ids look
like and where to find one.
*/
func (g idGuide) String() string {
	if g.text != "" {
		return g.text
	}

	text := "Id of the " + g.what + ", starting with " + g.prefix + "_, such as " + g.example() + "."
	if g.finder != "" {
		text += " Find it with " + g.finder + "."
	}

	return text
}

/*
example is what one of the record's ids looks like.
*/
func (g idGuide) example() string {
	prefix, _, _ := strings.Cut(g.prefix, " ")

	return prefix + "_01jx2m8k4t"
}

/*
argumentGuide explains arguments by the name the API gives them, the
same way in every tool: the ids of records and the arguments every list
shares.
*/
func argumentGuide() map[string]string {
	out := listArguments()
	for name, guide := range idGuides() {
		out[name] = guide.String()
	}

	return out
}

/*
idGuides are the arguments that hold a record's id, by the name the API
gives them.
*/
func idGuides() map[string]idGuide {
	out := idArguments()
	maps.Copy(out, featureIDArguments())
	maps.Copy(out, ticketIDArguments())
	maps.Copy(out, agentIDArguments())

	return out
}

/*
agentIDArguments explains the ids of AI agents and their simulations.
*/
func agentIDArguments() map[string]idGuide {
	return map[string]idGuide{
		"agent":      idArgument("AI agent", "aia", "dimmi_list_ai_agents"),
		"simulation": idArgument("AI agent simulation", "aias", "dimmi_list_ai_agent_simulations"),
	}
}

/*
listArguments explains the arguments every list shares.
*/
func listArguments() map[string]string {
	return map[string]string{
		"cursor": "Leave out for the first page. To read the next page, pass the next_cursor the previous page " +
			"returned.",
		"limit":   "How many items to return in one page, such as 25.",
		"q":       "Words to search for, such as refund.",
		"query":   "Words to search for, such as refund.",
		"sort":    "Order of the items. A leading minus, such as -created_at, puts the newest or largest first.",
		"include": "Related records to include, separated by commas.",
	}
}

/*
idArguments explains the arguments that hold a record's id.
*/
func idArguments() map[string]idGuide {
	return map[string]idGuide{
		"conversation": {what: "conversation", prefix: "conv", text: "Id of the conversation, such as " +
			"conv_01jx2m8k4t, or its number, such as 1042 for #1042. Find it with dimmi_list_conversations or " +
			"dimmi_search_conversations_and_contacts."},
		"message":         idArgument("message", "msg", "dimmi_list_conversation_messages"),
		"contact":         idArgument("contact (a customer or lead)", "ctc", "dimmi_list_contacts"),
		"company":         idArgument("company", "cmp", "dimmi_list_companies"),
		"inbox":           idArgument("inbox", "inbox", "dimmi_list_inboxes"),
		"team":            idArgument("team", "team", "dimmi_list_teams"),
		"member":          idArgument("teammate (workspace member)", "mem", "dimmi_list_members"),
		"tag":             idArgument("tag", "tag", "dimmi_list_tags"),
		"view":            idArgument("saved inbox view", "cvw", "dimmi_list_conversation_views"),
		"draft":           idArgument("reply draft", "drf", "dimmi_list_drafts"),
		"invitation":      idArgument("invitation", "inv", "dimmi_list_invitations"),
		"role":            idArgument("role", "rol", "dimmi_list_roles"),
		"audience":        idArgument("audience", "audn", "dimmi_list_audiences"),
		"attribute":       idArgument("custom attribute", "atd", "dimmi_list_attribute_definitions"),
		"event":           idArgument("event definition", "edf", "dimmi_list_event_definitions"),
		"data_export":     idArgument("data export", "dex", "dimmi_list_data_exports"),
		"export":          idArgument("export", "dex", "dimmi_list_data_exports"),
		"contact_erasure": idArgument("erasure request", "ces", "dimmi_list_contact_erasures"),
		"audit_log":       idArgument("audit log entry", "aud", "dimmi_list_audit_logs"),
		"help_center":     idArgument("help center", "hc", "dimmi_list_help_centers"),
		"collection":      idArgument("help center collection", "col", "dimmi_list_help_center_collections"),
		"article":         idArgument("help article", "art", "dimmi_list_help_center_articles"),
		"entry":           idArgument("changelog entry", "cle", "dimmi_list_help_center_changelog_entries"),
		"label":           idArgument("changelog label", "cll", "dimmi_list_help_center_changelog_labels"),
		"subscriber":      idArgument("changelog subscriber", "csub", ""),
		"post":            idArgument("feature request post", "fbp", "dimmi_list_help_center_feedback_posts"),
		"comment":         idArgument("feature request comment", "fbc", "dimmi_list_feedback_post_comments"),
		"procedure":       idArgument("AI agent procedure", "prc", "dimmi_list_procedures"),
		"workflow":        idArgument("workflow", "wf", "dimmi_list_workflows"),
		"run":             idArgument("run", "wfr or air", "the matching runs list"),
		"schedule":        idArgument("office hours schedule", "sch", "dimmi_list_schedules"),
		"action":          idArgument("AI agent action", "aac", "dimmi_list_ai_actions"),
		"connection":      idArgument("action connection", "acn", "dimmi_list_ai_action_connections"),
		"call":            idArgument("action call", "aax", "dimmi_list_ai_action_calls"),
		"key":             idArgument("AI provider key", "aik", "dimmi_list_ai_keys"),
		"thread":          idArgument("Operator thread", "opt", "dimmi_list_operator_threads"),
		"changeset":       idArgument("Operator changeset", "opc", "dimmi_list_operator_suggestions"),
		"memory":          idArgument("Operator memory", "opy", "dimmi_list_operator_memories"),
		"address":         idArgument("sending email address", "ema", "dimmi_list_email_addresses"),
		"domain":          idArgument("sending email domain", "edm", "dimmi_list_email_domains"),
		"signature":       idArgument("email signature", "sig", "dimmi_list_email_signatures"),
		"secret":          idArgument("identity verification secret", "isk", "dimmi_list_identity_secrets"),
		"redaction_rule":  idArgument("redaction rule", "rxr", "dimmi_list_redaction_rules"),
		"saved_reply":     idArgument("saved reply", "rpl", "dimmi_list_saved_replies"),
		"widget":          idArgument("messenger", "wgt", "dimmi_list_widget_settings"),
	}
}

/*
featureIDArguments explains the ids of broadcasts, series, knowledge
sources and integrations.
*/
func featureIDArguments() map[string]idGuide {
	return map[string]idGuide{
		"broadcast":         idArgument("broadcast", "bcst", "dimmi_list_broadcasts"),
		"topic":             idArgument("broadcast topic", "bctp", "dimmi_list_broadcast_topics"),
		"series":            idArgument("series", "srs", "dimmi_list_series"),
		"enrollment":        idArgument("journey on a series", "sren", "dimmi_list_series_enrollments"),
		"source":            idArgument("knowledge source", "ks", "dimmi_list_knowledge_sources"),
		"document":          idArgument("knowledge source document", "kd", "dimmi_list_knowledge_source_documents"),
		"destination":       idArgument("notification destination", "ndst", "dimmi_list_notification_destinations"),
		"integration":       idArgument("connected integration", "itg", "dimmi_list_integrations"),
		"channel":           idArgument("Slack connection", "chn", "dimmi_get_integration_by_id (its channel_ids)"),
		"room":              idArgument("Slack channel a connection watches", "chr", "dimmi_list_channel_rooms"),
		"endpoint":          idArgument("webhook endpoint", "whk", "dimmi_list_webhook_endpoints"),
		"delivery":          idArgument("webhook delivery", "whd", "dimmi_list_webhook_endpoint_deliveries"),
		"routine":           idArgument("Operator routine", "ort", "dimmi_list_operator_routines"),
		"test_case":         idArgument("procedure simulation", "prct", "dimmi_list_procedure_test_cases"),
		"version":           idArgument("saved version", "prcv or srsv", "the matching versions list"),
		"policy":            idArgument("SLA policy", "sla", "dimmi_list_sla_policies"),
		"board":             idArgument("feedback board", "fbd", "dimmi_list_help_center_feedback_boards"),
		"note":              idArgument("note about a customer or company", "cnt", "dimmi_list_contact_notes"),
		"identity":          idArgument("contact identity", "cid", "dimmi_list_contact_identities"),
		"import":            idArgument("import", "imp", "dimmi_list_imports"),
		"template":          idArgument("WhatsApp template", "wtp", "dimmi_list_channel_whatsapp_templates"),
		"test":              idArgument("forwarding test", "eat", "dimmi_test_email_address"),
		"notification":      idArgument("notification", "ntf", "dimmi_list_notifications"),
		"installation":      idArgument("connected tool", "itg", "dimmi_list_integrations"),
		"email_suppression": idArgument("email suppression", "sup", "dimmi_list_email_suppressions"),
	}
}

/*
ticketIDArguments explains the ids of tickets and what they are made of.
*/
func ticketIDArguments() map[string]idGuide {
	return map[string]idGuide{
		"ticket": idArgument("ticket", "tkt", "dimmi_list_tickets"),
		"type":   idArgument("ticket type", "tktt", "dimmi_list_ticket_types"),
		"field":  idArgument("ticket field", "tktf", "dimmi_list_ticket_type_fields"),
		"state":  idArgument("ticket state", "tkts", "dimmi_list_ticket_states"),
		"link":   idArgument("ticket link", "tktl", "dimmi_list_ticket_links"),
		"issue":  idArgument("linked issue", "tkti", "dimmi_list_ticket_issues"),
		"status": {what: "feature request status", text: "Key of the feature request status, such as planned. " +
			"Find it with dimmi_list_feedback_statuses."},
		"node": {what: "step of the series", text: "Id of the step in the series, as dimmi_get_series_by_id shows it."},
	}
}
