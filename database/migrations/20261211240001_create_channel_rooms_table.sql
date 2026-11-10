-- migrate: phase=expand
-- A room is one place on a chat app a connection watches, such as a
-- Slack channel. Its mode decides who counts as the customer there and
-- its trigger when a message becomes a conversation. Thread keys name
-- the room by its own id, never the provider's, because a provider may
-- change a channel's id when it is shared, so the room keeps its
-- conversations through the change.

-- +goose Up
CREATE TABLE channel_rooms (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind ~ '^[a-z][a-z0-9_]{1,39}$'),
    external_id text NOT NULL,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'removed', 'error')),
    mode text NOT NULL CHECK (mode IN ('shared', 'community', 'internal', 'helpdesk')),
    trigger text NOT NULL CHECK (trigger IN ('author_window', 'every_message', 'emoji', 'mention', 'ai', 'auto', 'off')),
    agent_enabled boolean NOT NULL DEFAULT true,
    company_id uuid REFERENCES companies ON DELETE SET NULL,
    settings jsonb NOT NULL DEFAULT '{}',
    is_shared_external boolean NOT NULL DEFAULT false,
    last_error text,
    last_seen_event_at timestamptz,
    removed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX channel_rooms_channel_connection_id_external_id_key
    ON channel_rooms (channel_connection_id, external_id);
CREATE INDEX channel_rooms_workspace_id_channel_connection_id_status_index
    ON channel_rooms (workspace_id, channel_connection_id, status);

ALTER TABLE channel_rooms ENABLE ROW LEVEL SECURITY;
CREATE POLICY channel_rooms_tenant ON channel_rooms USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE channel_rooms;
