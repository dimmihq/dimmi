-- name: ListSchedules :many
-- ListSchedules returns the workspace's schedules, the default first.
SELECT * FROM schedules
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
ORDER BY is_default DESC, lower(name), id;

-- name: FindSchedule :one
-- FindSchedule returns one schedule.
SELECT * FROM schedules WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: FindDefaultSchedule :one
-- FindDefaultSchedule returns the schedule used wherever none is named.
SELECT * FROM schedules WHERE workspace_id = @workspace_id AND is_default AND deleted_at IS NULL;

-- name: CreateSchedule :one
-- CreateSchedule stores a schedule.
INSERT INTO schedules (id, workspace_id, name, timezone, weekly, holidays, is_default, created_at, updated_at)
VALUES (@id, @workspace_id, @name, @timezone, @weekly, @holidays, @is_default, @now, @now)
RETURNING *;

-- name: UpdateSchedule :one
-- UpdateSchedule saves a schedule's name, timezone, week and holidays.
UPDATE schedules SET name = @name, timezone = @timezone, weekly = @weekly, holidays = @holidays, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ClearDefaultSchedule :exec
-- ClearDefaultSchedule takes the default mark off every schedule, so
-- another can take it.
UPDATE schedules SET is_default = false, updated_at = @now
WHERE workspace_id = @workspace_id AND is_default AND deleted_at IS NULL;

-- name: MarkDefaultSchedule :exec
-- MarkDefaultSchedule makes the schedule the workspace's default.
UPDATE schedules SET is_default = true, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: DeleteSchedule :exec
-- DeleteSchedule removes a schedule.
UPDATE schedules SET deleted_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: ScheduleNameTaken :one
-- ScheduleNameTaken reports whether another schedule has the name.
SELECT EXISTS (
    SELECT 1 FROM schedules
    WHERE workspace_id = @workspace_id AND lower(name) = lower(@name) AND id <> @id AND deleted_at IS NULL
)::boolean;

-- name: ScheduleInUse :one
-- ScheduleInUse reports whether a workflow that is not archived names
-- the schedule, live or in its working copy.
SELECT EXISTS (
    SELECT 1 FROM workflows w
    LEFT JOIN workflow_versions v ON v.id = w.live_version_id
    WHERE w.workspace_id = @workspace_id AND w.deleted_at IS NULL
      AND (strpos(w.draft::text, @public_id::text) > 0 OR strpos(coalesce(v.spec::text, ''), @public_id::text) > 0)
)::boolean;
