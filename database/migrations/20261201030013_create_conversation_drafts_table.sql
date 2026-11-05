-- migrate: phase=expand
-- A draft is the answer a member is still writing: a reply or a note on
-- one conversation, or the first message of a new one. It is the
-- member's own, kept on the server so it follows them to another device,
-- and the newest write wins. Files staged in a draft are never purged as
-- orphans while the draft lives.

-- +goose Up
CREATE TABLE conversation_drafts (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    conversation_id uuid REFERENCES conversations ON DELETE CASCADE,
    mode text NOT NULL CHECK (mode IN ('reply', 'note', 'new')),
    body text NOT NULL DEFAULT '' CHECK (octet_length(body) <= 65536),
    attachment_ids uuid[] NOT NULL DEFAULT '{}' CHECK (cardinality(attachment_ids) <= 10),
    recipients jsonb NOT NULL DEFAULT '{}',
    client_updated_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT conversation_drafts_target_check CHECK ((mode = 'new') = (conversation_id IS NULL))
);

CREATE UNIQUE INDEX conversation_drafts_member_id_conversation_id_mode_key
    ON conversation_drafts (member_id, conversation_id, mode) NULLS NOT DISTINCT;
CREATE INDEX conversation_drafts_workspace_id_member_id_updated_at_index
    ON conversation_drafts (workspace_id, member_id, updated_at DESC);
CREATE INDEX conversation_drafts_conversation_id_index ON conversation_drafts (conversation_id);
CREATE INDEX conversation_drafts_updated_at_index ON conversation_drafts (updated_at);
CREATE INDEX conversation_drafts_attachment_ids_index ON conversation_drafts USING gin (attachment_ids);

ALTER TABLE conversation_drafts ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_drafts_tenant ON conversation_drafts
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_drafts;
