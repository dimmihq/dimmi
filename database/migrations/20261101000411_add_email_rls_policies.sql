-- migrate: phase=expand
-- Every email table is tenant data: rows are visible only inside the
-- workspaces the connection may see, like the rest of the inbox.

-- +goose Up
ALTER TABLE email_domains ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_domains_tenant ON email_domains USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE email_addresses ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_addresses_tenant ON email_addresses USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE inbound_emails ENABLE ROW LEVEL SECURITY;
CREATE POLICY inbound_emails_tenant ON inbound_emails USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE email_message_refs ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_message_refs_tenant ON email_message_refs
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE email_threads ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_threads_tenant ON email_threads USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE outbound_emails ENABLE ROW LEVEL SECURITY;
CREATE POLICY outbound_emails_tenant ON outbound_emails USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE email_suppressions ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_suppressions_tenant ON email_suppressions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE email_signatures ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_signatures_tenant ON email_signatures USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE email_sending_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_sending_profiles_tenant ON email_sending_profiles
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE email_sending_stats ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_sending_stats_tenant ON email_sending_stats
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP POLICY email_sending_stats_tenant ON email_sending_stats;
ALTER TABLE email_sending_stats DISABLE ROW LEVEL SECURITY;
DROP POLICY email_sending_profiles_tenant ON email_sending_profiles;
ALTER TABLE email_sending_profiles DISABLE ROW LEVEL SECURITY;
DROP POLICY email_signatures_tenant ON email_signatures;
ALTER TABLE email_signatures DISABLE ROW LEVEL SECURITY;
DROP POLICY email_suppressions_tenant ON email_suppressions;
ALTER TABLE email_suppressions DISABLE ROW LEVEL SECURITY;
DROP POLICY outbound_emails_tenant ON outbound_emails;
ALTER TABLE outbound_emails DISABLE ROW LEVEL SECURITY;
DROP POLICY email_threads_tenant ON email_threads;
ALTER TABLE email_threads DISABLE ROW LEVEL SECURITY;
DROP POLICY email_message_refs_tenant ON email_message_refs;
ALTER TABLE email_message_refs DISABLE ROW LEVEL SECURITY;
DROP POLICY inbound_emails_tenant ON inbound_emails;
ALTER TABLE inbound_emails DISABLE ROW LEVEL SECURITY;
DROP POLICY email_addresses_tenant ON email_addresses;
ALTER TABLE email_addresses DISABLE ROW LEVEL SECURITY;
DROP POLICY email_domains_tenant ON email_domains;
ALTER TABLE email_domains DISABLE ROW LEVEL SECURITY;
