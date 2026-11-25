-- name: WorkflowRunFigures :one
-- WorkflowRunFigures counts a workflow's live runs that started in a
-- range and did something: how many, how many the customer wrote
-- during, completed, still going, went to a person, failed, and reached
-- the goal. Version zero counts every version. A goal kind of '' counts
-- no goal; 'resolved' is the conversation closed inside the window with
-- no handover and no teammate reply; 'csat' a rating of at least
-- min_rating given inside the window.
WITH runs AS (
    SELECT r.id, r.status, r.started_at, r.conversation_id,
        EXISTS (
            SELECT 1 FROM messages m
            WHERE m.workspace_id = r.workspace_id AND m.conversation_id = r.conversation_id AND m.type = 'message'
              AND m.author_type = 'contact' AND m.deleted_at IS NULL AND m.created_at > r.started_at
              AND m.created_at <= COALESCE(r.ended_at, now())
        ) AS engaged,
        EXISTS (
            SELECT 1 FROM workflow_run_steps s
            WHERE s.workspace_id = r.workspace_id AND s.run_id = r.id
              AND ((s.kind = 'dimmi' AND s.outcome = 'handed_over') OR (s.kind = 'assign' AND s.outcome = 'assigned')
                OR (s.kind = 'escalate' AND s.status = 'completed'))
        ) AS handed_over,
        EXISTS (
            SELECT 1 FROM workflow_run_steps s
            WHERE s.workspace_id = r.workspace_id AND s.run_id = r.id
              AND (s.status = 'failed' OR s.outcome = 'failed' OR (s.kind = 'classify' AND s.outcome = 'none'
                AND COALESCE(s.detail->>'reason', '') NOT IN ('no_match', 'low_confidence')))
        ) AS step_failed
    FROM workflow_runs r
    LEFT JOIN workflow_versions v ON v.id = r.version_id
    WHERE r.workspace_id = @workspace_id AND r.workflow_id = @workflow_id AND r.mode = 'live' AND r.status <> 'passed'
      AND r.started_at >= @since AND r.started_at < @until
      AND (@version::integer = 0 OR v.number = @version::integer)
)
SELECT count(*)::integer AS triggered,
    count(*) FILTER (WHERE r.engaged)::integer AS engaged,
    count(*) FILTER (WHERE r.status = 'completed')::integer AS completed,
    count(*) FILTER (WHERE r.status IN ('running', 'waiting'))::integer AS active,
    count(*) FILTER (WHERE r.handed_over)::integer AS handed_over,
    count(*) FILTER (WHERE r.status = 'failed' OR r.step_failed)::integer AS failed,
    count(*) FILTER (WHERE
        (@goal_kind::text = 'resolved' AND NOT r.handed_over AND f.ai_handed_off_at IS NULL AND f.human_replies = 0
            AND f.closed_at >= r.started_at
            AND f.closed_at < r.started_at + make_interval(secs => @goal_window_seconds::integer))
        OR (@goal_kind::text = 'csat' AND f.rating >= @goal_min_rating::integer AND f.rated_at >= r.started_at
            AND f.rated_at < r.started_at + make_interval(secs => @goal_window_seconds::integer))
    )::integer AS goal
FROM runs r
LEFT JOIN conversation_facts f ON f.conversation_id = r.conversation_id AND f.workspace_id = @workspace_id;

-- name: WorkflowRunsByDay :many
-- WorkflowRunsByDay counts a workflow's live runs that did something per
-- day of a range: started, completed and failed.
SELECT date_trunc('day', r.started_at)::timestamptz AS day, count(*)::integer AS triggered,
    count(*) FILTER (WHERE r.status = 'completed')::integer AS completed,
    count(*) FILTER (WHERE r.status = 'failed')::integer AS failed
