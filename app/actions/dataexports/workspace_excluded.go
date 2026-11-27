/*
|--------------------------------------------------------------------------
| Tables a Workspace Export Leaves Out
|--------------------------------------------------------------------------
|
| A workspace export carries what the workspace wrote and set up. It
| leaves out secrets, logs that have their own export, what is worked
| out from exported records and can be worked out again, and the
| application's own bookkeeping. Each table left out says why.
|
*/

package dataexports

import "maps"

/*
ArchiveExclusions returns every table of a workspace that its export
leaves out, with the reason.
*/
func ArchiveExclusions() map[string]string {
	out := secretTables()
	maps.Copy(out, logTables())
	maps.Copy(out, derivedTables())
	maps.Copy(out, bookkeepingTables())

	return out
}

/*
secretTables hold credentials or proofs of access and nothing else
worth taking.
*/
func secretTables() map[string]string {
	//nolint:gosec // table names with why each is left out, not credentials
	return map[string]string{
		"ai_provider_keys":          "secret: AI provider API keys",
		"identity_secrets":          "secret: identity verification signing keys",
		"integration_installations": "secret: OAuth credentials of connected apps",
		"integration_member_grants": "secret: members' OAuth credentials",
		"personal_access_tokens":    "secret: API token hashes",
		"widget_sessions":           "secret: visitors' session token hashes",
		"email_threads":             "secret: reply tokens that route inbound mail",
		"email_address_tests":       "secret: one-time test tokens",
		"contact_erasures":          "erasure ledger: holds what erased customers must not come back as",
	}
}

/*
logTables are logs and traces: the audit log has its own export, and
the rest record how the application ran, not what the workspace wrote.
*/
func logTables() map[string]string {
	return map[string]string{
		"audit_logs":                "log: the audit log has its own export",
		"ai_runs":                   "trace: one row per AI run, for debugging",
		"ai_run_steps":              "trace: AI debug steps with provider requests",
		"ai_action_calls":           "trace: requests AI actions sent, which may carry credentials",
		"ai_agent_test_case_runs":   "trace: results of test runs",
		"channel_inbound_events":    "raw channel webhook payloads",
		"inbound_emails":            "log: raw inbound mail; the messages made from it are exported",
		"outbound_emails":           "log: mail deliveries; delivery status is on each message",
		"webhook_deliveries":        "log: webhook deliveries and their payloads",
		"webhook_delivery_attempts": "log: webhook delivery attempts",
		"notification_deliveries":   "log: notifications sent to members",
		"knowledge_sync_runs":       "log: knowledge sync runs",
		"workflow_runs":             "log: workflow runs; what they did is on the conversations",
		"workflow_run_steps":        "log: workflow run steps",
		"workflow_match_logs":       "trace: why workflows matched",
		"workflow_simulation_runs":  "log: each run of a workflow simulation; the latest result is on the simulation",
		"attribute_problems":        "log: rejected attribute writes",
	}
}

/*
derivedTables are worked out from exported records, or count usage.
*/
func derivedTables() map[string]string {
	return map[string]string{
		"knowledge_embeddings":         "embeddings",
		"knowledge_embedding_failures": "embeddings bookkeeping",
		"knowledge_chunks":             "derived: articles cut up for search",
		"knowledge_document_chunks":    "derived: documents cut up for search",
		"knowledge_conversations":      "derived: closed conversations prepared for search",
		"conversation_facts":           "derived: report facts, with their own export",
		"conversation_member_facts":    "derived: report facts",
		"conversation_reply_facts":     "derived: report facts",
		"backlog_snapshots":            "derived: report snapshots",
		"article_views":                "counter: daily article views",
		"changelog_entry_views":        "counter: daily changelog views",
		"help_search_terms":            "counter: daily help center searches",
		"email_sending_stats":          "counter: daily mail totals",
		"identity_verification_stats":  "counter: daily verification totals",
		"ai_spend_periods":             "counter: AI usage per period",
		"knowledge_usage_periods":      "counter: knowledge usage per period",
		"tracker_suggestions":          "derived: suggestions the AI made and may make again",
	}
}

/*
bookkeepingTables keep the application running: cursors, states,
pending work and files with their own lifetime.
*/
func bookkeepingTables() map[string]string {
	return map[string]string{
		"data_exports":                  "bookkeeping: exports themselves",
		"contact_exports":               "bookkeeping: exports themselves",
		"imports":                       "bookkeeping: imports and their uploaded files",
		"help_center_theme_files":       "files: each theme version downloads whole as a zip from the API",
		"import_changes":                "bookkeeping: what an import can undo",
		"ticket_imports":                "bookkeeping: ticket imports",
		"ticket_import_records":         "bookkeeping: what a ticket import can undo",
		"migrations":                    "bookkeeping: migrations from other tools",
		"migration_records":             "bookkeeping: what a migration brought and can undo",
		"channel_contact_states":        "state: reply windows per channel",
		"channel_conversation_posts":    "state: where a channel posted a conversation",
		"channel_message_refs":          "state: providers' ids of sent messages",
		"email_message_refs":            "state: mail ids used to thread replies",
		"email_sending_profiles":        "state: sending limits the platform sets",
		"conversation_drafts":           "state: members' unsent drafts",
		"conversation_reads":            "state: members' read positions",
		"conversation_procedure_states": "state: where a procedure stands in a conversation",
		"operator_slack_threads":        "state: where the Operator answers in Slack",
		"report_pending":                "state: conversations waiting for their facts",
		"report_states":                 "state: report rebuild progress",
		"team_assignment_cursors":       "state: whose turn it is",
	}
}
