-- migrate: phase=expand
-- A read marker says how far one teammate has read one conversation,
-- which is all unread state is. A message removed from under a marker
-- leaves the time in place, so the conversation does not turn unread.

-- +goose Up
CREATE TABLE conversation_reads (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    workspace_member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    last_read_message_id uuid REFERENCES messages ON DELETE SET NULL,
    last_read_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, workspace_member_id)
);

ALTER TABLE conversation_reads ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_reads_tenant ON conversation_reads
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_reads;
