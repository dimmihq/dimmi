-- migrate: phase=expand
-- One message can be several messages on a chat app: a long reply split
-- in parts, a file shared beside the text. Every provider id points back
-- to its message here, so an edit, a deletion or a receipt for any part
-- finds the message, and a retried send finds the part it already sent.

-- +goose Up
CREATE TABLE channel_message_refs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    message_id uuid NOT NULL REFERENCES messages ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    external_message_id text NOT NULL,
    part_index smallint NOT NULL DEFAULT 0,
    kind text NOT NULL CHECK (kind IN ('text', 'file', 'media_group', 'template', 'triage')),
    status text NOT NULL DEFAULT 'sent' CHECK (status IN ('sent', 'delivered', 'read', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX channel_message_refs_channel_connection_id_external_message_id_key
    ON channel_message_refs (channel_connection_id, external_message_id);
CREATE INDEX channel_message_refs_workspace_id_message_id_index ON channel_message_refs (workspace_id, message_id);

ALTER TABLE channel_message_refs ENABLE ROW LEVEL SECURITY;
CREATE POLICY channel_message_refs_tenant ON channel_message_refs
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE channel_message_refs;
