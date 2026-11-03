-- migrate: phase=expand
-- A message is one entry in a conversation's timeline: a customer's
-- message, a reply, an internal note or a system event. Notes never
-- leave the workspace. The author id points at a contact, a member or an
-- agent depending on the author type, so it has no foreign key. A
-- channel retrying the same inbound message carries the same dedupe key,
-- which the workspace accepts once. A deleted message keeps its row with
-- the body cleared, so the timeline keeps its shape.

-- +goose Up
CREATE TABLE messages (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('message', 'note', 'event')),
    author_type text NOT NULL CHECK (author_type IN ('contact', 'member', 'agent', 'system', 'integration')),
    author_id uuid,
    body text,
    body_html text,
    event jsonb,
    channel_message_id text,
    dedupe_key text,
    delivery_status text
        CHECK (delivery_status IN ('pending', 'sent', 'delivered', 'read', 'failed', 'not_applicable')),
    delivery_error text,
    substantive boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}',
    edited_at timestamptz,
    sent_at timestamptz,
    read_at timestamptz,
    deleted_at timestamptz,
    search tsvector GENERATED ALWAYS AS (to_tsvector('english', coalesce(body, ''))) STORED,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX messages_workspace_id_dedupe_key_key ON messages (workspace_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL;
CREATE INDEX messages_workspace_id_conversation_id_created_at_index
    ON messages (workspace_id, conversation_id, created_at);
CREATE INDEX messages_search_index ON messages USING gin (search);

ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
CREATE POLICY messages_tenant ON messages USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE messages;
