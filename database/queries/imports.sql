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
-- first, without their files.
SELECT id, workspace_id, kind, status, file_name, row_count, mode, processed_rows, created_count, updated_count,
    skipped_count, failed_count, error, created_by_member_id, started_at, finished_at, created_at, updated_at
FROM imports
WHERE workspace_id = @workspace_id AND kind = @kind
ORDER BY created_at DESC, id DESC
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
