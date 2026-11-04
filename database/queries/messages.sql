-- name: CreateMessage :one
-- CreateMessage appends an entry to a conversation's timeline. A dedupe
-- key the workspace already accepted fails on
-- messages_workspace_id_dedupe_key_key, which is how a retried inbound
-- message is told apart from a new one.
INSERT INTO messages (
    workspace_id, conversation_id, type, author_type, author_id, body, body_format, body_full, body_html, event,
    channel_message_id, dedupe_key, delivery_status, substantive, metadata, sent_at, channel_connection_id,
    created_at, updated_at
) VALUES (
    @workspace_id, @conversation_id, @type, @author_type, @author_id, @body,
    coalesce(sqlc.narg(body_format), 'markdown'), sqlc.narg(body_full), @body_html, @event,
    @channel_message_id, @dedupe_key, @delivery_status, @substantive, @metadata, @sent_at,
    sqlc.narg(channel_connection_id), @now, @now
)
RETURNING *;

-- name: ListMessages :many
-- ListMessages returns a page of a conversation's timeline, oldest
-- first. A page after the first passes the created_at and id of the last
-- message it saw, and gets those that came after it.
SELECT * FROM messages
WHERE workspace_id = @workspace_id
  AND conversation_id = @conversation_id
  AND (
      sqlc.narg(after_id)::uuid IS NULL
      OR (created_at, id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at, id
LIMIT @row_limit;
