-- name: CreateOrganization :one
-- CreateOrganization stores the organization a new workspace belongs to.
INSERT INTO organizations (name, created_by_user_id, created_at, updated_at)
VALUES (@name, @created_by_user_id, @now, @now)
RETURNING *;
