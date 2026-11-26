-- name: ListAIConversationAttributes :many
-- ListAIConversationAttributes returns the workspace's AI conversation
-- attributes, active first, then by name.
SELECT * FROM ai_conversation_attributes
WHERE workspace_id = @workspace_id
ORDER BY status = 'archived', position, label, key;

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
    workspace_id, key, label, description, data_type, options, option_descriptions, filled_by, required_to_close,
    team_ids, position, created_at, updated_at
) VALUES (
    @workspace_id, @key, @label, @description, @data_type, @options, @option_descriptions, @filled_by,
    @required_to_close, @team_ids,
    (SELECT coalesce(max(position) + 1, 0) FROM ai_conversation_attributes WHERE workspace_id = @workspace_id),
    @now, @now
)
ON CONFLICT (workspace_id, key) DO NOTHING
RETURNING *;

-- name: UpdateAIConversationAttribute :one
-- UpdateAIConversationAttribute changes an attribute's words, its
-- choices and its status.
UPDATE ai_conversation_attributes
SET label = @label, description = @description, options = @options, option_descriptions = @option_descriptions,
    status = @status,
    filled_by = @filled_by, required_to_close = @required_to_close, team_ids = @team_ids, updated_at = @now
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

-- name: SaveConversationAttributeValues :exec
-- SaveConversationAttributeValues keeps the values of a conversation, who
-- set each and the files they hold, leaving when the AI last read it as
-- it was.
INSERT INTO conversation_ai_values (
    conversation_id, workspace_id, "values", sources, attachment_ids, created_at, updated_at
) VALUES (
    @conversation_id, @workspace_id, @values, @sources, @attachment_ids, @now, @now
)
ON CONFLICT (conversation_id) DO UPDATE
SET "values" = EXCLUDED."values", sources = EXCLUDED.sources, attachment_ids = EXCLUDED.attachment_ids,
    updated_at = EXCLUDED.updated_at;

-- name: SaveDetectedConversationAttributeValues :exec
-- SaveDetectedConversationAttributeValues keeps the values the AI found,
-- who set each, and the message they were checked at.
INSERT INTO conversation_ai_values (
    conversation_id, workspace_id, "values", sources, checked_message_id, checked_at, created_at, updated_at
) VALUES (
    @conversation_id, @workspace_id, @values, @sources, @checked_message_id, @now, @now, @now
)
ON CONFLICT (conversation_id) DO UPDATE
SET "values" = EXCLUDED."values", sources = EXCLUDED.sources, checked_message_id = EXCLUDED.checked_message_id,
    checked_at = EXCLUDED.checked_at, updated_at = EXCLUDED.updated_at;

-- name: ListRequiredToCloseConversationAttributes :many
-- ListRequiredToCloseConversationAttributes returns the active attributes
-- a teammate must fill before closing a conversation.
SELECT * FROM ai_conversation_attributes
WHERE workspace_id = @workspace_id AND status = 'active' AND required_to_close
ORDER BY position, label, key;

-- name: MoveConversationAttribute :exec
-- MoveConversationAttribute puts an attribute at a place in the list.
UPDATE ai_conversation_attributes SET position = @position, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;