FROM workflow_runs r
LEFT JOIN workflow_versions v ON v.id = r.version_id
WHERE r.workspace_id = @workspace_id AND r.workflow_id = @workflow_id AND r.mode = 'live' AND r.status <> 'passed'
  AND r.started_at >= @since AND r.started_at < @until
  AND (@version::integer = 0 OR v.number = @version::integer)
GROUP BY 1
ORDER BY 1;

-- name: WorkflowStepFigures :many
-- WorkflowStepFigures counts, for each step of a workflow, the live runs
-- of a range that reached it, wait on it now and failed on it.
SELECT s.step_id, count(DISTINCT s.run_id)::integer AS reached,
    count(DISTINCT s.run_id) FILTER (WHERE s.status = 'waiting' AND r.status = 'waiting')::integer AS waiting,
    count(DISTINCT s.run_id) FILTER (WHERE s.status = 'failed' OR s.outcome = 'failed' OR (s.kind = 'classify'
        AND s.outcome = 'none' AND COALESCE(s.detail->>'reason', '') NOT IN ('no_match', 'low_confidence'))
    )::integer AS failed
FROM workflow_run_steps s
JOIN workflow_runs r ON r.id = s.run_id AND r.workspace_id = s.workspace_id
LEFT JOIN workflow_versions v ON v.id = r.version_id
WHERE s.workspace_id = @workspace_id AND s.workflow_id = @workflow_id AND r.mode = 'live'
  AND r.started_at >= @since AND r.started_at < @until
  AND (@version::integer = 0 OR v.number = @version::integer)
GROUP BY s.step_id;

-- name: WorkflowStepPaths :many
-- WorkflowStepPaths counts, for each step of a workflow, the live runs
-- of a range that left it by each path.
SELECT s.step_id, COALESCE(s.path_id, '')::text AS path_id, count(DISTINCT s.run_id)::integer AS runs
FROM workflow_run_steps s
JOIN workflow_runs r ON r.id = s.run_id AND r.workspace_id = s.workspace_id
LEFT JOIN workflow_versions v ON v.id = r.version_id
WHERE s.workspace_id = @workspace_id AND s.workflow_id = @workflow_id AND r.mode = 'live' AND s.path_id IS NOT NULL
  AND s.status = 'completed' AND r.started_at >= @since AND r.started_at < @until
  AND (@version::integer = 0 OR v.number = @version::integer)
GROUP BY s.step_id, s.path_id;

-- name: WorkflowStepDropoffs :many
-- WorkflowStepDropoffs counts, for each step of a workflow, the live runs
-- of a range that ended on it without completing: the customer left,
-- a teammate stepped in, the run failed or it expired.
SELECT last.step_id, count(*)::integer AS dropped
FROM (
    SELECT DISTINCT ON (s.run_id) s.run_id, s.step_id
    FROM workflow_run_steps s
    JOIN workflow_runs r ON r.id = s.run_id AND r.workspace_id = s.workspace_id
    LEFT JOIN workflow_versions v ON v.id = r.version_id
    WHERE s.workspace_id = @workspace_id AND s.workflow_id = @workflow_id AND r.mode = 'live'
      AND r.status IN ('exited', 'canceled', 'failed', 'expired')
      AND r.started_at >= @since AND r.started_at < @until
      AND (@version::integer = 0 OR v.number = @version::integer)
    ORDER BY s.run_id, s.started_at DESC, s.id DESC
) last
GROUP BY last.step_id;

-- name: WorkflowFailures :many
-- WorkflowFailures lists a workflow's live runs of a range that failed,
-- or one of whose steps failed (a call or action that failed, a classify
-- step the AI could not answer), newest first, with the first failing
-- step and the conversation, before a cursor when paging, narrowed to
-- one step when step_id is set.
SELECT r.id, r.conversation_id, r.status, r.end_reason, r.error, r.started_at, r.ended_at,
    COALESCE(cv.number, 0)::bigint AS conversation_number, COALESCE(v.number, 0)::integer AS version_number,
    COALESCE(fs.step_id, '')::text AS step_id, COALESCE(fs.kind, '')::text AS step_kind,
    COALESCE(fs.outcome, '')::text AS step_outcome, COALESCE(fs.detail, '{}'::jsonb)::jsonb AS step_detail
