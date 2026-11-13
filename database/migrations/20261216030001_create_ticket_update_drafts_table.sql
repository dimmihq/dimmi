-- migrate: phase=expand
-- What engineering tools hand back to support: a message to everyone a
-- tracker links, drafted when a linked issue is done or an engineer
-- comments starting with @customers. A draft waits for a person to send or discard it, or is
-- sent at once when the workspace chose automatic. One waiting for the
-- fix to reach production stays waiting_deploy until a deployment says
-- so; nothing is dropped, a draft only ends as sent or discarded.
--
-- Linked issues also keep the provider's team id, when the issue was
-- seen done, when its fix was seen deployed, its severity and a
-- fingerprint of the impact last written back to it, so the impact is
-- only written again when it changed.
--
-- A provider delivers to an installation's own address before any
-- workspace is known, so a security definer function names the
-- workspace and provider of an installation, and nothing else.

-- +goose Up
CREATE TABLE ticket_update_drafts (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    ticket_id uuid NOT NULL REFERENCES tickets ON DELETE CASCADE,
    issue_id uuid REFERENCES ticket_issues ON DELETE SET NULL,
    source text NOT NULL CHECK (source ~ '^[a-z_]{1,40}$'),
    reason text NOT NULL CHECK (reason IN ('issue_done', 'issue_canceled', 'comment')),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 20000),
    author_name text NOT NULL DEFAULT '' CHECK (char_length(author_name) <= 200),
    status text NOT NULL DEFAULT 'ready' CHECK (status IN ('waiting_deploy', 'ready', 'sent', 'discarded')),
    resolve boolean NOT NULL DEFAULT false,
    update_id uuid REFERENCES ticket_updates ON DELETE SET NULL,
    decided_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    decided_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ticket_update_drafts_ticket_id_created_at_index ON ticket_update_drafts (ticket_id, created_at DESC);
CREATE INDEX ticket_update_drafts_workspace_id_status_index ON ticket_update_drafts (workspace_id, status);
CREATE INDEX ticket_update_drafts_issue_id_index ON ticket_update_drafts (issue_id);
CREATE INDEX ticket_update_drafts_update_id_index ON ticket_update_drafts (update_id);
CREATE INDEX ticket_update_drafts_decided_by_member_id_index ON ticket_update_drafts (decided_by_member_id);

ALTER TABLE ticket_update_drafts ENABLE ROW LEVEL SECURITY;
CREATE POLICY ticket_update_drafts_tenant ON ticket_update_drafts
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ticket_issues ADD COLUMN team_id text NOT NULL DEFAULT '' CHECK (char_length(team_id) <= 200);
ALTER TABLE ticket_issues ADD COLUMN severity text NOT NULL DEFAULT '' CHECK (char_length(severity) <= 100);
ALTER TABLE ticket_issues ADD COLUMN done_at timestamptz;
ALTER TABLE ticket_issues ADD COLUMN deployed_at timestamptz;
ALTER TABLE ticket_issues ADD COLUMN impact_hash text NOT NULL DEFAULT '' CHECK (char_length(impact_hash) <= 64);

-- +goose StatementBegin
CREATE FUNCTION integration_installation_owner(installation_id uuid)
    RETURNS TABLE (workspace_id uuid, provider text, status text)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.workspace_id, i.provider, i.status FROM integration_installations i
        WHERE i.id = installation_id AND i.deleted_at IS NULL
    $$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION integration_installation_owner(uuid);
ALTER TABLE ticket_issues DROP COLUMN impact_hash;
ALTER TABLE ticket_issues DROP COLUMN deployed_at;
ALTER TABLE ticket_issues DROP COLUMN done_at;
ALTER TABLE ticket_issues DROP COLUMN severity;
ALTER TABLE ticket_issues DROP COLUMN team_id;
DROP TABLE ticket_update_drafts;
