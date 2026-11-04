-- migrate: phase=expand
-- Who a note mentions, one row per member, a team expanded to its members
-- when the note is written. The Mentions view and notifications read this
-- table, never the note's text. Rows go with the note.

-- +goose Up
CREATE TABLE message_mentions (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    message_id uuid NOT NULL REFERENCES messages ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    workspace_member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    via_team_id uuid REFERENCES teams ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, workspace_member_id)
);

CREATE INDEX message_mentions_workspace_id_member_created_at_index
    ON message_mentions (workspace_id, workspace_member_id, created_at DESC, conversation_id);

ALTER TABLE message_mentions ENABLE ROW LEVEL SECURITY;
CREATE POLICY message_mentions_tenant ON message_mentions USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE message_mentions;
