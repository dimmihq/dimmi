-- migrate: phase=expand
-- The engineering work behind a ticket: Linear, Jira or GitHub issues,
-- and incident.io, Rootly or PagerDuty incidents. A ticket may link many.
-- Each link keeps the issue's key, title and status as last seen, the
-- status group dimmi maps it to, when it was last checked and any
-- warning, such as a Linear team the app cannot see. Provider webhooks
-- find links by the issue's external id, and a sweep re-checks links
-- whose last sync is old, so a missed webhook is caught.

-- +goose Up
CREATE TABLE ticket_issues (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    ticket_id uuid NOT NULL REFERENCES tickets ON DELETE CASCADE,
    installation_id uuid REFERENCES integration_installations ON DELETE SET NULL,
    provider text NOT NULL CHECK (provider IN ('linear', 'jira', 'github', 'incident_io', 'rootly', 'pagerduty')),
    external_id text NOT NULL CHECK (char_length(external_id) BETWEEN 1 AND 255),
    key text NOT NULL CHECK (char_length(key) BETWEEN 1 AND 100),
    url text NOT NULL CHECK (char_length(url) BETWEEN 1 AND 2048),
    title text NOT NULL DEFAULT '' CHECK (char_length(title) <= 500),
    status text NOT NULL DEFAULT '' CHECK (char_length(status) <= 100),
    status_group text NOT NULL DEFAULT 'unstarted'
        CHECK (status_group IN ('backlog', 'unstarted', 'started', 'done', 'canceled')),
    team_name text NOT NULL DEFAULT '' CHECK (char_length(team_name) <= 200),
    warning text CHECK (char_length(warning) <= 500),
    sync_error text CHECK (char_length(sync_error) <= 2000),
    synced_at timestamptz,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ticket_issues_ticket_id_provider_external_id_key UNIQUE (ticket_id, provider, external_id)
);

CREATE INDEX ticket_issues_provider_external_id_index ON ticket_issues (provider, external_id);
CREATE INDEX ticket_issues_installation_id_index ON ticket_issues (installation_id);
CREATE INDEX ticket_issues_created_by_member_id_index ON ticket_issues (created_by_member_id);

ALTER TABLE ticket_issues ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_issues_tenant ON ticket_issues USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose StatementBegin
CREATE FUNCTION ticket_issues_by_external_id(issue_provider text, issue_external_id text)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.workspace_id, i.id FROM ticket_issues i
        WHERE i.provider = issue_provider AND i.external_id = issue_external_id
    $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION ticket_issues_due_sync(synced_before timestamptz, row_limit integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.workspace_id, i.id FROM ticket_issues i
        WHERE i.installation_id IS NOT NULL AND i.status_group NOT IN ('done', 'canceled')
          AND (i.synced_at IS NULL OR i.synced_at < synced_before)
        ORDER BY i.synced_at NULLS FIRST
        LIMIT row_limit
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION ticket_issues_due_sync(timestamptz, integer);
DROP FUNCTION ticket_issues_by_external_id(text, text);
DROP TABLE ticket_issues;
