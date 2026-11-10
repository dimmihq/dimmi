-- migrate: phase=expand
-- A teammate talks with Operator in Slack, in a thread of a direct
-- message or of an internal room. Each Slack thread is one Operator
-- thread, so a follow-up in the same Slack thread carries on the same
-- conversation with Operator, and a new Slack thread starts a new one.

-- +goose Up
CREATE TABLE operator_slack_threads (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    operator_thread_id uuid NOT NULL REFERENCES operator_threads ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    slack_channel text NOT NULL,
    thread_ts text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX operator_slack_threads_connection_channel_thread_key
    ON operator_slack_threads (channel_connection_id, slack_channel, thread_ts);
CREATE INDEX operator_slack_threads_operator_thread_id_index ON operator_slack_threads (operator_thread_id);

ALTER TABLE operator_slack_threads ENABLE ROW LEVEL SECURITY;
CREATE POLICY operator_slack_threads_tenant ON operator_slack_threads
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE operator_slack_threads;
