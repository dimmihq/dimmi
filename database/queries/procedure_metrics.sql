-- name: ProcedureConversationFigures :one
-- ProcedureConversationFigures counts the conversations a procedure ran
-- in over a range: how many, how many the AI was in, resolved (the same
-- definition as the dimmi agent report), confirmed, handed to the team,
-- rated, rated four or five, and the mean rating. Version zero counts
-- every version.
WITH convs AS (
    SELECT DISTINCT r.conversation_id FROM ai_runs r
    WHERE r.workspace_id = @workspace_id AND r.kind = 'conversation' AND r.conversation_id IS NOT NULL
        AND r.created_at >= @since AND r.created_at < @until
        AND ('procedure:' || (@procedure_id::uuid)::text) = ANY (r.flags)
        AND (@version::integer = 0 OR (r.procedure_trace->>'version')::integer = @version)
)
SELECT count(*)::integer AS matched,
    count(*) FILTER (WHERE f.ai_involved)::integer AS involved,
    count(*) FILTER (WHERE f.ai_involved AND f.human_replies = 0 AND f.ai_handed_off_at IS NULL AND (f.ai_confirmed
        OR f.closed_at IS NOT NULL OR (f.ai_last_answer_at >= COALESCE(f.last_customer_message_at, f.started_at)
        AND f.ai_last_answer_at < @quiet_before)))::integer AS resolved,
    count(*) FILTER (WHERE f.ai_involved AND f.human_replies = 0 AND f.ai_handed_off_at IS NULL
        AND f.ai_confirmed)::integer AS confirmed,
    count(*) FILTER (WHERE f.ai_involved AND f.ai_handed_off_at IS NOT NULL)::integer AS handed_off,
    count(f.rating)::integer AS rated,
    count(*) FILTER (WHERE f.rating >= 4)::integer AS positive,
    COALESCE(avg(f.rating), 0)::float8 AS mean_rating
FROM convs c
JOIN conversation_facts f ON f.conversation_id = c.conversation_id AND f.workspace_id = @workspace_id;

-- name: ProcedureHandoffReasons :many
-- ProcedureHandoffReasons counts the conversations a procedure handed to
-- the team over a range, by why.
SELECT COALESCE(r.handoff_reason, 'other')::text AS reason, count(DISTINCT r.conversation_id)::integer AS conversations
FROM ai_runs r
WHERE r.workspace_id = @workspace_id AND r.kind = 'conversation' AND r.outcome = 'handoff'
    AND r.created_at >= @since AND r.created_at < @until
    AND ('procedure:' || (@procedure_id::uuid)::text) = ANY (r.flags)
    AND (@version::integer = 0 OR (r.procedure_trace->>'version')::integer = @version)
GROUP BY 1
ORDER BY 2 DESC;

-- name: ProcedureGuardEvents :many
-- ProcedureGuardEvents counts the times a procedure's guardrails stepped
-- in over a range, by kind.
SELECT (e->>'type')::text AS type, count(*)::integer AS events
FROM ai_runs r, jsonb_array_elements(COALESCE(r.procedure_trace->'guard_events', '[]'::jsonb)) e
WHERE r.workspace_id = @workspace_id AND r.kind = 'conversation' AND r.created_at >= @since
    AND r.created_at < @until AND ('procedure:' || (@procedure_id::uuid)::text) = ANY (r.flags)
    AND (@version::integer = 0 OR (r.procedure_trace->>'version')::integer = @version)
GROUP BY 1;

-- name: ProcedureStepFigures :many
-- ProcedureStepFigures counts, for each step of a procedure, the
-- conversations that reached it, finished it, and took each side of it.
SELECT (s->>'step_id')::text AS step_id,
    count(DISTINCT r.conversation_id)::integer AS reached,
    count(DISTINCT r.conversation_id) FILTER (WHERE s->>'status' = 'done')::integer AS done,
    count(DISTINCT r.conversation_id) FILTER (WHERE s->>'branch_taken' = 'then')::integer AS took_then,
    count(DISTINCT r.conversation_id) FILTER (WHERE s->>'branch_taken' = 'else')::integer AS took_else
