-- name: ListSLAPolicies :many
-- ListSLAPolicies returns the workspace's policies in its order, archived
-- ones last.
SELECT * FROM sla_policies WHERE workspace_id = @workspace_id
ORDER BY archived_at IS NOT NULL, position, created_at, id;

-- name: FindSLAPolicy :one
-- FindSLAPolicy returns one policy.
SELECT * FROM sla_policies WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateSLAPolicy :one
-- CreateSLAPolicy stores a policy after the others.
INSERT INTO sla_policies (workspace_id, name, description, schedule_id, first_reply_seconds, next_reply_seconds,
    follow_up_seconds, resolution_seconds, warn_before_seconds, pause_on_waiting_customer, pause_on_waiting_team,
    breach_priority, breach_team_id, ticket_type_ids, priorities, position, created_at, updated_at)
VALUES (@workspace_id, @name, @description, sqlc.narg(schedule_id), sqlc.narg(first_reply_seconds),
    sqlc.narg(next_reply_seconds), sqlc.narg(follow_up_seconds), sqlc.narg(resolution_seconds), @warn_before_seconds,
    @pause_on_waiting_customer, @pause_on_waiting_team, sqlc.narg(breach_priority), sqlc.narg(breach_team_id),
    @ticket_type_ids, @priorities,
    (SELECT coalesce(max(position) + 1, 0) FROM sla_policies WHERE workspace_id = @workspace_id), @now, @now)
RETURNING *;

-- name: UpdateSLAPolicy :one
-- UpdateSLAPolicy changes a policy; running clocks keep their due times.
UPDATE sla_policies SET name = @name, description = @description, schedule_id = sqlc.narg(schedule_id),
    first_reply_seconds = sqlc.narg(first_reply_seconds), next_reply_seconds = sqlc.narg(next_reply_seconds),
    follow_up_seconds = sqlc.narg(follow_up_seconds), resolution_seconds = sqlc.narg(resolution_seconds),
    warn_before_seconds = @warn_before_seconds, pause_on_waiting_customer = @pause_on_waiting_customer,
    pause_on_waiting_team = @pause_on_waiting_team, breach_priority = sqlc.narg(breach_priority),
    breach_team_id = sqlc.narg(breach_team_id), ticket_type_ids = @ticket_type_ids, priorities = @priorities,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ArchiveSLAPolicy :exec
-- ArchiveSLAPolicy stops a policy from applying to new work and cancels
-- its running clocks.
UPDATE sla_policies SET archived_at = @now, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: CancelPolicyClocks :exec
-- CancelPolicyClocks stops every running or paused clock of a policy.
UPDATE sla_clocks SET state = 'canceled', stopped_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND policy_id = @policy_id AND state IN ('running', 'paused');

-- name: CreateSLAClock :one
-- CreateSLAClock starts a target's clock, unless one is already running.
INSERT INTO sla_clocks (workspace_id, policy_id, ticket_id, conversation_id, target, state, started_at, due_at,
    warn_at, created_at, updated_at)
VALUES (@workspace_id, @policy_id, sqlc.narg(ticket_id), sqlc.narg(conversation_id), @target, 'running', @now,
    @due_at, sqlc.narg(warn_at), @now, @now)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ListSubjectClocks :many
-- ListSubjectClocks returns every clock of a ticket or conversation,
-- newest first.
SELECT * FROM sla_clocks
WHERE workspace_id = @workspace_id
  AND (ticket_id = sqlc.narg(ticket_id)::uuid OR conversation_id = sqlc.narg(conversation_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT 100;

-- name: ListOpenSubjectClocks :many
-- ListOpenSubjectClocks returns the running and paused clocks of a ticket
-- or conversation.
SELECT * FROM sla_clocks
WHERE workspace_id = @workspace_id AND state IN ('running', 'paused')
  AND (ticket_id = sqlc.narg(ticket_id)::uuid OR conversation_id = sqlc.narg(conversation_id)::uuid)
ORDER BY due_at;

-- name: ListOpenClocksOfSubjects :many
-- ListOpenClocksOfSubjects returns the running and paused clocks of the
-- tickets and conversations, for their rows.
SELECT * FROM sla_clocks
WHERE workspace_id = @workspace_id AND state IN ('running', 'paused')
  AND (ticket_id = ANY (@ticket_ids::uuid[]) OR conversation_id = ANY (@conversation_ids::uuid[]))
ORDER BY due_at;

-- name: FindSLAClock :one
-- FindSLAClock returns one clock.
SELECT * FROM sla_clocks WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockSLAClock :one
-- LockSLAClock holds a clock's row until the transaction ends.
SELECT * FROM sla_clocks WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: SaveSLAClock :one
-- SaveSLAClock stores a clock's state and times.
UPDATE sla_clocks SET state = @state, due_at = @due_at, warn_at = sqlc.narg(warn_at), paused_at = sqlc.narg(paused_at),
    warned_at = sqlc.narg(warned_at), breached_at = sqlc.narg(breached_at), stopped_at = sqlc.narg(stopped_at),
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SLAClocksDue :many
-- SLAClocksDue returns the running clocks that must warn or have
-- breached by the time, across workspaces.
SELECT d.workspace_id::uuid AS workspace_id, d.id::uuid AS id
FROM sla_clocks_due(@due_at::timestamptz, @row_limit::integer) AS d;

-- name: ListClocksBetween :many
-- ListClocksBetween returns the clocks that ended in the range, for SLA
-- attainment reports.
SELECT c.*, p.name AS policy_name FROM sla_clocks c
JOIN sla_policies p ON p.id = c.policy_id
WHERE c.workspace_id = @workspace_id AND c.state IN ('met', 'breached')
  AND coalesce(c.stopped_at, c.breached_at) >= @ended_from AND coalesce(c.stopped_at, c.breached_at) < @ended_to;
