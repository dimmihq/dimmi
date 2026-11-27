/*
|--------------------------------------------------------------------------
| Workspace Export Tables
|--------------------------------------------------------------------------
|
| Every table a workspace owns is listed here on purpose: written to the
| archive, with the columns that never leave, or left out with the
| reason. A table added with a workspace_id column must be added to one
| of the two lists; a test fails until it is, so nothing is exported or
| forgotten by accident. Secrets never leave: sealed credentials, token
| hashes and signing keys are named in Omit, and columns of bytes,
| search vectors and embeddings are dropped whatever the table.
|
*/

package dataexports

import "slices"

/*
ArchiveTable is one table a workspace export writes as a file of JSON
lines, named after the table. Key holds the unique columns its rows are
paged by, each "column" for a uuid or "column:type"; a table without
one is small and read in one pass. Omit names the columns never
written. Scope is the column holding the workspace, workspace_id unless
said. Extra is a jsonb expression merged into each row, t being the
row.
*/
type ArchiveTable struct {
	Name  string
	About string
	Key   []string
	Omit  []string
	Scope string
	Extra string
}

/*
archiveFiles is a table whose rows name stored files the archive
carries. Disk, Path and Ext are expressions over the row t: the disk
("" for the default one), the path on it, and the text the file's
extension is read from. Blocked is an expression giving why a file is
withheld, null when it may leave.
*/
type archiveFiles struct {
	Table   string
	Disk    string
	Path    string
	Ext     string
	Blocked string
}

/*
byID pages a table by its id.
*/
func byID() []string { return []string{"id"} }

/*
ArchiveTables returns every table a workspace export writes, in the
order the archive lists them.
*/
func ArchiveTables() []ArchiveTable {
	return slices.Concat(
		workspaceTables(), peopleTables(), conversationTables(), ticketTables(), helpCenterTables(),
		knowledgeTables(), feedbackTables(), outreachTables(), automationTables(), agentTables(), channelTables(),
	)
}

/*
workspaceTables are the workspace itself, its members and how it is
set up.
*/
func workspaceTables() []ArchiveTable {
	member := `(SELECT jsonb_build_object('name', u.name, 'email', u.email) FROM users u WHERE u.id = t.user_id)`

	return []ArchiveTable{
		{Name: "workspaces", About: "The workspace and its settings.", Scope: "id", Omit: []string{"logo_path"}},
		{Name: "workspace_members", About: "Members with name, email and role.", Key: byID(), Extra: member},
		{Name: "roles", About: "Custom roles and their permissions.", Key: byID()},
		{Name: "invitations", About: "Invitations sent to join.", Key: byID(), Omit: []string{"token_hash"}},
		{Name: "teams", About: "Teams.", Key: byID()},
		{Name: "team_members", About: "Who is on which team.", Key: []string{"team_id", "workspace_member_id"}},
		{
			Name: "member_access_teams", About: "Teams a member's access is limited to.",
			Key: []string{"member_id", "team_id"},
		},
		{Name: "member_inbox_preferences", About: "Each member's inbox layout.", Key: []string{"member_id"}},
		{Name: "inboxes", About: "Inboxes and their settings.", Key: byID()},
		{Name: "inbox_access", About: "Teams and members an inbox is limited to."},
		{Name: "conversation_views", About: "Saved views.", Key: byID()},
		{Name: "schedules", About: "Business hours and holidays.", Key: byID()},
		{Name: "sla_policies", About: "SLA policies.", Key: byID()},
		{Name: "saved_replies", About: "Saved replies (macros).", Key: byID()},
		{Name: "tags", About: "Tags.", Key: byID()},
		{Name: "snooze_lists", About: "Snooze options per team."},
		{Name: "redaction_rules", About: "Rules that mask sensitive text.", Key: byID()},
		{Name: "attribute_definitions", About: "Custom attribute definitions.", Key: byID()},
		{Name: "ai_conversation_attributes", About: "Attributes the AI reads from conversations.", Key: byID()},
		{Name: "event_definitions", About: "Customer event definitions.", Key: byID()},
		{Name: "conversation_rating_settings", About: "How satisfaction ratings are asked."},
		{Name: "identity_settings", About: "How customers' identities are verified."},
		{Name: "notification_destinations", About: "Where the team is notified.", Key: byID()},
		{Name: "notification_preferences", About: "Each member's notification choices.", Key: byID()},
		{Name: "notification_settings", About: "Each member's quiet hours.", Key: []string{"workspace_member_id"}},
		{Name: "report_digests", About: "Report digests members subscribed to.", Key: byID()},
		{
			Name: "webhook_endpoints", About: "Webhook endpoints, without their signing secrets.", Key: byID(),
			Omit: []string{"secret_encrypted", "previous_secret_encrypted"},
		},
	}
}

