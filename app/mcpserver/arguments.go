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

import "maps"

/*
idArgument explains an argument holding a record's id: what the record
is, the prefix its ids start with, and the tool that finds them.
*/
func idArgument(what, prefix, finder string) string {
	text := "Id of the " + what + ", starting with " + prefix + "_."
	if finder != "" {
		text += " Find it with " + finder + "."
	}

	return text
}

/*
argumentGuide explains arguments by name, the same way in every tool:
the ids of records and the arguments every list shares.
*/
func argumentGuide() map[string]string {
	out := listArguments()
	maps.Copy(out, idArguments())
	maps.Copy(out, featureIDArguments())

	return out
}

/*
listArguments explains the arguments every list shares.
*/
func listArguments() map[string]string {
	return map[string]string{
		"cursor": "Leave out for the first page. To read the next page, pass the next_cursor the previous page " +
			"returned.",
		"limit":   "How many items to return in one page.",
		"q":       "Search text.",
		"query":   "Search text.",
		"sort":    "Order of the items. A leading minus, such as -created_at, puts the newest or largest first.",
		"include": "Related records to include, separated by commas.",
	}
}

/*
idArguments explains the arguments that hold a record's id.
*/
func idArguments() map[string]string {
	return map[string]string{
		"conversation": "Id of the conversation (conv_...) or its number, such as 1042 for #1042. " +
			"Find it with dimmi_conversations_list or dimmi_search_list.",
		"message":         idArgument("message", "msg", "dimmi_conversations_messages_list"),
		"contact":         idArgument("contact (a customer or lead)", "ctc", "dimmi_contacts_list"),
		"company":         idArgument("company", "cmp", "dimmi_companies_list"),
		"inbox":           idArgument("inbox", "inbox", "dimmi_inboxes_list"),
		"team":            idArgument("team", "team", "dimmi_teams_list"),
		"member":          idArgument("teammate (workspace member)", "mem", "dimmi_members_list"),
		"tag":             idArgument("tag", "tag", "dimmi_tags_list"),
		"view":            idArgument("saved inbox view", "cvw", "dimmi_conversation_views_list"),
		"draft":           idArgument("reply draft", "drf", "dimmi_drafts_list"),
		"invitation":      idArgument("invitation", "inv", "dimmi_invitations_list"),
		"role":            idArgument("role", "rol", "dimmi_roles_list"),
		"audience":        idArgument("audience", "audn", "dimmi_audiences_list"),
		"attribute":       idArgument("custom attribute", "atd", "dimmi_attribute_definitions_list"),
		"event":           idArgument("event definition", "edf", "dimmi_event_definitions_list"),
		"data_export":     idArgument("data export", "dex", "dimmi_data_exports_list"),
		"export":          idArgument("export", "dex", "dimmi_data_exports_list"),
		"contact_erasure": idArgument("erasure request", "ces", "dimmi_contact_erasures_list"),
		"audit_log":       idArgument("audit log entry", "aud", "dimmi_audit_logs_list"),
		"help_center":     idArgument("help center", "hc", "dimmi_help_centers_list"),
		"collection":      idArgument("help center collection", "col", "dimmi_help_centers_collections_list"),
		"article":         idArgument("help article", "art", "dimmi_help_centers_articles_list"),
		"entry":           idArgument("changelog entry", "cle", "dimmi_help_centers_changelog_entries_list"),
		"label":           idArgument("changelog label", "cll", "dimmi_help_centers_changelog_labels_list"),
		"subscriber":      idArgument("changelog subscriber", "csub", ""),
		"post":            idArgument("feature request post", "fbp", "dimmi_help_centers_feedback_posts_list"),
		"comment":         idArgument("feature request comment", "fbc", "dimmi_feedback_posts_comments_list"),
		"procedure":       idArgument("AI agent procedure", "prc", "dimmi_procedures_list"),
		"workflow":        idArgument("workflow", "wf", "dimmi_workflows_list"),
		"run":             idArgument("run", "wfr or air", "the matching runs list"),
		"schedule":        idArgument("office hours schedule", "sch", "dimmi_schedules_list"),
		"action":          idArgument("AI agent action", "aac", "dimmi_ai_actions_list"),
		"connection":      idArgument("action connection", "acn", "dimmi_ai_action_connections_list"),
		"call":            idArgument("action call", "aax", "dimmi_ai_action_calls_list"),
		"key":             idArgument("AI provider key", "aik", "dimmi_ai_keys_list"),
		"thread":          idArgument("Operator thread", "opt", "dimmi_operator_threads_list"),
		"changeset":       idArgument("Operator changeset", "opc", "dimmi_operator_changesets_list"),
		"memory":          idArgument("Operator memory", "opy", "dimmi_operator_memories_list"),
		"address":         idArgument("sending email address", "ema", "dimmi_email_addresses_list"),
		"domain":          idArgument("sending email domain", "edm", "dimmi_email_domains_list"),
		"signature":       idArgument("email signature", "sig", "dimmi_email_signatures_list"),
		"secret":          idArgument("identity verification secret", "isk", "dimmi_identity_secrets_list"),
		"redaction_rule":  idArgument("redaction rule", "rxr", "dimmi_redaction_rules_list"),
		"widget":          idArgument("messenger", "wgt", "dimmi_widget_settings_list"),
	}
}

/*
featureIDArguments explains the ids of broadcasts, series, knowledge
sources and integrations.
*/
func featureIDArguments() map[string]string {
	return map[string]string{
		"broadcast":   idArgument("broadcast", "bcst", "dimmi_broadcasts_list"),
		"topic":       idArgument("broadcast topic", "bctp", "dimmi_broadcast_topics_list"),
		"series":      idArgument("series", "srs", "dimmi_series_list"),
		"enrollment":  idArgument("journey on a series", "sren", "dimmi_series_enrollments_list"),
		"source":      idArgument("knowledge source", "ks", "dimmi_knowledge_sources_list"),
		"document":    idArgument("knowledge source document", "kd", "dimmi_knowledge_sources_documents_list"),
		"destination": idArgument("notification destination", "ndst", "dimmi_notification_destinations_list"),
		"integration": idArgument("connected integration", "itg", "dimmi_integrations_list"),
		"channel":     idArgument("Slack connection", "chn", "dimmi_integrations_get (its channel_ids)"),
		"room":        idArgument("Slack channel a connection watches", "chr", "dimmi_channels_rooms_list"),
	}
}
