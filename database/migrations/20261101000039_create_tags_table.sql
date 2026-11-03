-- migrate: phase=expand
-- Tags label conversations. A tag's name is unique in its workspace
-- without regard to case, so Billing and billing are one tag. The link
-- from a conversation to a tag records who added it, any kind of actor,
-- and carries its workspace so row level security covers it alone.

-- +goose Up
CREATE TABLE tags (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name citext NOT NULL,
    color text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tags_workspace_id_name_key UNIQUE (workspace_id, name)
);

CREATE TABLE conversation_tag (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES tags ON DELETE CASCADE,
    created_by_type text NOT NULL CHECK (created_by_type ~ '^[a-z][a-z0-9_]{1,39}$'),
    created_by_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, tag_id)
);

CREATE INDEX conversation_tag_tag_id_index ON conversation_tag (tag_id);

ALTER TABLE tags ENABLE ROW LEVEL SECURITY;
CREATE POLICY tags_tenant ON tags USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE conversation_tag ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_tag_tenant ON conversation_tag USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_tag;
DROP TABLE tags;
