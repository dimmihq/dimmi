-- name: CreateSSOConnection :one
-- CreateSSOConnection stores a draft connection of the organization.
INSERT INTO sso_connections (organization_id, protocol, name, config, secret_encrypted, created_by_user_id, created_at, updated_at)
VALUES (@organization_id, @protocol, @name, @config, @secret_encrypted, @created_by_user_id, @now, @now)
RETURNING *;

-- name: CountSSOConnections :one
-- CountSSOConnections counts the organization's connections, to keep
-- them under the limit.
SELECT count(*) FROM sso_connections WHERE organization_id = @organization_id;

-- name: ListSSOConnections :many
-- ListSSOConnections returns the organization's connections, oldest first.
SELECT * FROM sso_connections WHERE organization_id = @organization_id ORDER BY created_at, id;

-- name: FindSSOConnection :one
-- FindSSOConnection returns the organization's connection with the id.
SELECT * FROM sso_connections WHERE organization_id = @organization_id AND id = @id;

-- name: LockSSOConnection :one
-- LockSSOConnection returns the organization's connection and holds its
-- row until the transaction ends.
SELECT * FROM sso_connections WHERE organization_id = @organization_id AND id = @id FOR UPDATE;

-- name: UpdateSSOConnection :one
-- UpdateSSOConnection writes every changeable field of the connection.
UPDATE sso_connections
SET name = @name, status = @status, config = @config, secret_encrypted = @secret_encrypted,
    jit_enabled = @jit_enabled, default_role = @default_role, default_custom_role_id = sqlc.narg('default_custom_role_id'),
    default_team_ids = @default_team_ids::uuid[],
    group_role_map = @group_role_map, enforced = @enforced, tested_at = sqlc.narg('tested_at'), updated_at = @now
WHERE organization_id = @organization_id AND id = @id
RETURNING *;

-- name: DeleteSSOConnection :execrows
-- DeleteSSOConnection removes the connection.
DELETE FROM sso_connections WHERE organization_id = @organization_id AND id = @id;

-- name: ListActiveSSOConnectionIDs :many
-- ListActiveSSOConnectionIDs returns the organization's active
-- connections and whether each is enforced, mirrored into its
-- workspaces' rules.
SELECT id, enforced FROM sso_connections WHERE organization_id = @organization_id AND status = 'active'
ORDER BY created_at, id;

-- name: ListOrganizationWorkspaces :many
-- ListOrganizationWorkspaces returns the organization's live workspaces,
-- locked, whose rules mirror its connections.
SELECT * FROM workspaces WHERE organization_id = @organization_id AND deleted_at IS NULL ORDER BY id FOR UPDATE;

-- name: SetWorkspaceSettings :exec
-- SetWorkspaceSettings replaces the workspace's settings document.
UPDATE workspaces SET settings = @settings, updated_at = @now WHERE id = @id;

-- name: ListOrganizationSSOConnectionsBySlug :many
-- ListOrganizationSSOConnectionsBySlug returns the connections of the
-- organization behind the workspace with the slug, for the operator.
SELECT c.*, w.id AS workspace_id
FROM sso_connections c
JOIN workspaces w ON w.organization_id = c.organization_id AND w.deleted_at IS NULL
WHERE w.slug = @slug
ORDER BY c.created_at, c.id;

-- name: DisableSSOConnections :many
-- DisableSSOConnections stops every connection of the organization from
-- signing anyone in, returning those it changed.
UPDATE sso_connections SET status = 'disabled', enforced = false, updated_at = @now
WHERE organization_id = @organization_id AND status = 'active'
RETURNING *;
