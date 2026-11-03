-- migrate: phase=expand
-- A channel connection is one connected source of conversations: a
-- widget install, an email address, a chat workspace, a phone number.
-- Its type is a registry value, so a new channel is new code and never a
-- migration; the database only checks the value's shape. Inbound traffic
-- is routed by the provider's own key, so one live connection per type
-- and key may exist across every workspace, or a message could land in
-- the wrong tenant.

-- +goose Up
CREATE TABLE channel_connections (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    inbox_id uuid NOT NULL REFERENCES inboxes,
    type text NOT NULL CHECK (type ~ '^[a-z][a-z0-9_]{1,39}$'),
    name text NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'error', 'disconnected')),
    external_id text,
    config jsonb NOT NULL DEFAULT '{}',
    credentials_encrypted bytea,
    last_error text,
    last_error_at timestamptz,
    last_activity_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX channel_connections_type_external_id_key ON channel_connections (type, external_id)
    WHERE deleted_at IS NULL AND external_id IS NOT NULL;
CREATE INDEX channel_connections_workspace_id_inbox_id_index ON channel_connections (workspace_id, inbox_id);

ALTER TABLE channel_connections ENABLE ROW LEVEL SECURITY;
CREATE POLICY channel_connections_tenant ON channel_connections
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE channel_connections;
