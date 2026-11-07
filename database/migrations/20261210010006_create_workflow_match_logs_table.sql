-- migrate: phase=expand
-- Answers "why did this workflow run, and why not that one": one row per
-- event a conversation had live workflows for, with each workflow's
-- verdict and the reason it was skipped. Rows are short-lived; they only
-- exist while someone may still ask.

-- +goose Up
CREATE TABLE workflow_match_logs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    trigger text NOT NULL,
    event_id text,
    verdicts jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workflow_match_logs_workspace_id_conversation_id_created_at_index
    ON workflow_match_logs (workspace_id, conversation_id, created_at DESC);
CREATE INDEX workflow_match_logs_created_at_index ON workflow_match_logs (created_at);

ALTER TABLE workflow_match_logs ENABLE ROW LEVEL SECURITY;
CREATE POLICY workflow_match_logs_tenant ON workflow_match_logs
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE workflow_match_logs;