/*
peopleTables are customers, companies and what is known about them.
*/
func peopleTables() []ArchiveTable {
	return []ArchiveTable{
		{Name: "contacts", About: "Customers.", Key: byID()},
		{Name: "contact_identities", About: "Customers' addresses and handles per channel.", Key: byID()},
		{Name: "companies", About: "Companies.", Key: byID()},
		{
			Name: "contact_companies", About: "The companies each customer belongs to, and their default.",
			Key: []string{"contact_id", "company_id"},
		},
		{Name: "contact_tag", About: "Tags on customers.", Key: []string{"contact_id", "tag_id"}},
		{Name: "company_tag", About: "Tags on companies.", Key: []string{"company_id", "tag_id"}},
		{Name: "customer_notes", About: "The team's notes on customers and companies.", Key: byID()},
		{
			Name: "contact_event_summaries", About: "Events each customer triggered.",
			Key: []string{"contact_id", "event_definition_id"},
		},
		{Name: "audiences", About: "Audiences (segments) and their rules.", Key: byID()},
	}
}

/*
conversationTables are conversations and everything said in them.
*/
func conversationTables() []ArchiveTable {
	return []ArchiveTable{
		{Name: "conversations", About: "Conversations.", Key: byID()},
		{
			Name: "messages", About: "Messages, internal notes and events of every conversation.", Key: byID(),
			Omit: []string{"dedupe_key"},
		},
		{
			Name: "conversation_participants", About: "Customers taking part in a conversation.",
			Key: []string{"conversation_id", "contact_id"},
		},
		{Name: "conversation_tag", About: "Tags on conversations.", Key: []string{"conversation_id", "tag_id"}},
		{
			Name: "conversation_ai_values", About: "Values the AI read for each conversation.",
			Key: []string{"conversation_id"},
		},
		{Name: "conversation_ratings", About: "Satisfaction ratings.", Key: byID()},
		{Name: "conversation_events", About: "Assignments, closes and reopens of conversations.", Key: byID()},
		{
			Name: "conversation_sla_policies", About: "The SLA policy each conversation follows.",
			Key: []string{"conversation_id"},
		},
		{Name: "sla_clocks", About: "SLA targets and whether they were met.", Key: byID()},
		{
			Name: "message_mentions", About: "Members mentioned in notes.",
			Key: []string{"message_id", "workspace_member_id"},
		},
		{
			Name: "message_reactions", About: "Reactions on messages.",
			Key: []string{"message_id", "member_id", "emoji:text"},
		},
		{
			Name: "attachments", About: "Files on messages; archive_path is the file in this archive.", Key: byID(),
			Omit: []string{"disk", "path"},
		},
		{Name: "email_signatures", About: "Email signatures.", Key: byID()},
		{Name: "email_suppressions", About: "Addresses mail is no longer sent to.", Key: byID()},
	}
}

/*
ticketTables are tickets and how they are set up.
*/
func ticketTables() []ArchiveTable {
	return []ArchiveTable{
		{Name: "tickets", About: "Tickets.", Key: byID()},
		{Name: "ticket_types", About: "Ticket types.", Key: byID()},
		{Name: "ticket_fields", About: "Ticket fields per type.", Key: byID()},
		{Name: "ticket_states", About: "Ticket states.", Key: byID()},
		{Name: "ticket_type_states", About: "States each ticket type uses.", Key: []string{"type_id", "state_id"}},
		{Name: "ticket_events", About: "The history of each ticket.", Key: byID()},
		{Name: "ticket_links", About: "Links between tickets and conversations.", Key: byID()},
		{Name: "ticket_issues", About: "Issues in a tracker linked to tickets.", Key: byID()},
		{Name: "ticket_summaries", About: "Summaries of tickets.", Key: []string{"ticket_id"}},
	}
}

