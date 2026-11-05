-- name: CreateMessage :one
-- CreateMessage appends an entry to a conversation's timeline. A dedupe
-- key the workspace already accepted fails on
-- messages_workspace_id_dedupe_key_key, which is how a retried inbound
-- message is told apart from a new one.
INSERT INTO messages (
    workspace_id, conversation_id, type, author_type, author_id, body, body_html, event, channel_message_id,
    dedupe_key, delivery_status, substantive, metadata, sent_at, channel_connection_id, created_at, updated_at
) VALUES (
    @workspace_id, @conversation_id, @type, @author_type, @author_id, @body, @body_html, @event,
    @channel_message_id, @dedupe_key, @delivery_status, @substantive, @metadata, @sent_at,
    sqlc.narg(channel_connection_id), @now, @now
)
RETURNING *;

-- name: ListMessages :many
-- ListMessages returns a page of a conversation's timeline, oldest
-- first. A page after the first passes the created_at and id of the last
-- message it saw, and gets those that came after it. Undone replies are
-- left out.
SELECT * FROM messages
WHERE workspace_id = @workspace_id
  AND conversation_id = @conversation_id
  AND undone_at IS NULL
  AND (
      sqlc.narg(after_id)::uuid IS NULL
      OR (created_at, id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at, id
LIMIT @row_limit;

-- name: ScheduleMessage :one
-- ScheduleMessage holds a reply until the moment, keeping what undoing it
-- puts back in its metadata.
UPDATE messages
SET scheduled_send_at = @scheduled_send_at, metadata = metadata || @metadata::jsonb, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: LockMessage :one
-- LockMessage returns the entry with the id in the conversation and holds
-- its row until the transaction ends.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id = @id
FOR UPDATE;

-- name: ReleaseScheduledMessage :exec
-- ReleaseScheduledMessage ends a reply's wait as it starts to leave, so
-- it can no longer be undone.
UPDATE messages SET scheduled_send_at = NULL, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: UndoMessage :one
-- UndoMessage takes back a reply still waiting to leave: its text is
-- emptied, it is marked failed for good and it is shown nowhere again.
UPDATE messages
SET undone_at = @now::timestamptz, body = '', body_html = NULL, scheduled_send_at = NULL, delivery_status = 'failed',
    delivery_error = 'canceled', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: UncountConversationMessage :one
-- UncountConversationMessage takes an undone reply out of the
-- conversation's counts and puts back what the reply changed, the latest
-- live message becoming the last again.
UPDATE conversations AS c
SET message_count = greatest(c.message_count - 1, 0),
    last_message_id = latest.id,
    last_message_at = latest.created_at,
    waiting_since = @waiting_since,
    first_reply_at = @first_reply_at,
    last_reply_at = @last_reply_at,
    updated_at = @now
FROM (
    SELECT (
        SELECT m.id FROM messages m
        WHERE m.workspace_id = @workspace_id AND m.conversation_id = @id::uuid AND m.type = 'message'
          AND m.undone_at IS NULL
        ORDER BY m.created_at DESC, m.id DESC LIMIT 1
    ) AS id, (
        SELECT m.created_at FROM messages m
        WHERE m.workspace_id = @workspace_id AND m.conversation_id = @id::uuid AND m.type = 'message'
          AND m.undone_at IS NULL
        ORDER BY m.created_at DESC, m.id DESC LIMIT 1
    ) AS created_at
) AS latest
WHERE c.workspace_id = @workspace_id AND c.id = @id::uuid
RETURNING c.*;

-- name: RestoreConversationState :one
-- RestoreConversationState puts back the status, snooze and holder an
-- undone reply changed.
UPDATE conversations
SET status = @status::text,
    snoozed_until = CASE WHEN @status::text = 'snoozed' THEN sqlc.narg('snoozed_until')::timestamptz END,
    closed_at = CASE WHEN @status::text = 'closed' THEN coalesce(closed_at, @now::timestamptz) END,
    assignee_member_id = sqlc.narg('assignee_member_id')::uuid, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteMessage :one
-- DeleteMessage empties an entry for good, keeping who removed it in the
-- metadata and dropping the copy of the raw email. A reply still waiting
-- to leave never will.
UPDATE messages
SET deleted_at = @now::timestamptz, body = NULL, body_html = NULL, scheduled_send_at = NULL,
    metadata = (metadata - 'email') || @metadata::jsonb,
    delivery_status = CASE WHEN delivery_status = 'pending' THEN 'failed' ELSE delivery_status END,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: EditMessage :one
-- EditMessage replaces a note's text, keeping who edited it.
UPDATE messages
SET body = @body, edited_at = @now::timestamptz, metadata = metadata || @metadata::jsonb,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: UncountDeletedMessage :one
-- UncountDeletedMessage takes a deleted message out of the conversation's
-- count, the latest live message becoming the last again.
UPDATE conversations AS c
SET message_count = greatest(c.message_count - 1, 0),
    last_message_id = (
        SELECT m.id FROM messages m
        WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id AND m.type = 'message'
          AND m.undone_at IS NULL AND m.deleted_at IS NULL
        ORDER BY m.created_at DESC, m.id DESC LIMIT 1
    ),
    last_message_at = (
        SELECT m.created_at FROM messages m
        WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id AND m.type = 'message'
          AND m.undone_at IS NULL AND m.deleted_at IS NULL
        ORDER BY m.created_at DESC, m.id DESC LIMIT 1
    ),
    updated_at = @now::timestamptz
WHERE c.workspace_id = @workspace_id AND c.id = @id
RETURNING c.*;
