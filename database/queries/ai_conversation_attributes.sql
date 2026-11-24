-- name: ListAIConversationAttributes :many
-- ListAIConversationAttributes returns the workspace's AI conversation
-- attributes, active first, then by name.
SELECT * FROM ai_conversation_attributes
WHERE workspace_id = @workspace_id
ORDER BY status = 'archived', label, key;

-- name: ListActiveAIConversationAttributes :many
-- ListActiveAIConversationAttributes returns the attributes the decision
-- model fills in, by key.
SELECT * FROM ai_conversation_attributes
WHERE workspace_id = @workspace_id AND status = 'active'
ORDER BY key;

-- name: FindAIConversationAttribute :one
-- FindAIConversationAttribute returns one attribute.
SELECT * FROM ai_conversation_attributes WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountActiveAIConversationAttributes :one
-- CountActiveAIConversationAttributes counts the active attributes, which
-- a workspace may have 30 of.
SELECT count(*) FROM ai_conversation_attributes WHERE workspace_id = @workspace_id AND status = 'active';

-- name: CreateAIConversationAttribute :one
-- CreateAIConversationAttribute stores a new attribute, nothing when
-- the key is taken.
INSERT INTO ai_conversation_attributes (
    workspace_id, key, label, description, data_type, options, created_at, updated_at
) VALUES (
    @workspace_id, @key, @label, @description, @data_type, @options, @now, @now
)
ON CONFLICT (workspace_id, key) DO NOTHING
RETURNING *;

-- name: UpdateAIConversationAttribute :one
-- UpdateAIConversationAttribute changes an attribute's words, its
-- choices and its status.
UPDATE ai_conversation_attributes
SET label = @label, description = @description, options = @options, status = @status, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FindConversationAIValues :one
-- FindConversationAIValues returns what was detected on a conversation.
SELECT * FROM conversation_ai_values WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id;

-- name: LockConversationAIValues :one
-- LockConversationAIValues returns what was detected on a conversation,
-- locked while it is changed.
SELECT * FROM conversation_ai_values
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id
FOR UPDATE;

-- name: SaveConversationAIValues :exec
-- SaveConversationAIValues keeps the values detected on a conversation
-- and the message they were checked at.
INSERT INTO conversation_ai_values (
    conversation_id, workspace_id, "values", checked_message_id, checked_at, created_at, updated_at
) VALUES (
    @conversation_id, @workspace_id, @values, @checked_message_id, @now, @now, @now
)
ON CONFLICT (conversation_id) DO UPDATE
SET "values" = EXCLUDED."values", checked_message_id = EXCLUDED.checked_message_id,
    checked_at = EXCLUDED.checked_at, updated_at = EXCLUDED.updated_at;
