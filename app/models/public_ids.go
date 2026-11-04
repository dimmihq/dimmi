/*
|--------------------------------------------------------------------------
| Public ID Prefixes
|--------------------------------------------------------------------------
|
| Rows are keyed by UUIDv7, but clients only ever see public ids such as
| usr_01jc...: a prefix naming the kind of thing, then the UUID in
| Crockford base32. The prefix makes a pasted id readable and lets the
| server refuse an id of the wrong kind before any query, as 404.
|
| Every prefix of the product is listed here, those of tables other
| areas will build and those reserved for tables nobody builds yet
| included, so a new table can never take a prefix another one owns or
| will own. Registering them all is part of boot; a clash stops it.
|
*/

package models

import (
	"errors"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Prefix names the kind of row a public id points at.
*/
type Prefix string

/*
The prefixes of the account tables and of the tables other parts of the
product read here.
*/
const (
	PrefixWidget             Prefix = "wgt"
	PrefixWidgetSession      Prefix = "wss"
	PrefixUser               Prefix = "usr"
	PrefixWorkspace          Prefix = "ws"
	PrefixOrganization       Prefix = "org"
	PrefixMember             Prefix = "mem"
	PrefixTeam               Prefix = "team"
	PrefixInvitation         Prefix = "inv"
	PrefixSession            Prefix = "ses"
	PrefixPersonalToken      Prefix = "tok"
	PrefixAuditLog           Prefix = "aud"
	PrefixOAuthIdentity      Prefix = "idn"
	PrefixSecurityEvent      Prefix = "sev"
	PrefixOneTimeToken       Prefix = "ott"
	PrefixPasswordReset      Prefix = "prs"
	PrefixAuthFactor         Prefix = "fac"
	PrefixOAuthGrant         Prefix = "ogr"
	PrefixOAuthClient        Prefix = "ocl"
	PrefixWorkspaceDomain    Prefix = "dom"
	PrefixSSOConnection      Prefix = "ssoc"
	PrefixOrganizationDomain Prefix = "odom"
	PrefixRole               Prefix = "rol"
	PrefixUserDevice         Prefix = "dev"
)

/*
The prefixes of the shared inbox's tables.
*/
const (
	PrefixInbox             Prefix = "inbox"
	PrefixChannelConnection Prefix = "chn"
	PrefixContact           Prefix = "ctc"
	PrefixCompany           Prefix = "cmp"
	PrefixConversation      Prefix = "conv"
	PrefixMessage           Prefix = "msg"
	PrefixAttachment        Prefix = "att"
	PrefixTag               Prefix = "tag"
	PrefixSavedReply        Prefix = "rpl"
	PrefixConversationView  Prefix = "cvw"
)

/*
The prefixes of the email channel's tables.
*/
const (
	PrefixEmailDomain      Prefix = "edm"
	PrefixEmailAddress     Prefix = "ema"
	PrefixInboundEmail     Prefix = "ibm"
	PrefixOutboundEmail    Prefix = "obm"
	PrefixEmailSuppression Prefix = "sup"
	PrefixEmailSignature   Prefix = "sig"
)

/*
PublicIDEntry is one row of the catalog: a prefix and the table it
names. The table of a reserved prefix starts with "reserved: ".
*/
type PublicIDEntry struct {
	Prefix Prefix
	Table  string
}

/*
PublicID returns the public id of a row of the kind.
*/
func PublicID(prefix Prefix, id ids.UUID) string {
	return ids.Encode(string(prefix), id)
}

/*
RegisterPublicIDs adds every prefix of the catalog to the registry, the
reserved ones included, and reports every prefix already taken.
*/
func RegisterPublicIDs(registry *ids.Registry) error {
	var errs []error

	for _, entry := range PublicIDCatalog() {
		errs = append(errs, registry.Register(string(entry.Prefix), entry.Table))
	}

	return errors.Join(errs...)
}

/*
PublicIDCatalog lists every prefix with the table it names, area by
area, then the prefixes reserved for tables that do not exist yet.
*/
func PublicIDCatalog() []PublicIDEntry {
	return slices.Concat(
		accountPublicIDs(), inboxPublicIDs(), channelPublicIDs(), assistantPublicIDs(), helpCenterPublicIDs(),
		platformPublicIDs(), billingPublicIDs(), changelogPublicIDs(), feedbackPublicIDs(), engagePublicIDs(),
		reservedPublicIDs(),
	)
}

/*
accountPublicIDs lists the prefixes of accounts, workspaces and sign-in.
*/
func accountPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{PrefixUser, "users"},
		{PrefixWorkspace, "workspaces"},
		{PrefixOrganization, "organizations"},
		{PrefixMember, "workspace_members"},
		{PrefixTeam, "teams"},
		{PrefixRole, "roles"},
		{PrefixSSOConnection, "sso_connections"},
		{PrefixOrganizationDomain, "organization_domains"},
		{PrefixInvitation, "invitations"},
		{PrefixSession, "sessions"},
		{PrefixPersonalToken, "personal_access_tokens"},
		{PrefixAuditLog, "audit_logs"},
		{PrefixOAuthIdentity, "oauth_identities"},
		{PrefixSecurityEvent, "security_events"},
		{PrefixOneTimeToken, "one_time_tokens"},
		{PrefixPasswordReset, "password_reset_tokens"},
		{PrefixAuthFactor, "user_auth_factors"},
		{PrefixOAuthGrant, "oauth_grants"},
		{PrefixOAuthClient, "oauth_clients"},
		{"oar", "oauth_authorization_requests"},
		{"odr", "oauth_device_codes"},
	}
}

