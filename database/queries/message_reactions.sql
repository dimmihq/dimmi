-- name: AddMessageReaction :execrows
-- AddMessageReaction records the member's emoji on the note and reports
-- whether it is new; reacting twice with the same emoji changes nothing.
INSERT INTO message_reactions (workspace_id, message_id, member_id, emoji, created_at)
VALUES (@workspace_id, @message_id, @member_id, @emoji, @now)
ON CONFLICT (message_id, member_id, emoji) DO NOTHING;

-- name: RemoveMessageReaction :execrows
-- RemoveMessageReaction takes the member's emoji off the note and reports
-- whether there was one.
DELETE FROM message_reactions
WHERE workspace_id = @workspace_id AND message_id = @message_id AND member_id = @member_id AND emoji = @emoji;

-- name: HasMessageReactionEmoji :one
-- HasMessageReactionEmoji reports whether anyone already reacted to the
-- note with the emoji, and how many distinct emoji the note has.
SELECT
    EXISTS (
        SELECT 1 FROM message_reactions r
        WHERE r.workspace_id = @workspace_id AND r.message_id = @message_id AND r.emoji = @emoji
    )::boolean AS present,
    (
        SELECT count(DISTINCT r.emoji) FROM message_reactions r
        WHERE r.workspace_id = @workspace_id AND r.message_id = @message_id
    )::bigint AS distinct_emoji;

-- name: SummarizeMessageReactions :one
-- SummarizeMessageReactions writes the note's reactions into its metadata,
-- one entry per emoji in the order first used, with who reacted, so the
-- note shows them without another read.
UPDATE messages m
SET metadata = jsonb_set(m.metadata, '{reactions}', coalesce((
        SELECT jsonb_agg(jsonb_build_object('emoji', g.emoji, 'member_ids', g.member_ids) ORDER BY g.first)
        FROM (
            SELECT r.emoji, min(r.created_at) AS first, jsonb_agg(r.member_id ORDER BY r.created_at) AS member_ids
            FROM message_reactions r
            WHERE r.workspace_id = m.workspace_id AND r.message_id = m.id
            GROUP BY r.emoji
        ) g
    ), '[]'::jsonb)),
    updated_at = @now::timestamptz
WHERE m.workspace_id = @workspace_id AND m.id = @id
RETURNING *;
