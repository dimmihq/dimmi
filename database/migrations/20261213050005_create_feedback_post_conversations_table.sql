-- migrate: phase=expand
-- The conversations a feature request came up in. A teammate links a
-- conversation to a request from the inbox, which also votes for the
-- conversation's customer, so the request shows who asked and where,
-- and the customer hears when it ships.

-- +goose Up
CREATE TABLE feedback_post_conversations (
    post_id uuid NOT NULL REFERENCES feedback_posts ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid REFERENCES contacts ON DELETE SET NULL,
    member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, conversation_id)
);

CREATE INDEX feedback_post_conversations_conversation_id_index ON feedback_post_conversations (conversation_id);

ALTER TABLE feedback_post_conversations ENABLE ROW LEVEL SECURITY;
CREATE POLICY feedback_post_conversations_tenant ON feedback_post_conversations
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE feedback_post_conversations;