FROM ai_runs r, jsonb_array_elements(COALESCE(r.procedure_trace->'steps', '[]'::jsonb)) s
WHERE r.workspace_id = @workspace_id AND r.kind = 'conversation' AND r.created_at >= @since
    AND r.created_at < @until AND ('procedure:' || (@procedure_id::uuid)::text) = ANY (r.flags)
    AND (@version::integer = 0 OR (r.procedure_trace->>'version')::integer = @version)
GROUP BY 1;

-- name: ProcedureActionFigures :many
-- ProcedureActionFigures counts the calls of each action in the runs that
-- followed a procedure over a range: how many, how many failed, and how
-- long they took on average.
SELECT c.action_id, count(*)::integer AS calls,
    count(*) FILTER (WHERE c.state = 'failed')::integer AS failed,
    COALESCE(avg(c.latency_ms), 0)::float8 AS mean_latency_ms
FROM ai_action_calls c
JOIN ai_runs r ON r.id = c.run_id AND r.workspace_id = c.workspace_id
WHERE c.workspace_id = @workspace_id AND r.kind = 'conversation' AND r.created_at >= @since
    AND r.created_at < @until AND ('procedure:' || (@procedure_id::uuid)::text) = ANY (r.flags)
    AND (@version::integer = 0 OR (r.procedure_trace->>'version')::integer = @version)
GROUP BY c.action_id;

-- name: ProcedureRunFacts :many
-- ProcedureRunFacts returns the conversation runs that followed a
-- procedure over a range, newest first, with their conversation's rating
-- and reopens, for drilling into a metric and finding what went wrong.
SELECT r.id, r.conversation_id, r.outcome, r.handoff_reason, r.procedure_trace, r.created_at,
    cv.number, f.rating, COALESCE(f.reopens, 0)::integer AS reopens
FROM ai_runs r
JOIN conversations cv ON cv.id = r.conversation_id AND cv.workspace_id = r.workspace_id
LEFT JOIN conversation_facts f ON f.conversation_id = r.conversation_id AND f.workspace_id = r.workspace_id
WHERE r.workspace_id = @workspace_id AND r.kind = 'conversation' AND r.created_at >= @since
    AND r.created_at < @until AND ('procedure:' || (@procedure_id::uuid)::text) = ANY (r.flags)
ORDER BY r.created_at DESC, r.id DESC
LIMIT @row_limit;

-- name: ReportProcedures :many
-- ReportProcedures sums up every procedure that ran in a range: the
-- conversations it ran in, resolved, handed to the team and rated, and
-- how often its guardrails stepped in.
WITH runs AS (
    SELECT r.conversation_id, substring(flag FROM 11)::uuid AS procedure_id, r.procedure_trace
    FROM ai_runs r, unnest(r.flags) AS flag
    WHERE r.workspace_id = @workspace_id AND r.kind = 'conversation' AND r.conversation_id IS NOT NULL
        AND r.created_at >= @since AND r.created_at < @until AND flag LIKE 'procedure:%'
), convs AS (
    SELECT DISTINCT procedure_id, conversation_id FROM runs
), guards AS (
    SELECT procedure_id, count(*)::integer AS events
    FROM runs, jsonb_array_elements(COALESCE(procedure_trace->'guard_events', '[]'::jsonb))
    GROUP BY procedure_id
)
SELECT p.id, p.name, count(*)::integer AS matched,
    count(*) FILTER (WHERE f.ai_involved AND f.human_replies = 0 AND f.ai_handed_off_at IS NULL AND (f.ai_confirmed
        OR f.closed_at IS NOT NULL OR (f.ai_last_answer_at >= COALESCE(f.last_customer_message_at, f.started_at)
        AND f.ai_last_answer_at < @quiet_before)))::integer AS resolved,
    count(*) FILTER (WHERE f.ai_involved)::integer AS involved,
    count(*) FILTER (WHERE f.ai_involved AND f.ai_handed_off_at IS NOT NULL)::integer AS handed_off,
    count(f.rating)::integer AS rated,
    COALESCE(avg(f.rating), 0)::float8 AS mean_rating,
    COALESCE(max(g.events), 0)::integer AS guard_events
FROM convs c
JOIN procedures p ON p.id = c.procedure_id AND p.workspace_id = @workspace_id
JOIN conversation_facts f ON f.conversation_id = c.conversation_id AND f.workspace_id = @workspace_id
LEFT JOIN guards g ON g.procedure_id = p.id
GROUP BY p.id, p.name
ORDER BY matched DESC, p.name;
