-- name: CreateWorkspaceMember :one
-- CreateWorkspaceMember adds a user to a workspace with a role.
INSERT INTO workspace_members (workspace_id, user_id, role, joined_via, created_at, updated_at)
VALUES (@workspace_id, @user_id, @role, @joined_via, @now, @now)
RETURNING *;

-- name: FindWorkspaceMemberForUser :one
-- FindWorkspaceMemberForUser returns the user's membership of the
-- workspace, active or not.
SELECT * FROM workspace_members WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: FindWorkspaceMember :one
-- FindWorkspaceMember returns the member with the id in the workspace.
SELECT * FROM workspace_members WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListUserWorkspaces :many
-- ListUserWorkspaces returns the live workspaces the user is an active
-- member of, the one used most recently first. It runs before any
-- workspace is chosen, through a security definer function.
SELECT
    m.member_id::uuid AS member_id,
    m.workspace_id::uuid AS workspace_id,
    m.workspace_name::text AS workspace_name,
    m.workspace_slug::citext AS workspace_slug,
    m.role::text AS role
FROM auth_user_memberships(@user_id) AS m
WHERE m.status = 'active' AND m.deleted_at IS NULL;

-- name: UpdateWorkspaceMemberStatus :exec
-- UpdateWorkspaceMemberStatus activates or deactivates a member.
UPDATE workspace_members
SET status = @status,
    deactivated_at = CASE WHEN @status::text = 'deactivated' THEN @now::timestamptz END,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;