/*
helpCenterTables are help centers, their articles and changelogs.
*/
func helpCenterTables() []ArchiveTable {
	return []ArchiveTable{
		{Name: "help_centers", About: "Help centers and their settings.", Key: byID()},
		{
			Name: "help_center_domains", About: "Custom domains of help centers.", Key: byID(),
			Omit: []string{"verification_token"},
		},
		{Name: "help_center_redirects", About: "Redirects of help centers.", Key: byID()},
		{
			Name: "help_center_themes", About: "Theme versions of help centers and their settings.", Key: byID(),
		},
		{
			Name: "help_center_images", About: "Pictures in articles; archive_path is the file in this archive.",
			Key: byID(),
		},
		{Name: "collections", About: "Help center collections.", Key: byID()},
		{
			Name: "articles", About: "Help center articles as they are being edited.", Key: byID(),
			Omit: []string{"title_trgm"},
		},
		{Name: "article_versions", About: "Every stored version of each article.", Key: byID()},
		{Name: "article_translations", About: "Articles' translations into other languages.", Key: byID()},
		{Name: "collection_translations", About: "Collections' names in other languages.", Key: byID()},
		{Name: "article_ratings", About: "Readers' ratings of articles.", Key: byID()},
		{Name: "changelog_entries", About: "Changelog entries.", Key: byID()},
		{Name: "changelog_labels", About: "Changelog labels.", Key: byID()},
		{Name: "changelog_entry_labels", About: "Labels on changelog entries.", Key: []string{"entry_id", "label_id"}},
		{
			Name: "changelog_entry_people", About: "Authors of changelog entries.",
			Key: []string{"entry_id", "role:text", "member_id"},
		},
		{
			Name: "changelog_reactions", About: "Reactions on changelog entries.",
			Key: []string{"entry_id", "reactor_hash:text", "emoji:text"},
		},
		{
			Name: "changelog_subscribers", About: "Subscribers to the changelog.", Key: byID(),
			Omit: []string{"nonce"},
		},
	}
}

/*
knowledgeTables are the sources the AI reads and what they hold.
*/
func knowledgeTables() []ArchiveTable {
	return []ArchiveTable{
		{
			Name: "knowledge_sources", About: "Knowledge sources; archive_path is an uploaded file in this archive.",
			Key: byID(), Omit: []string{"credentials_encrypted"},
		},
		{Name: "knowledge_documents", About: "Documents read from knowledge sources, with their text.", Key: byID()},
		{Name: "knowledge_settings", About: "Which model indexes knowledge."},
	}
}

/*
feedbackTables are feedback boards and what customers posted.
*/
func feedbackTables() []ArchiveTable {
	return []ArchiveTable{
		{Name: "feedback_boards", About: "Feedback boards.", Key: byID()},
		{Name: "feedback_statuses", About: "Feedback statuses.", Key: byID()},
		{Name: "feedback_status_positions", About: "The order of feedback statuses.", Key: []string{"status:text"}},
		{Name: "feedback_posts", About: "Feedback posts.", Key: byID(), Omit: []string{"title_trgm"}},
		{Name: "feedback_votes", About: "Votes on feedback posts.", Key: []string{"post_id", "contact_id"}},
		{Name: "feedback_comments", About: "Comments on feedback posts.", Key: byID()},
		{
			Name: "feedback_post_conversations", About: "Conversations linked to feedback posts.",
			Key: []string{"post_id", "conversation_id"},
		},
	}
}

/*
outreachTables are broadcasts and series with what happened to each
recipient.
*/
func outreachTables() []ArchiveTable {
	return []ArchiveTable{
		{Name: "broadcasts", About: "Broadcasts.", Key: byID()},
		{
			Name: "broadcast_recipients", About: "What happened for each recipient of a broadcast.", Key: byID(),
			Omit: []string{"reply_token"},
		},
		{
			Name: "broadcast_links", About: "Links in broadcasts and their clicks.",
			Key: []string{"broadcast_id", "n:int"},
		},
		{Name: "broadcast_topics", About: "Subscription topics.", Key: byID()},
		{Name: "broadcast_opt_outs", About: "Customers who opted out of broadcasts.", Key: []string{"contact_id"}},
		{
			Name: "contact_topic_subscriptions", About: "Customers' choices per topic.",
			Key: []string{"contact_id", "topic_id"},
		},
		{Name: "series", About: "Series.", Key: byID()},
		{Name: "series_versions", About: "Published versions of series.", Key: byID()},
		{Name: "series_enrollments", About: "Who entered each series and how it ended.", Key: byID()},
		{Name: "series_steps", About: "Each step a recipient went through in a series.", Key: byID()},
	}
}