/*
inboxPublicIDs lists the prefixes of the shared inbox: its tables, the
contact exports and the events it records.
*/
func inboxPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{PrefixInbox, "inboxes"},
		{PrefixChannelConnection, "channel_connections"},
		{PrefixContact, "contacts"},
		{PrefixCompany, "companies"},
		{PrefixConversation, "conversations"},
		{PrefixMessage, "messages"},
		{PrefixAttachment, "attachments"},
		{PrefixTag, "tags"},
		{PrefixSavedReply, "saved_replies"},
		{PrefixConversationView, "conversation_views"},
		{"exp", "contact_exports"},
		{"evt", "events"},
	}
}

/*
channelPublicIDs lists the prefixes of the messenger widget, email and
chat app channels, and notifications.
*/
func channelPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{"wgt", "widget_settings"},
		{"wss", "widget_sessions"},
		{"isk", "identity_secrets"},
		{PrefixEmailSignature, "email_signatures"},
		{PrefixEmailDomain, "email_domains"},
		{PrefixEmailAddress, "email_addresses"},
		{PrefixInboundEmail, "inbound_emails"},
		{PrefixOutboundEmail, "outbound_emails"},
		{PrefixEmailSuppression, "email_suppressions"},
		{"chr", "channel_rooms"},
		{"cml", "channel_member_links"},
		{"cie", "channel_inbound_events"},
		{"wtp", "whatsapp_templates"},
		{"ntf", "notifications"},
		{"psub", "push_subscriptions"},
		{"ndst", "notification_destinations"},
		{"ndlv", "notification_deliveries"},
	}
}

/*
assistantPublicIDs lists the prefixes of the AI agent and its actions.
*/
func assistantPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{"aia", "ai_agents"},
		{"aik", "ai_provider_keys"},
		{"air", "ai_runs"},
		{"ais", "ai_run_steps"},
		{"acn", "ai_action_connections"},
		{"aac", "ai_actions"},
		{"aacv", "ai_action_versions"},
		{"aax", "ai_action_calls"},
	}
}

/*
helpCenterPublicIDs lists the prefixes of the help center, its
knowledge sources and the documentation sync.
*/
func helpCenterPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{"hc", "help_centers"},
		{"col", "collections"},
		{"art", "articles"},
		{"artv", "article_versions"},
		{"sug", "article_suggestions"},
		{"sugb", "article_suggestion_batches"},
		{"hcd", "help_center_domains"},
		{"rdr", "help_center_redirects"},
		{"ast", "help_center_assets"},
		{"ks", "knowledge_sources"},
		{"ksr", "knowledge_sync_runs"},
		{"kd", "knowledge_documents"},
		{"kc", "knowledge_chunks"},
		{"ghc", "github_connections"},
		{"ghr", "github_repositories"},
		{"dss", "docs_sync_sources"},
		{"dsr", "docs_sync_runs"},
		{"asr", "article_source_refs"},
	}
}

/*
platformPublicIDs lists the prefixes of webhooks, self-hosting and
importers.
*/
func platformPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{"whk", "webhook_endpoints"},
		{"whd", "webhook_deliveries"},
		{"wha", "webhook_delivery_attempts"},
		{"bak", "backups"},
		{"iex", "data_exports"},
		{"imp", "imports"},
		{"impr", "import_runs"},
		{"impm", "import_mappings"},
		{"impf", "import_files"},
	}
}

/*
billingPublicIDs lists the prefixes of cloud billing.
*/
func billingPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{"ba", "billing_accounts"},
		{"ul", "usage_ledger"},
		{"ivc", "invoices_cache"},
		{"cg", "credit_grants"},
		{"bd", "billing_disputes"},
		{"bpa", "billing_program_applications"},
		{"bri", "billing_reconciliation_issues"},
		{"ent", "entitlement_overrides"},
	}
}

/*
changelogPublicIDs lists the prefixes of the changelog.
*/
func changelogPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{"clg", "changelogs"},
		{"cle", "changelog_entries"},
		{"cll", "changelog_labels"},
		{"csub", "changelog_subscribers"},
		{"clm", "changelog_mailings"},
		{"cls", "changelog_sources"},
		{"cgn", "changelog_generations"},
	}
}

/*
feedbackPublicIDs lists the prefixes of feedback boards and the
roadmap portal.
*/
func feedbackPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{"fbd", "feedback_boards"},
		{"fbct", "feedback_categories"},
		{"fbs", "feedback_statuses"},
		{"fbp", "feedback_posts"},
		{"fbv", "feedback_votes"},
		{"fbc", "feedback_comments"},
		{"fbr", "feedback_requests"},
		{"fbm", "feedback_post_merges"},
		{"fbsb", "feedback_subscriptions"},
		{"fbu", "feedback_post_updates"},
		{"fbn", "feedback_notifications"},
		{"fbsg", "feedback_suggestions"},
		{"fbi", "feedback_imports"},
		{"pts", "portal_sessions"},
		{"psi", "portal_sign_ins"},
	}
}

/*
engagePublicIDs lists the prefixes of announcements and surveys.
*/
func engagePublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{"seg", "segments"},
		{"ann", "announcements"},
		{"svy", "surveys"},
		{"svq", "survey_questions"},
		{"svr", "survey_responses"},
		{"erc", "engage_receipts"},
		{"eas", "engage_assets"},
	}
}

/*
reservedPublicIDs lists the prefixes kept for tables that do not exist
yet, so no other table takes them in the meantime.
*/
func reservedPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{PrefixWorkspaceDomain, "reserved: workspace_domains"},
		{"drf", "reserved: reply_drafts"},
		{"brd", "reserved: brands"},
		{"lgh", "reserved: legal_holds"},
		{"ains", "reserved: app_installations"},
		{PrefixUserDevice, "reserved: user_devices"},
	}
}
