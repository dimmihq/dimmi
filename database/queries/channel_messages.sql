-- name: FindChannelMessage :one
-- FindChannelMessage returns the message the connection knows by its
-- provider id.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND channel_message_id = @channel_message_id
ORDER BY created_at
LIMIT 1;

-- name: ReplaceInboundMessageBody :one
-- ReplaceInboundMessageBody follows an edit made at the provider, keeping
-- the earlier text in the metadata it is given.
UPDATE messages
SET body = @body, body_html = NULL, body_full = NULL, edited_at = @now::timestamptz, metadata = @metadata,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: MarkInboundMessageDeleted :one
-- MarkInboundMessageDeleted records that the author deleted the message
-- where they wrote it. The words, files and earlier versions all stay,
-- so the team still sees what was said and that it was taken back.
UPDATE messages
SET metadata = metadata || @metadata::jsonb, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CountConversationVoices :one
-- CountConversationVoices counts the customer's messages they have not
-- deleted and the team's replies, so a conversation whose customer took
-- back everything they said, and was never answered, can close.
SELECT
    count(*) FILTER (
        WHERE author_type = 'contact' AND NOT (metadata ? 'source_deleted')
    )::bigint AS contact_messages,
    count(*) FILTER (WHERE author_type IN ('member', 'agent'))::bigint AS replies
FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type = 'message'
  AND deleted_at IS NULL AND undone_at IS NULL;

-- name: FindWindowConversation :one
-- FindWindowConversation returns the person's newest open conversation
-- of the connection whose thread key starts with the prefix, started
-- since the time and still unanswered, which their next message joins.
SELECT c.* FROM conversations AS c
JOIN contact_identities AS i ON i.contact_id = c.contact_id AND i.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND c.channel_connection_id = @channel_connection_id
  AND i.type = @identity_type AND i.value = @identity_value
  AND c.external_thread_key LIKE @key_prefix::text || '%'
  AND c.created_at >= @since::timestamptz AND c.first_reply_at IS NULL AND c.status <> 'closed'
  AND c.deleted_at IS NULL
ORDER BY c.created_at DESC
LIMIT 1;

-- name: FindThreadConversation :one
-- FindThreadConversation returns the newest live conversation of the
-- connection on the thread.
SELECT * FROM conversations
WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id
  AND external_thread_key = @external_thread_key AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: FindWorkspaceMessage :one
-- FindWorkspaceMessage returns the message with the id in the workspace.
SELECT * FROM messages WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListControlledAnswers :many
-- ListControlledAnswers returns the agent's delivered answers in the
-- conversation that still show their answer controls.
SELECT * FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND author_type = 'agent'
  AND channel_message_id IS NOT NULL AND deleted_at IS NULL
  AND jsonb_array_length(coalesce(metadata -> 'ai' -> 'actions', '[]'::jsonb)) > 0
  AND NOT (metadata ? 'controls_cleared')
ORDER BY created_at;

-- name: MarkControlsCleared :exec
-- MarkControlsCleared records that an answer's controls were taken off.
UPDATE messages SET metadata = metadata || jsonb_build_object('controls_cleared', @now::timestamptz),
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindConversationChannelType :one
-- FindConversationChannelType returns the type of the connection the
-- conversation lives on.
SELECT cc.type FROM conversations AS c
JOIN channel_connections AS cc ON cc.id = c.channel_connection_id AND cc.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND c.id = @id;
