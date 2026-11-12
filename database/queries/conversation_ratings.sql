-- name: FindConversationRatingSettings :one
-- FindConversationRatingSettings returns whether and how the workspace
-- asks customers to rate a closed conversation.
SELECT * FROM conversation_rating_settings WHERE workspace_id = @workspace_id;

-- name: SaveConversationRatingSettings :one
-- SaveConversationRatingSettings keeps the workspace's rating settings,
-- creating them the first time.
INSERT INTO conversation_rating_settings (
    workspace_id, enabled, delay_minutes, include_agent_closed, messenger, email, created_at, updated_at
)
VALUES (@workspace_id, @enabled, @delay_minutes, @include_agent_closed, @messenger, @email, @now, @now)
ON CONFLICT (workspace_id) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    delay_minutes = EXCLUDED.delay_minutes,
    include_agent_closed = EXCLUDED.include_agent_closed,
    messenger = EXCLUDED.messenger,
    email = EXCLUDED.email,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: CreateConversationRating :one
-- CreateConversationRating opens the ask of one close. A close asks
-- once: asking again for the same close returns nothing.
INSERT INTO conversation_ratings (
    workspace_id, conversation_id, contact_id, member_id, channel, closed_at, ask_at, created_at, updated_at
)
VALUES (
    @workspace_id, @conversation_id, @contact_id, sqlc.narg('member_id'), @channel, @closed_at, @ask_at, @now, @now
)
ON CONFLICT (conversation_id, closed_at) DO NOTHING
RETURNING *;

-- name: FindConversationRating :one
-- FindConversationRating returns one ask in the workspace.
SELECT * FROM conversation_ratings WHERE workspace_id = @workspace_id AND id = @id;

-- name: MarkConversationRatingAsked :one
-- MarkConversationRatingAsked records that the ask went out, once.
UPDATE conversation_ratings SET asked_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND asked_at IS NULL
RETURNING *;

-- name: FindOpenConversationRating :one
-- FindOpenConversationRating returns the latest ask of a conversation
-- that went out, rated or not.
SELECT * FROM conversation_ratings
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND asked_at IS NOT NULL
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: RateConversation :one
-- RateConversation keeps the customer's score and comment on an ask
-- that went out after since, so a rating can change only for a while.
-- A score left out keeps the one given before.
UPDATE conversation_ratings SET
    score = COALESCE(sqlc.narg('score')::smallint, score),
    comment = CASE WHEN @set_comment::boolean THEN sqlc.narg('comment')::text ELSE comment END,
    rated_at = COALESCE(rated_at, @now),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND asked_at IS NOT NULL AND asked_at >= @since
    AND (sqlc.narg('score')::smallint IS NOT NULL OR score IS NOT NULL)
RETURNING *;

-- name: ListConversationRatings :many
-- ListConversationRatings returns a conversation's rated asks, newest
-- first, with the name of the teammate rated.
SELECT r.*, u.name AS member_name
FROM conversation_ratings r
LEFT JOIN workspace_members m ON m.workspace_id = r.workspace_id AND m.id = r.member_id
LEFT JOIN users u ON u.id = m.user_id
WHERE r.workspace_id = @workspace_id AND r.conversation_id = @conversation_id AND r.score IS NOT NULL
ORDER BY r.created_at DESC, r.id DESC
LIMIT 20;
