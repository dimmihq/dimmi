-- name: StartSeriesStep :one
-- StartSeriesStep records that a traveler reached a step.
INSERT INTO series_steps (workspace_id, series_id, enrollment_id, version_id, node_id, kind, status, test, entered_at)
VALUES (@workspace_id, @series_id, @enrollment_id, @version_id, @node_id, @kind, 'waiting', @test, @now::timestamptz)
RETURNING *;

-- name: FinishSeriesStep :exec
-- FinishSeriesStep records how a step ended: the path taken and the
-- outcome, with a few facts for the trace.
UPDATE series_steps SET status = @status, path_id = sqlc.narg(path_id), outcome = sqlc.narg(outcome),
    detail = @detail, left_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: NoteSeriesStep :exec
-- NoteSeriesStep keeps what a waiting step learned so far for the trace.
UPDATE series_steps SET outcome = sqlc.narg(outcome), detail = @detail
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindSeriesStep :one
-- FindSeriesStep returns one step of a journey.
SELECT * FROM series_steps WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListEnrollmentSteps :many
-- ListEnrollmentSteps returns a journey's steps in the order taken.
SELECT * FROM series_steps WHERE workspace_id = @workspace_id AND enrollment_id = @enrollment_id
ORDER BY entered_at, id;

-- name: SeriesNodeTotals :many
-- SeriesNodeTotals counts, for each step of a series, how many reached
-- it and how many left by each path or outcome.
SELECT node_id, coalesce(path_id, '')::text AS path_id, coalesce(outcome, '')::text AS outcome, status,
    count(*) AS n
FROM series_steps
WHERE workspace_id = @workspace_id AND series_id = @series_id AND NOT test
GROUP BY node_id, path_id, outcome, status;

-- name: CountContactSeriesStepsSince :one
-- CountContactSeriesStepsSince counts the series messages a contact was
-- sent since the time, across every series.
SELECT count(*) FROM broadcast_recipients
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND series_step_id IS NOT NULL
  AND status IN ('queued', 'sent', 'delivered') AND created_at >= @since::timestamptz;
