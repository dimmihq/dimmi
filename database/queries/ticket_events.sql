-- name: CreateTicketEvent :one
-- CreateTicketEvent appends one change to a ticket's history.
INSERT INTO ticket_events (workspace_id, ticket_id, kind, actor_type, actor_id, data, created_at)
VALUES (@workspace_id, @ticket_id, @kind, @actor_type, sqlc.narg(actor_id), @data, @now)
RETURNING *;

-- name: ListTicketEvents :many
-- ListTicketEvents returns a ticket's history, oldest first, after the
-- cursor.
SELECT * FROM ticket_events
WHERE workspace_id = @workspace_id AND ticket_id = @ticket_id
  AND (sqlc.narg(after)::uuid IS NULL OR id > sqlc.narg(after)::uuid)
ORDER BY id
LIMIT @row_limit;

-- name: ListWorkspaceTicketEvents :many
-- ListWorkspaceTicketEvents returns every ticket change of the
-- workspace after the cursor, oldest first, trashed tickets included,
-- for exports that follow the log.
SELECT e.*, t.number AS ticket_number FROM ticket_events e
JOIN tickets t ON t.id = e.ticket_id
WHERE e.workspace_id = @workspace_id
  AND (sqlc.narg(after)::uuid IS NULL OR e.id > sqlc.narg(after)::uuid)
  AND (sqlc.narg(since)::timestamptz IS NULL OR e.created_at >= sqlc.narg(since)::timestamptz)
ORDER BY e.id
LIMIT @row_limit;

-- name: ListStateEventsBetween :many
-- ListStateEventsBetween returns the state changes of tickets created
-- in the range, for time-in-state reports.
SELECT e.ticket_id, e.data, e.created_at, t.type_id, t.created_at AS ticket_created_at
FROM ticket_events e
JOIN tickets t ON t.id = e.ticket_id
WHERE e.workspace_id = @workspace_id AND e.kind IN ('created', 'state')
  AND t.created_at >= @created_from AND t.created_at < @created_to AND t.deleted_at IS NULL
ORDER BY e.ticket_id, e.id;
