-- name: CreateTicketImport :one
-- CreateTicketImport starts an import of tickets from another tool.
INSERT INTO ticket_imports (workspace_id, source, created_by_member_id, started_at, created_at, updated_at)
VALUES (@workspace_id, @source, sqlc.narg(created_by_member_id), @now, @now, @now)
RETURNING *;

-- name: FindTicketImport :one
SELECT * FROM ticket_imports WHERE workspace_id = @workspace_id AND id = @id;

-- name: RecordTicketImportProgress :exec
-- RecordTicketImportProgress adds what a step of the import came to.
UPDATE ticket_imports
SET types_created = types_created + @types_created, fields_created = fields_created + @fields_created,
    states_created = states_created + @states_created, tickets_created = tickets_created + @tickets_created,
    tickets_found = tickets_found + @tickets_found, tickets_failed = tickets_failed + @tickets_failed,
    links_created = links_created + @links_created, row_errors = row_errors || @row_errors::jsonb,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FinishTicketImport :one
-- FinishTicketImport ends an import, completed or failed with why.
UPDATE ticket_imports SET status = @status, error = sqlc.narg(error), finished_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RecordTicketImportRecord :exec
-- RecordTicketImportRecord keeps one record the import made or found,
-- under the id it had in the other tool.
INSERT INTO ticket_import_records (workspace_id, import_id, record_kind, external_id, record_id, created, created_at)
VALUES (@workspace_id, @import_id, @record_kind, @external_id, @record_id, @created, @now)
ON CONFLICT (import_id, record_kind, external_id) DO NOTHING;

-- name: ListTicketImportRecords :many
-- ListTicketImportRecords returns the records the import made, oldest
-- first.
SELECT * FROM ticket_import_records
WHERE workspace_id = @workspace_id AND import_id = @import_id AND created
ORDER BY id;

-- name: RequestTicketImportRollback :one
-- RequestTicketImportRollback marks a finished import to be undone, once.
UPDATE ticket_imports
SET rollback_status = 'queued', rollback_by_member_id = @member_id::uuid, rollback_requested_at = @now::timestamptz,
    rollback_error = NULL, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND finished_at IS NOT NULL
  AND (rollback_status IS NULL OR rollback_status = 'failed')
RETURNING *;

-- name: StartTicketImportRollback :one
-- StartTicketImportRollback claims a queued undo for its run.
UPDATE ticket_imports SET rollback_status = 'running', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND rollback_status IN ('queued', 'running')
RETURNING *;

-- name: FinishTicketImportRollback :exec
-- FinishTicketImportRollback ends an undo, completed or failed with why.
UPDATE ticket_imports
SET rollback_status = @status::text, rollback_trashed = @trashed, rollback_archived = @archived,
    rollback_error = sqlc.narg(error), rollback_finished_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindImportContactByEmail :one
-- FindImportContactByEmail returns the oldest live customer with the
-- email, for a ticket's requester brought in from another tool.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id AND email = @email::text AND deleted_at IS NULL
ORDER BY created_at, id
LIMIT 1;

-- name: CountTicketsUsingType :one
-- CountTicketsUsingType says how many live tickets have the type.
SELECT count(*) FROM tickets WHERE workspace_id = @workspace_id AND type_id = @type_id AND deleted_at IS NULL;

-- name: CountTicketsUsingState :one
-- CountTicketsUsingState says how many live tickets are in the state.
SELECT count(*) FROM tickets WHERE workspace_id = @workspace_id AND state_id = @state_id AND deleted_at IS NULL;

