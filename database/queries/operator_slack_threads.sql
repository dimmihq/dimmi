-- name: FindOperatorSlackThread :one
-- FindOperatorSlackThread returns the Operator thread of a Slack thread.
SELECT * FROM operator_slack_threads
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND slack_channel = @slack_channel AND thread_ts = @thread_ts;

-- name: CreateOperatorSlackThread :one
-- CreateOperatorSlackThread ties a Slack thread to an Operator thread.
INSERT INTO operator_slack_threads (
    workspace_id, operator_thread_id, channel_connection_id, slack_channel, thread_ts, created_at
) VALUES (@workspace_id, @operator_thread_id, @channel_connection_id, @slack_channel, @thread_ts, @now)
RETURNING *;
