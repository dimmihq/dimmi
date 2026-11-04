-- name: ListTimeline :many
-- ListTimeline returns a page of a conversation's timeline narrowed to
-- the entry types asked for, none meaning all. Descending pages walk
-- back from the newest entry, or from the cursor's entry; ascending pages
-- walk forward from the cursor's entry.
SELECT * FROM messages
WHERE workspace_id = @workspace_id
  AND conversation_id = @conversation_id
  AND (coalesce(cardinality(@types::text[]), 0) = 0 OR type = ANY (@types::text[]))
  AND (
      sqlc.narg(cursor_id)::uuid IS NULL
      OR (@descending::boolean AND (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
      OR (NOT @descending::boolean AND (created_at, id) > (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
  )
ORDER BY
    CASE WHEN @descending::boolean THEN created_at END DESC,
    CASE WHEN @descending::boolean THEN id END DESC,
    CASE WHEN NOT @descending::boolean THEN created_at END,
    CASE WHEN NOT @descending::boolean THEN id END
LIMIT @row_limit::bigint;

-- name: FindMessage :one
-- FindMessage returns the entry with the id in the conversation.
SELECT * FROM messages WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id = @id;

-- name: FindMessageByDedupeKey :one
-- FindMessageByDedupeKey returns the entry the workspace accepted under
-- the dedupe key.
SELECT * FROM messages WHERE workspace_id = @workspace_id AND dedupe_key = @dedupe_key;

-- name: LatestTimelineEntry :one
-- LatestTimelineEntry returns the conversation's newest entry of any
-- type, the furthest a reader can read to.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id
ORDER BY id DESC
LIMIT 1;

-- name: LatestConversationMessage :one
-- LatestConversationMessage returns the conversation's newest live
-- message or note, the entry a teammate must have seen before replying.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type <> 'event' AND deleted_at IS NULL
ORDER BY id DESC
LIMIT 1;

-- name: MessagesAfter :many
-- MessagesAfter returns the conversation's live messages and notes
-- written after the entry, oldest first: what arrived while a teammate
-- was typing.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type <> 'event'
  AND deleted_at IS NULL AND id > @after_id
ORDER BY id
LIMIT 20;

-- name: LatestForeignMessage :one
-- LatestForeignMessage returns the newest live message or note in the
-- conversation that the member did not write.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type <> 'event'
  AND deleted_at IS NULL AND NOT (author_type = 'member' AND author_id IS NOT DISTINCT FROM @member_id::uuid)
ORDER BY id DESC
LIMIT 1;

-- name: TimelineEntryBefore :one
-- TimelineEntryBefore returns the entry of any type just before the one
-- with the id.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id < @before_id
ORDER BY id DESC
LIMIT 1;

-- name: UpdateMessageDelivery :one
-- UpdateMessageDelivery records how delivering the message on its
-- channel went, the channel's own id for it once it has one, and what
-- the channel reported, merged into its metadata.
UPDATE messages
SET delivery_status = @delivery_status, delivery_error = sqlc.narg(delivery_error),
    channel_message_id = coalesce(sqlc.narg(channel_message_id), channel_message_id),
    sent_at = coalesce(sqlc.narg(sent_at), sent_at), updated_at = @now,
    metadata = metadata || coalesce(sqlc.narg(metadata)::jsonb, '{}'::jsonb)
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FindConversationRead :one
-- FindConversationRead returns how far the member has read the
-- conversation.
SELECT * FROM conversation_reads
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND workspace_member_id = @member_id;

-- name: SaveConversationRead :one
-- SaveConversationRead moves the member's read marker on the
-- conversation, creating it the first time.
INSERT INTO conversation_reads (
    workspace_id, conversation_id, workspace_member_id, last_read_message_id, last_read_at, created_at, updated_at
) VALUES (
    @workspace_id, @conversation_id, @member_id, sqlc.narg(last_read_message_id), @last_read_at, @now, @now
)
ON CONFLICT (conversation_id, workspace_member_id) DO UPDATE
SET last_read_message_id = excluded.last_read_message_id, last_read_at = excluded.last_read_at,
    updated_at = excluded.updated_at
RETURNING *;

-- name: RetryMessageDelivery :one
-- RetryMessageDelivery puts a reply that failed back in line to be sent,
-- forgetting why it failed.
UPDATE messages SET delivery_status = 'pending', delivery_error = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id = @id
  AND type = 'message' AND delivery_status = 'failed' AND deleted_at IS NULL
RETURNING *;
