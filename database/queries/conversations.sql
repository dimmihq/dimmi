-- name: AllocateConversationNumber :one
-- AllocateConversationNumber hands out the workspace's next conversation
-- number. The row stays locked until the transaction ends, so two
-- conversations started at once never share a number.
UPDATE workspaces SET next_conversation_number = next_conversation_number + 1
WHERE id = @workspace_id
RETURNING (next_conversation_number - 1)::bigint AS number;

-- name: CreateConversation :one
-- CreateConversation starts a conversation with a contact on a channel,
-- with a number from AllocateConversationNumber.
INSERT INTO conversations (
    workspace_id, number, inbox_id, channel_connection_id, contact_id, subject, status, external_thread_key,
    source, created_at, updated_at
) VALUES (
    @workspace_id, @number, @inbox_id, @channel_connection_id, @contact_id, @subject, @status,
    @external_thread_key, @source, @now, @now
)
RETURNING *;

-- name: SetConversationThreadKey :exec
-- SetConversationThreadKey records the thread the channel opened for the
-- conversation, unless it already has one.
UPDATE conversations SET external_thread_key = @external_thread_key, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND external_thread_key IS NULL;

-- name: FindConversation :one
-- FindConversation returns the conversation with the id in the
-- workspace, deleted or not.
SELECT * FROM conversations WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListConversations :many
-- ListConversations returns a page of the workspace's conversations that
-- are not deleted, newest first. A page after the first passes the
-- created_at and id of the last conversation it saw, and gets those
-- created before it.
SELECT * FROM conversations
WHERE workspace_id = @workspace_id
  AND deleted_at IS NULL
  AND (
      sqlc.narg(before_id)::uuid IS NULL
      OR (created_at, id) < (sqlc.narg(before_created_at)::timestamptz, sqlc.narg(before_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;
