-- migrate: phase=expand
-- Teammates' emoji reactions to notes, one row per teammate and emoji.
-- The note keeps a summary in its metadata for showing; this table is
-- the record of who reacted with what. Rows go with the note or the
-- teammate.

-- +goose Up
CREATE TABLE message_reactions (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    message_id uuid NOT NULL REFERENCES messages ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    emoji text NOT NULL CHECK (octet_length(emoji) BETWEEN 1 AND 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, member_id, emoji)
);

CREATE INDEX message_reactions_workspace_id_index ON message_reactions (workspace_id);
CREATE INDEX message_reactions_member_id_index ON message_reactions (member_id);

ALTER TABLE message_reactions ENABLE ROW LEVEL SECURITY;
CREATE POLICY message_reactions_tenant ON message_reactions
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE message_reactions;
