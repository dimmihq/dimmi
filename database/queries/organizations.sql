-- name: CreateOrganization :one
-- CreateOrganization stores the organization a new workspace belongs to.
INSERT INTO organizations (name, created_by_user_id, created_at, updated_at)
VALUES (@name, @created_by_user_id, @now, @now)
RETURNING *;

-- name: RenameOrganization :exec
-- RenameOrganization gives the organization the name of its workspace
-- while the workspace is its only one; an organization of several
-- workspaces keeps a name of its own.
UPDATE organizations o
SET name = @name, updated_at = @now
WHERE o.id = @id AND (SELECT count(*) FROM workspaces w WHERE w.organization_id = o.id) = 1;

-- name: RepairOrganization :execrows
-- RepairOrganization brings back the organization of a live workspace
-- that was deleted by mistake, and gives it the workspace's name while
-- that workspace is its only one. It changes nothing on an organization
-- already in order.
UPDATE organizations o
SET deleted_at = NULL,
    name = CASE WHEN (SELECT count(*) FROM workspaces x WHERE x.organization_id = o.id) = 1 THEN w.name ELSE o.name END,
    updated_at = @now
FROM workspaces w
WHERE w.id = @workspace_id AND o.id = w.organization_id AND w.deleted_at IS NULL
  AND (
    o.deleted_at IS NOT NULL
    OR (o.name <> w.name AND (SELECT count(*) FROM workspaces x WHERE x.organization_id = o.id) = 1)
  );
