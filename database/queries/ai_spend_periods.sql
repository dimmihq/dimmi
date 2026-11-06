-- name: GetSpendPeriod :one
-- GetSpendPeriod returns the workspace's tally for a period.
SELECT * FROM ai_spend_periods
WHERE workspace_id = @workspace_id AND period_start = @period_start;

-- name: AddSpend :one
-- AddSpend adds a run's cost and counts to the period tally, atomically.
INSERT INTO ai_spend_periods (
    workspace_id, period_start, cost_micros, input_tokens, cached_input_tokens, output_tokens,
    runs_count, answered_count, unknown_count, handoff_count, silent_count, updated_at
) VALUES (
    @workspace_id, @period_start, @cost_micros, @input_tokens, @cached_input_tokens, @output_tokens,
    @runs_count, @answered_count, @unknown_count, @handoff_count, @silent_count, @now
)
ON CONFLICT (workspace_id, period_start) DO UPDATE SET
    cost_micros = ai_spend_periods.cost_micros + @cost_micros,
    input_tokens = ai_spend_periods.input_tokens + @input_tokens,
    cached_input_tokens = ai_spend_periods.cached_input_tokens + @cached_input_tokens,
    output_tokens = ai_spend_periods.output_tokens + @output_tokens,
    runs_count = ai_spend_periods.runs_count + @runs_count,
    answered_count = ai_spend_periods.answered_count + @answered_count,
    unknown_count = ai_spend_periods.unknown_count + @unknown_count,
    handoff_count = ai_spend_periods.handoff_count + @handoff_count,
    silent_count = ai_spend_periods.silent_count + @silent_count,
    updated_at = @now
RETURNING *;

-- name: SetSpendCap :exec
-- SetSpendCap marks the period capped, or clears it when the cap
-- lifted.
UPDATE ai_spend_periods SET capped_at = @capped_at, updated_at = @now
WHERE workspace_id = @workspace_id AND period_start = @period_start;

-- name: MarkSpendThreshold :exec
-- MarkSpendThreshold records a threshold notice, once per period.
UPDATE ai_spend_periods SET thresholds_notified = array_append(thresholds_notified, @threshold)
WHERE workspace_id = @workspace_id AND period_start = @period_start
    AND NOT (@threshold = ANY(thresholds_notified));

-- name: ListDailySpend :many
-- ListDailySpend returns per-day cost and runs for the period, days
-- counted in UTC like the period itself.
SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day,
    COALESCE(sum(cost_micros), 0)::bigint AS cost_micros,
    count(*)::integer AS runs_count
FROM ai_runs
WHERE workspace_id = @workspace_id AND created_at >= @period_start AND created_at < @period_end
GROUP BY 1 ORDER BY 1;

-- name: ListSpendByModel :many
-- ListSpendByModel returns per-model cost for the period.
SELECT COALESCE(s.model, '') AS model,
    COALESCE(sum(s.cost_micros), 0)::bigint AS cost_micros,
    count(*)::integer AS calls
FROM ai_run_steps s
JOIN ai_runs r ON r.id = s.run_id AND r.workspace_id = s.workspace_id
WHERE s.workspace_id = @workspace_id AND r.created_at >= @period_start AND r.created_at < @period_end
    AND s.model IS NOT NULL
GROUP BY s.model ORDER BY 2 DESC, 1;

-- name: ListSpendByOutcome :many
-- ListSpendByOutcome returns per-outcome counts for the period.
SELECT COALESCE(outcome, '') AS outcome, count(*)::integer AS runs_count
FROM ai_runs
WHERE workspace_id = @workspace_id AND created_at >= @period_start AND created_at < @period_end
    AND outcome IS NOT NULL
GROUP BY outcome ORDER BY outcome;
