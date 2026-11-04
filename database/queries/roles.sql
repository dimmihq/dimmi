-- name: CreateRole :one
-- CreateRole adds a custom role to the workspace.
INSERT INTO roles (workspace_id, name, description, base_role, permissions, created_by_member_id, created_at, updated_at)
VALUES (@workspace_id, @name, sqlc.narg('description'), @base_role, @permissions::text[], @created_by_member_id, @now, @now)
RETURNING *;

-- name: CountRoles :one
-- CountRoles counts the workspace's custom roles, to keep them under the
-- limit.
SELECT count(*) FROM roles WHERE workspace_id = @workspace_id;

-- name: ListRoles :many
-- ListRoles returns the workspace's custom roles by name, each with how
-- many members hold it.
SELECT sqlc.embed(r),
    (SELECT count(*) FROM workspace_members m
     WHERE m.workspace_id = r.workspace_id AND m.custom_role_id = r.id)::bigint AS member_count
FROM roles r
WHERE r.workspace_id = @workspace_id
ORDER BY lower(r.name), r.id;

-- name: FindRole :one
-- FindRole returns the custom role with the id in the workspace, with how
-- many members hold it.
SELECT sqlc.embed(r),
    (SELECT count(*) FROM workspace_members m
     WHERE m.workspace_id = r.workspace_id AND m.custom_role_id = r.id)::bigint AS member_count
FROM roles r
WHERE r.workspace_id = @workspace_id AND r.id = @id;

-- name: LockRole :one
-- LockRole returns the custom role and holds its row until the
-- transaction ends, so it cannot be deleted while it is assigned.
SELECT * FROM roles WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: ListRolesByIDs :many
-- ListRolesByIDs returns which of the ids name custom roles of the
-- workspace, to show the role each listed member holds.
SELECT id, name FROM roles WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: FindRolePermissions :one
-- FindRolePermissions returns what the custom role lists, read on every
-- request of a member who holds it.
SELECT permissions FROM roles WHERE workspace_id = @workspace_id AND id = @id;

-- name: UpdateRole :one
-- UpdateRole changes the custom role's name, description and permissions.
UPDATE roles
SET name = @name, description = sqlc.narg('description'), permissions = @permissions::text[], updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeleteRole :execrows
-- DeleteRole removes the custom role. The database refuses while a member
-- holds it.
DELETE FROM roles WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetMemberCustomRole :one
-- SetMemberCustomRole gives the member the custom role and the built-in
-- role it narrows, or clears the custom role when none is given.
UPDATE workspace_members
SET role = @role, custom_role_id = sqlc.narg('custom_role_id'), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ListRoleHolderUserIDs :many
-- ListRoleHolderUserIDs returns the account of every member holding the
-- custom role, whose open connections a change to it refreshes.
SELECT user_id FROM workspace_members WHERE workspace_id = @workspace_id AND custom_role_id = @id;

-- name: SetInvitationCustomRole :exec
-- SetInvitationCustomRole names the custom role the invitee starts with,
-- or none.
UPDATE invitations SET custom_role_id = sqlc.narg('custom_role_id')
WHERE workspace_id = @workspace_id AND id = @id;
