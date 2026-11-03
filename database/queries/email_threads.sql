-- name: EnsureEmailThread :one
-- EnsureEmailThread gives the conversation a reply token for the
-- address, keeping the one it has. A token another thread holds fails
-- on email_threads_reply_token_key.
INSERT INTO email_threads (
    workspace_id, conversation_id, email_address_id, reply_token, root_message_id, normalized_subject, created_at,
    updated_at
) VALUES (
    @workspace_id, @conversation_id, @email_address_id, @reply_token, @root_message_id, @normalized_subject, @now::timestamptz,
    @now::timestamptz
)
ON CONFLICT (conversation_id, email_address_id) DO UPDATE SET updated_at = email_threads.updated_at
RETURNING *;

-- name: FindEmailThreadByToken :one
-- FindEmailThreadByToken returns the live thread holding the reply
-- token in the workspace. A token of another workspace is not found.
SELECT t.* FROM email_threads AS t
JOIN conversations AS c ON c.id = t.conversation_id AND c.workspace_id = t.workspace_id
WHERE t.workspace_id = @workspace_id AND t.reply_token = @reply_token AND t.revoked_at IS NULL
  AND c.deleted_at IS NULL;

-- name: FindSubjectThreadCandidates :many
-- FindSubjectThreadCandidates returns the open or snoozed conversations
-- of the contact on the connection whose subject normalizes to the same
-- text and that saw a message since the cutoff.
SELECT DISTINCT c.id FROM conversations AS c
JOIN email_threads AS t ON t.conversation_id = c.id AND t.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND c.channel_connection_id = @channel_connection_id
  AND c.contact_id = @contact_id AND c.status IN ('open', 'snoozed') AND c.deleted_at IS NULL
  AND c.last_message_at >= @since::timestamptz AND t.normalized_subject = @normalized_subject::text
LIMIT 2;
