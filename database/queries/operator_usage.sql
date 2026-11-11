-- name: OperatorUsageTotals :one
-- OperatorUsageTotals sums Operator's runs started in a range: how many,
-- how many failed, the tokens they read and wrote and what they cost.
SELECT
    count(*)::bigint AS runs_count,
    count(*) FILTER (WHERE status = 'failed')::bigint AS failed_count,
    COALESCE(sum(input_tokens), 0)::bigint AS input_tokens,
    COALESCE(sum(output_tokens), 0)::bigint AS output_tokens,
    COALESCE(sum(cost_micros), 0)::bigint AS cost_micros
FROM ai_runs
WHERE workspace_id = @workspace_id AND kind = 'task' AND created_at >= @since AND created_at < @until;

-- name: OperatorUsageByDay :many
-- OperatorUsageByDay returns Operator's cost and runs for each day of a
-- range with runs, in UTC.
SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day,
    COALESCE(sum(cost_micros), 0)::bigint AS cost_micros,
    count(*)::bigint AS runs_count
FROM ai_runs
WHERE workspace_id = @workspace_id AND kind = 'task' AND created_at >= @since AND created_at < @until
GROUP BY 1 ORDER BY 1;

-- name: OperatorUsageByPurpose :many
-- OperatorUsageByPurpose splits Operator's runs in a range by why they
-- ran: a teammate's question, a brief, or a triage on arrival. Runs from
-- before the purpose was recorded count as questions.
SELECT COALESCE(config_snapshot->>'purpose', 'ask')::text AS purpose,
    count(*)::bigint AS runs_count,
    COALESCE(sum(cost_micros), 0)::bigint AS cost_micros
FROM ai_runs
WHERE workspace_id = @workspace_id AND kind = 'task' AND created_at >= @since AND created_at < @until
GROUP BY 1 ORDER BY 3 DESC, 1;

-- name: OperatorUsageByMember :many
-- OperatorUsageByMember splits Operator's runs in a range by the
-- teammate they ran for, most costly first.
SELECT m.id AS member_id, u.name, count(*)::bigint AS runs_count,
    COALESCE(sum(r.cost_micros), 0)::bigint AS cost_micros
FROM ai_runs r
JOIN workspace_members m ON m.id = r.actor_id AND m.workspace_id = r.workspace_id
JOIN users u ON u.id = m.user_id
WHERE r.workspace_id = @workspace_id AND r.kind = 'task' AND r.actor_type = 'member'
  AND r.created_at >= @since AND r.created_at < @until
GROUP BY m.id, u.name ORDER BY 4 DESC, 3 DESC
LIMIT 20;

-- name: OperatorUsageByModel :many
-- OperatorUsageByModel returns what each model cost Operator in a range;
-- tool and action calls are not models and are left out.
SELECT COALESCE(s.model, '')::text AS model,
    COALESCE(sum(s.cost_micros), 0)::bigint AS cost_micros,
    count(*)::bigint AS calls
FROM ai_run_steps s
JOIN ai_runs r ON r.id = s.run_id AND r.workspace_id = s.workspace_id
WHERE s.workspace_id = @workspace_id AND r.kind = 'task' AND r.created_at >= @since AND r.created_at < @until
  AND s.model IS NOT NULL AND s.kind <> 'tool_call'
GROUP BY s.model ORDER BY 2 DESC, 1;

-- name: OperatorChangesetsByStatus :many
-- OperatorChangesetsByStatus counts the changes Operator proposed in a
-- range by what became of them.
SELECT status, count(*)::bigint AS changesets_count
FROM operator_changesets
WHERE workspace_id = @workspace_id AND created_at >= @since AND created_at < @until
GROUP BY status ORDER BY status;

-- name: ListRecentOperatorRuns :many
-- ListRecentOperatorRuns returns Operator's latest runs in a range,
-- newest first, with why each ran, who it ran for, the thread it
-- answered on and the conversation it was about. A run about a
-- conversation the viewer may not see still counts, but names none.
SELECT r.id, r.status, r.cost_micros, r.latency_ms, r.created_at, c.id AS conversation_id,
    COALESCE(r.config_snapshot->>'purpose', 'ask')::text AS purpose,
    COALESCE(u.name, '')::text AS member_name,
    COALESCE(t.title, '')::text AS thread_title,
    c.number AS conversation_number
FROM ai_runs r
LEFT JOIN workspace_members m ON m.id = r.actor_id AND m.workspace_id = r.workspace_id AND r.actor_type = 'member'
LEFT JOIN users u ON u.id = m.user_id
LEFT JOIN LATERAL (
    SELECT th.title FROM operator_messages om
    JOIN operator_threads th ON th.id = om.thread_id AND th.workspace_id = om.workspace_id
    WHERE om.workspace_id = r.workspace_id AND om.run_id = r.id
    LIMIT 1
) t ON true
LEFT JOIN conversations c ON c.id = r.conversation_id AND c.workspace_id = r.workspace_id
    AND c.deleted_at IS NULL AND dimmi_conversation_visible(
        c.assignee_member_id, c.assignee_team_id, c.inbox_id, @access_mode::text,
        sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean,
        @access_hidden_inboxes::uuid[]
    )
WHERE r.workspace_id = @workspace_id AND r.kind = 'task' AND r.created_at >= @since AND r.created_at < @until
ORDER BY r.created_at DESC
LIMIT @row_limit;