FROM workflow_runs r
LEFT JOIN workflow_versions v ON v.id = r.version_id
LEFT JOIN conversations cv ON cv.id = r.conversation_id AND cv.workspace_id = r.workspace_id
LEFT JOIN LATERAL (
    SELECT s.step_id, s.kind, s.outcome, s.detail
    FROM workflow_run_steps s
    WHERE s.workspace_id = r.workspace_id AND s.run_id = r.id
      AND (s.status = 'failed' OR s.outcome = 'failed' OR (s.kind = 'classify' AND s.outcome = 'none'
        AND COALESCE(s.detail->>'reason', '') NOT IN ('no_match', 'low_confidence')))
    ORDER BY s.started_at, s.id
    LIMIT 1
) fs ON true
WHERE r.workspace_id = @workspace_id AND r.workflow_id = @workflow_id AND r.mode = 'live'
  AND r.started_at >= @since AND r.started_at < @until
  AND (r.status = 'failed' OR fs.step_id IS NOT NULL)
  AND (sqlc.narg(step_id)::text IS NULL OR fs.step_id = sqlc.narg(step_id)::text)
  AND (sqlc.narg(before)::uuid IS NULL OR r.id < sqlc.narg(before)::uuid)
ORDER BY r.id DESC
LIMIT @row_limit;

-- name: WorkflowVariantGoals :many
-- WorkflowVariantGoals counts, for each variant of each split step of a
-- workflow, the live runs of a range that did something and took it,
-- and how many of those reached the goal, counted as WorkflowRunFigures
-- counts it.
WITH runs AS (
    SELECT DISTINCT ON (s.run_id, s.step_id) s.run_id, s.step_id, s.path_id, r.started_at, r.conversation_id,
        EXISTS (
            SELECT 1 FROM workflow_run_steps h
            WHERE h.workspace_id = r.workspace_id AND h.run_id = r.id
              AND ((h.kind = 'dimmi' AND h.outcome = 'handed_over') OR (h.kind = 'assign' AND h.outcome = 'assigned')
                OR (h.kind = 'escalate' AND h.status = 'completed'))
        ) AS handed_over
    FROM workflow_run_steps s
    JOIN workflow_runs r ON r.id = s.run_id AND r.workspace_id = s.workspace_id
    LEFT JOIN workflow_versions v ON v.id = r.version_id
    WHERE s.workspace_id = @workspace_id AND s.workflow_id = @workflow_id AND s.kind = 'split'
      AND s.status = 'completed' AND s.path_id IS NOT NULL
      AND r.mode = 'live' AND r.status <> 'passed' AND r.started_at >= @since AND r.started_at < @until
      AND (@version::integer = 0 OR v.number = @version::integer)
    ORDER BY s.run_id, s.step_id, s.started_at, s.id
)
SELECT r.step_id, COALESCE(r.path_id, '')::text AS path_id, count(*)::integer AS reached,
    count(*) FILTER (WHERE
        (@goal_kind::text = 'resolved' AND NOT r.handed_over AND f.ai_handed_off_at IS NULL AND f.human_replies = 0
            AND f.closed_at >= r.started_at
            AND f.closed_at < r.started_at + make_interval(secs => @goal_window_seconds::integer))
        OR (@goal_kind::text = 'csat' AND f.rating >= @goal_min_rating::integer AND f.rated_at >= r.started_at
            AND f.rated_at < r.started_at + make_interval(secs => @goal_window_seconds::integer))
    )::integer AS goal
FROM runs r
LEFT JOIN conversation_facts f ON f.conversation_id = r.conversation_id AND f.workspace_id = @workspace_id
GROUP BY r.step_id, r.path_id;
