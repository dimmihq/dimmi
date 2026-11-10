-- migrate: phase=expand
-- The team works a conversation in Slack beside the customer's thread:
-- a triage post, discussion threads and the agent's drafts each point
-- back to their conversation, so a click, a reply or an update finds it.
-- Runs keep the drafts the agent wrote for a teammate to send, when
-- feedback on an answer came, and whether a draft agreed with the reply
-- the teammate sent.

-- +goose Up
CREATE TABLE channel_conversation_posts (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('triage', 'discussion', 'draft')),
    external_channel text NOT NULL,
    external_ts text NOT NULL,
    run_id uuid REFERENCES ai_runs ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'sent', 'discarded')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX channel_conversation_posts_channel_connection_id_external_key
    ON channel_conversation_posts (channel_connection_id, external_channel, external_ts);
CREATE UNIQUE INDEX channel_conversation_posts_conversation_id_triage_key
    ON channel_conversation_posts (conversation_id) WHERE kind = 'triage';
CREATE INDEX channel_conversation_posts_workspace_id_conversation_id_index
    ON channel_conversation_posts (workspace_id, conversation_id);
CREATE INDEX channel_conversation_posts_run_id_index ON channel_conversation_posts (run_id);

ALTER TABLE channel_conversation_posts ENABLE ROW LEVEL SECURITY;
CREATE POLICY channel_conversation_posts_tenant ON channel_conversation_posts
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE ai_runs ADD COLUMN draft_body text CHECK (octet_length(draft_body) <= 65536);
ALTER TABLE ai_runs ADD COLUMN feedback_at timestamptz;
ALTER TABLE ai_runs ADD COLUMN shadow_agreed boolean;

-- +goose Down
ALTER TABLE ai_runs DROP COLUMN shadow_agreed;
ALTER TABLE ai_runs DROP COLUMN feedback_at;
ALTER TABLE ai_runs DROP COLUMN draft_body;
DROP TABLE channel_conversation_posts;