/*
automationTables are workflows and procedures.
*/
func automationTables() []ArchiveTable {
	return []ArchiveTable{
		{Name: "workflows", About: "Workflows.", Key: byID()},
		{Name: "workflow_versions", About: "Published versions of workflows.", Key: byID()},
		{Name: "workflow_simulations", About: "Saved simulations of workflows with their latest results.", Key: byID()},
		{Name: "procedures", About: "Procedures the AI follows.", Key: byID()},
		{Name: "procedure_versions", About: "Published versions of procedures.", Key: byID()},
		{Name: "procedure_test_cases", About: "Test cases of procedures.", Key: byID()},
	}
}

/*
agentTables are the AI agents, their actions and the Operator.
*/
func agentTables() []ArchiveTable {
	return []ArchiveTable{
		{Name: "ai_agents", About: "AI agents: guidance and settings, never provider keys.", Key: byID()},
		{Name: "ai_agent_versions", About: "Published versions of AI agents.", Key: byID()},
		{Name: "ai_agent_test_cases", About: "Test cases of AI agents.", Key: byID()},
		{Name: "workspace_ai_settings", About: "Which models the workspace's AI uses."},
		{Name: "ai_actions", About: "Actions the AI may take.", Key: byID(), Omit: []string{"last_test_response"}},
		{Name: "ai_action_versions", About: "Published versions of AI actions.", Key: byID()},
		{
			Name: "ai_action_connections", About: "Systems AI actions reach, without their credentials.", Key: byID(),
			Omit: []string{"credentials_encrypted", "secret_hint"},
		},
		{Name: "operator_settings", About: "The Operator's instructions and settings."},
		{Name: "operator_memories", About: "What the Operator was told to remember.", Key: byID()},
		{Name: "operator_threads", About: "Members' threads with the Operator.", Key: byID()},
		{Name: "operator_messages", About: "Messages in Operator threads.", Key: byID(), Omit: []string{"steps"}},
		{Name: "operator_changesets", About: "Changes the Operator proposed.", Key: byID()},
		{Name: "operator_routines", About: "Routines the Operator runs on a schedule.", Key: byID()},
		{Name: "operator_routine_runs", About: "Each run of a routine.", Key: byID(), Omit: []string{"steps"}},
	}
}

/*
channelTables are the connected channels and email setup, without what
signs them in.
*/
func channelTables() []ArchiveTable {
	return []ArchiveTable{
		{
			Name: "channel_connections", About: "Connected channels, without their credentials.", Key: byID(),
			Omit: []string{"credentials_encrypted"},
		},
		{Name: "channel_rooms", About: "Chat rooms and channels the workspace answers in.", Key: byID()},
		{Name: "channel_member_links", About: "Members' linked chat accounts.", Key: byID()},
		{Name: "widget_settings", About: "Messenger settings.", Key: byID()},
		{Name: "whatsapp_templates", About: "WhatsApp message templates.", Key: byID()},
		{
			Name: "email_addresses", About: "Support email addresses.", Key: byID(),
			Omit: []string{"forwarding_confirmation"},
		},
		{Name: "email_domains", About: "Sending domains, without their DKIM private keys.", Key: byID(), Omit: []string{
			"dkim_private_key_encrypted", "dkim_next_private_key_encrypted", "ownership_token", "dkim_tokens",
		}},
	}
}

/*
archiveFileTables returns the tables whose rows name files the archive
carries under attachments/.
*/
func archiveFileTables() []archiveFiles {
	upload := `CASE WHEN starts_with(t.config->>'upload', 'knowledge-uploads/' || t.workspace_id || '/')
		THEN t.config->>'upload' END`

	return []archiveFiles{
		{
			Table: "attachments", Disk: "t.disk", Path: "t.path", Ext: "t.filename",
			Blocked: `CASE WHEN t.scan_status NOT IN ('clean', 'skipped') THEN 'scan ' || t.scan_status END`,
		},
		{Table: "help_center_images", Disk: "''", Path: "t.path", Ext: "t.path", Blocked: "NULL::text"},
		{Table: "knowledge_sources", Disk: "''", Path: upload, Ext: "t.config->>'upload'", Blocked: "NULL::text"},
	}
}
