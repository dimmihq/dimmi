-- name: OperatorConversationTotals :one
-- OperatorConversationTotals sums the conversations opened in a range:
-- how many, how many were closed, how many are still open, and the
-- median minutes to the first reply and to closing. Test, spam and
-- deleted conversations never count.
SELECT
    count(*)::bigint AS opened,
    count(*) FILTER (WHERE closed_at IS NOT NULL)::bigint AS closed,
    count(*) FILTER (WHERE status <> 'closed')::bigint AS still_open,
    count(*) FILTER (WHERE first_reply_at IS NOT NULL)::bigint AS replied,
    COALESCE(percentile_cont(0.5) WITHIN GROUP (
        ORDER BY extract(epoch FROM first_reply_at - COALESCE(first_customer_message_at, created_at)) / 60
    ) FILTER (WHERE first_reply_at IS NOT NULL), 0)::float8 AS median_first_reply_minutes,
    COALESCE(percentile_cont(0.5) WITHIN GROUP (
        ORDER BY extract(epoch FROM closed_at - created_at) / 60
    ) FILTER (WHERE closed_at IS NOT NULL), 0)::float8 AS median_resolution_minutes
FROM conversations
WHERE workspace_id = @workspace_id AND created_at >= @since AND created_at < @until
  AND deleted_at IS NULL AND NOT spam AND NOT is_test;

-- name: OperatorConversationsByDay :many
-- OperatorConversationsByDay counts the conversations opened and closed
-- on each day of a range, in UTC; days with neither are left out.
SELECT x.day::timestamptz AS day, sum(x.opened)::bigint AS opened, sum(x.closed)::bigint AS closed
FROM (
    SELECT date_trunc('day', created_at) AS day, 1 AS opened, 0 AS closed
    FROM conversations
    WHERE workspace_id = @workspace_id::uuid AND created_at >= @since::timestamptz AND created_at < @until::timestamptz
      AND deleted_at IS NULL AND NOT spam AND NOT is_test
    UNION ALL
    SELECT date_trunc('day', closed_at) AS day, 0 AS opened, 1 AS closed
    FROM conversations
    WHERE workspace_id = @workspace_id::uuid AND closed_at >= @since::timestamptz AND closed_at < @until::timestamptz
      AND deleted_at IS NULL AND NOT spam AND NOT is_test
) x
GROUP BY x.day
ORDER BY x.day;

-- name: OperatorConversationsByChannel :many
-- OperatorConversationsByChannel counts the conversations opened in a
-- range by the kind of channel they came through.
SELECT ch.type AS channel, count(*)::bigint AS opened
FROM conversations c
JOIN channel_connections ch ON ch.id = c.channel_connection_id
WHERE c.workspace_id = @workspace_id AND c.created_at >= @since AND c.created_at < @until
  AND c.deleted_at IS NULL AND NOT c.spam AND NOT c.is_test
GROUP BY ch.type
ORDER BY opened DESC;

-- name: OperatorBusiestHours :many
-- OperatorBusiestHours counts customer conversations opened in each
-- hour of the day, in UTC, over a range.
SELECT extract(hour FROM created_at)::int AS hour, count(*)::bigint AS opened
FROM conversations
WHERE workspace_id = @workspace_id AND created_at >= @since AND created_at < @until
  AND deleted_at IS NULL AND NOT spam AND NOT is_test
GROUP BY 1
ORDER BY 1;

-- name: OperatorTopTags :many
-- OperatorTopTags returns the tags most often put on conversations
-- opened in a range.
SELECT t.name, count(*)::bigint AS conversations
FROM conversation_tag ct
JOIN tags t ON t.id = ct.tag_id
JOIN conversations c ON c.id = ct.conversation_id
WHERE ct.workspace_id = @workspace_id AND c.created_at >= @since AND c.created_at < @until
  AND c.deleted_at IS NULL AND NOT c.spam AND NOT c.is_test
GROUP BY t.name
ORDER BY conversations DESC, t.name
LIMIT @row_limit;

-- name: OperatorAiOutcomes :many
-- OperatorAiOutcomes counts the AI agent's conversation runs in a range
-- by how they ended, with what they cost.
SELECT COALESCE(outcome, status) AS outcome, count(*)::bigint AS runs, COALESCE(sum(cost_micros), 0)::bigint AS cost_micros
FROM ai_runs
WHERE workspace_id = @workspace_id AND kind = 'conversation' AND created_at >= @since AND created_at < @until
GROUP BY 1
ORDER BY runs DESC;

-- name: OperatorHandoffReasons :many
-- OperatorHandoffReasons counts why the AI agent handed conversations
-- to the team in a range.
SELECT handoff_reason AS reason, count(*)::bigint AS runs
FROM ai_runs
WHERE workspace_id = @workspace_id AND kind = 'conversation' AND handoff_reason IS NOT NULL
  AND created_at >= @since AND created_at < @until
GROUP BY handoff_reason
ORDER BY runs DESC;

-- name: OperatorArticleHelpfulness :many
-- OperatorArticleHelpfulness returns the articles readers rated in a
-- range, least helpful first, with their answers and latest comments.
SELECT a.id, a.title,
    count(*) FILTER (WHERE r.helpful)::bigint AS helpful,
    count(*) FILTER (WHERE NOT r.helpful)::bigint AS not_helpful,
    COALESCE((array_agg(r.comment ORDER BY r.updated_at DESC) FILTER (WHERE r.comment IS NOT NULL))[1:3],
        '{}')::text[] AS comments
FROM article_ratings r
JOIN articles a ON a.id = r.article_id
WHERE r.workspace_id = @workspace_id AND r.updated_at >= @since AND r.updated_at < @until
GROUP BY a.id, a.title
ORDER BY (count(*) FILTER (WHERE r.helpful))::float8 / count(*), count(*) DESC
LIMIT @row_limit;
