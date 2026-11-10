-- migrate: phase=expand
-- Teammates hear about activity in Slack too, through the workspace's one
-- Slack installation: a channel the team picked hears about the triggers
-- it listed and gets its daily and weekly digests, and a teammate who
-- linked their Slack user gets their own alerts as direct messages. A
-- destination never holds a token; it names the installation, and goes to
-- error when the installation is removed. The digest sweep runs before
-- any workspace is set, so it finds what is due through a security
-- definer function that returns only which destination and workspace.

-- +goose Up
CREATE TABLE notification_destinations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    scope text NOT NULL CHECK (scope IN ('workspace', 'member')),
    workspace_member_id uuid REFERENCES workspace_members ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('slack_channel', 'slack_dm')),
    integration_installation_id uuid NOT NULL REFERENCES integration_installations ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    config jsonb NOT NULL DEFAULT '{}',
    triggers text[] NOT NULL DEFAULT '{}',
    inbox_ids uuid[],
    include_ai_handled boolean NOT NULL DEFAULT false,
    show_previews boolean NOT NULL DEFAULT true,
    digests text[] NOT NULL DEFAULT '{}',
    digest_time text NOT NULL DEFAULT '09:00' CHECK (digest_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    next_digest_at timestamptz,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'error', 'disabled')),
    last_error_code text,
    last_error text,
    last_error_at timestamptz,
    last_delivered_at timestamptz,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((scope = 'member') = (workspace_member_id IS NOT NULL)),
    CHECK ((type = 'slack_dm') = (scope = 'member'))
);

CREATE UNIQUE INDEX notification_destinations_member_type_key
    ON notification_destinations (workspace_id, workspace_member_id, type) WHERE scope = 'member';
CREATE INDEX notification_destinations_workspace_id_scope_status_index
    ON notification_destinations (workspace_id, scope, status);
CREATE INDEX notification_destinations_integration_installation_id_index
    ON notification_destinations (integration_installation_id);
CREATE INDEX notification_destinations_next_digest_at_index
    ON notification_destinations (next_digest_at) WHERE next_digest_at IS NOT NULL;

ALTER TABLE notification_destinations ENABLE ROW LEVEL SECURITY;
CREATE POLICY notification_destinations_tenant ON notification_destinations
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE FUNCTION notification_digests_due(due_before timestamptz, max_rows integer)
    RETURNS TABLE (workspace_id uuid, notification_destination_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT d.workspace_id, d.id
        FROM notification_destinations d
        JOIN workspaces w ON w.id = d.workspace_id
        WHERE d.next_digest_at IS NOT NULL AND d.next_digest_at <= due_before AND d.status = 'active'
          AND w.deleted_at IS NULL
        ORDER BY d.next_digest_at, d.id
        LIMIT max_rows
    $$;

-- +goose Down
DROP FUNCTION notification_digests_due(timestamptz, integer);
DROP TABLE notification_destinations;
