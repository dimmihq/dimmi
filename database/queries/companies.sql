-- name: CreateCompany :one
-- CreateCompany stores a company in the workspace. An external id
-- another company holds fails on companies_workspace_id_external_id_key.
INSERT INTO companies (workspace_id, external_id, name, domain, custom_attributes, created_at, updated_at)
VALUES (@workspace_id, @external_id, @name, @domain, @custom_attributes, @now, @now)
RETURNING *;

-- name: FindCompany :one
-- FindCompany returns the company with the id in the workspace, deleted
-- or not.
SELECT * FROM companies WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindCompanyByExternalID :one
-- FindCompanyByExternalID returns the live company the host app knows by
-- the id.
SELECT * FROM companies
WHERE workspace_id = @workspace_id AND external_id = @external_id AND deleted_at IS NULL;

-- name: FindCompanyByDomain :one
-- FindCompanyByDomain returns the oldest live company with the domain,
-- the match an upsert falls back to when no external id matches. The
-- caller normalizes the domain first.
SELECT * FROM companies
WHERE workspace_id = @workspace_id AND domain = @domain AND deleted_at IS NULL
ORDER BY created_at, id
LIMIT 1;

-- name: ListCompanies :many
-- ListCompanies returns a page of the workspace's live companies, newest
-- first, narrowed to those carrying every tag given and whose name or
-- domain contains the search when one is given. A page after the first
-- passes the created_at and id of the last company it saw.
SELECT c.* FROM companies AS c
WHERE c.workspace_id = @workspace_id
  AND c.deleted_at IS NULL
  AND (
      sqlc.narg(search)::text IS NULL
      OR strpos(lower(c.name), lower(sqlc.narg(search)::text)) > 0
      OR strpos(lower(coalesce(c.domain::text, '')), lower(sqlc.narg(search)::text)) > 0
  )
  AND (
      coalesce(cardinality(@tag_ids::uuid[]), 0) = 0
      OR c.id IN (
          SELECT ct.company_id FROM company_tag AS ct
          WHERE ct.workspace_id = @workspace_id AND ct.tag_id = ANY (@tag_ids::uuid[])
          GROUP BY ct.company_id
          HAVING count(*) = cardinality(@tag_ids::uuid[])
      )
  )
  AND (
      sqlc.narg(before_id)::uuid IS NULL
      OR (c.created_at, c.id) < (sqlc.narg(before_created_at)::timestamptz, sqlc.narg(before_id)::uuid)
  )
ORDER BY c.created_at DESC, c.id DESC
LIMIT @row_limit::bigint;

-- name: UpdateCompany :one
-- UpdateCompany replaces a live company's attributes. A deleted company
-- is not found.
UPDATE companies
SET external_id = @external_id, name = @name, domain = @domain, custom_attributes = @custom_attributes,
    notes = sqlc.narg(notes), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteCompany :execrows
-- DeleteCompany moves a live company to the trash and reports whether it
-- was there to delete. Its contacts are detached separately.
UPDATE companies SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: DetachCompanyContacts :execrows
-- DetachCompanyContacts clears the company of every contact that works
-- for it, as deleting the company does, and returns how many changed.
UPDATE contacts SET company_id = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND company_id = @company_id::uuid;

-- name: ListCompanyContacts :many
-- ListCompanyContacts returns a page of the live contacts that work for
-- the company, newest first. A page after the first passes the
-- created_at and id of the last contact it saw.
SELECT * FROM contacts
WHERE workspace_id = @workspace_id
  AND company_id = @company_id::uuid
  AND deleted_at IS NULL
  AND merged_into_id IS NULL
  AND (
      sqlc.narg(before_id)::uuid IS NULL
      OR (created_at, id) < (sqlc.narg(before_created_at)::timestamptz, sqlc.narg(before_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT @row_limit::bigint;

-- name: CompaniesByIDs :many
-- CompaniesByIDs returns the live companies of the workspace with the ids.
SELECT * FROM companies
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;

-- name: LockCompany :one
-- LockCompany returns the live company and holds its row until the
-- transaction ends.
SELECT * FROM companies
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
FOR UPDATE;
