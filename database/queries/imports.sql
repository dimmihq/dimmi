-- name: CreateImport :one
-- CreateImport keeps an uploaded spreadsheet as a draft, with its header
-- and how many rows it holds.
INSERT INTO imports (workspace_id, kind, file_name, file, columns, row_count, created_by_member_id, created_at, updated_at)
VALUES (@workspace_id, @kind, @file_name, @file, @columns, @row_count, @created_by_member_id, @now, @now)
RETURNING *;

-- name: FindImport :one
SELECT * FROM imports WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListImports :many
-- ListImports returns the workspace's latest imports of the kind, newest
-- first, without their files, each with the name of who uploaded it.
SELECT i.id, i.workspace_id, i.kind, i.status, i.file_name, i.row_count, i.mode, i.processed_rows,
    i.created_count, i.updated_count, i.skipped_count, i.failed_count, i.error, i.created_by_member_id,
    i.started_at, i.finished_at, i.created_at, i.updated_at, i.rollback_status, i.rollback_total,
    i.rollback_processed, i.rollback_removed, i.rollback_reverted, i.rollback_kept, i.rollback_kept_records,
    i.rollback_error, i.rollback_requested_at, i.rollback_finished_at, i.file_expired_at,
    u.name AS created_by_name
FROM imports i
LEFT JOIN workspace_members m ON m.id = i.created_by_member_id
LEFT JOIN users u ON u.id = m.user_id
WHERE i.workspace_id = @workspace_id AND i.kind = @kind
ORDER BY i.created_at DESC, i.id DESC
LIMIT @row_limit;

-- name: QueueImport :one
-- QueueImport fixes a draft's mapping and mode and hands it to the
-- background. Only a draft is queued, so a run starts once.
UPDATE imports SET status = 'queued', mapping = @mapping, mode = @mode, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND status = 'draft'
RETURNING *;

-- name: StartImport :one
-- StartImport claims a queued import for its run; a retried run picks up
-- where the last one stopped.
UPDATE imports SET status = 'running', started_at = coalesce(started_at, @now::timestamptz), updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('queued', 'running')
RETURNING *;

-- name: RecordImportProgress :exec
-- RecordImportProgress adds what a batch of rows came to.
UPDATE imports
SET processed_rows = @processed_rows, created_count = created_count + @created,
    updated_count = updated_count + @updated, skipped_count = skipped_count + @skipped,
    failed_count = failed_count + @failed, row_errors = row_errors || @row_errors::jsonb, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FinishImport :one
UPDATE imports SET status = 'completed', finished_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: FailImport :exec
UPDATE imports SET status = 'failed', error = @error, finished_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindCompanyByName :one
-- FindCompanyByName returns the oldest live company with the name, in
-- any case, for an imported customer whose company has only a name.
SELECT * FROM companies
WHERE workspace_id = @workspace_id AND lower(name) = lower(@name::text) AND deleted_at IS NULL
ORDER BY created_at, id
LIMIT 1;

-- name: LinkImportedContactCompany :exec
-- LinkImportedContactCompany puts an imported customer in a company.
UPDATE contacts SET company_id = @company_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: RecordImportChange :exec
-- RecordImportChange keeps what one row did to a record, so the import
-- can be undone: whether it made the record, the fields it changed with
-- their values before and after, and the tags it added.
INSERT INTO import_changes (workspace_id, import_id, record_id, created, before, after, added_tag_ids, created_at)
VALUES (@workspace_id, @import_id, @record_id, @created, @before, @after, @added_tag_ids, @now);

-- name: ListImportChanges :many
-- ListImportChanges returns what the import did, in the order it did it.
SELECT * FROM import_changes WHERE workspace_id = @workspace_id AND import_id = @import_id ORDER BY id;

-- name: CountImportChangedRecords :one
SELECT count(DISTINCT record_id)::integer FROM import_changes
WHERE workspace_id = @workspace_id AND import_id = @import_id;

-- name: RequestImportRollback :one
-- RequestImportRollback queues the undoing of a finished import, once,
-- or again after an undo that failed.
UPDATE imports
SET rollback_status = 'queued', rollback_total = @total, rollback_processed = 0, rollback_removed = 0,
    rollback_reverted = 0, rollback_kept = 0, rollback_kept_records = '[]', rollback_error = NULL,
    rollback_by_member_id = @member_id, rollback_requested_at = @now::timestamptz, rollback_finished_at = NULL,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND status IN ('completed', 'failed')
  AND (rollback_status IS NULL OR rollback_status = 'failed')
RETURNING *;

-- name: StartImportRollback :one
-- StartImportRollback claims a queued undo for its run; a retried run
-- picks up where the last one stopped.
UPDATE imports SET rollback_status = 'running', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND rollback_status IN ('queued', 'running')
RETURNING *;

-- name: RecordImportRollbackProgress :exec
-- RecordImportRollbackProgress adds what a batch of records came to.
UPDATE imports
SET rollback_processed = @processed, rollback_removed = rollback_removed + @removed,
    rollback_reverted = rollback_reverted + @reverted, rollback_kept = rollback_kept + @kept,
    rollback_kept_records = rollback_kept_records || @kept_records::jsonb, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FinishImportRollback :exec
UPDATE imports SET rollback_status = 'completed', rollback_finished_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: FailImportRollback :exec
UPDATE imports
SET rollback_status = 'failed', rollback_error = @error, rollback_finished_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListImportsToExpire :many
-- ListImportsToExpire returns which finished imports still hold a file
-- older than the cutoff, across workspaces, oldest first.
SELECT e.workspace_id::uuid AS workspace_id, e.id::uuid AS id
FROM imports_to_expire(@expire_before::timestamptz, @row_limit::integer) AS e;

-- name: ExpireImportFile :exec
-- ExpireImportFile empties the import's file; the import and its counts
-- stay.
UPDATE imports SET file = ''::bytea, file_expired_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListImportChangesToPrune :many
-- ListImportChangesToPrune returns which imports finished before the
-- cutoff still keep changes for undoing, across workspaces.
SELECT e.workspace_id::uuid AS workspace_id, e.id::uuid AS id
FROM import_changes_to_prune(@prune_before::timestamptz, @row_limit::integer) AS e;

-- name: DeleteImportChanges :exec
DELETE FROM import_changes WHERE workspace_id = @workspace_id AND import_id = @import_id;
