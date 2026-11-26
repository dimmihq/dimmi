-- name: CreateImportedWorkspaceMember :one
-- CreateImportedWorkspaceMember adds a teammate brought over from another
-- tool: deactivated, waiting to be enabled, with the role and teams
-- enabling gives them.
INSERT INTO workspace_members (
    workspace_id, user_id, role, status, joined_via, import_source, import_role, import_team_ids,
    import_pending, created_at, updated_at
)
VALUES (
    @workspace_id, @user_id, @role, 'deactivated', 'import', @import_source, @role, @team_ids::uuid[],
    true, @now, @now
)
RETURNING *;

-- name: ListUserAwaitingMemberships :many
-- ListUserAwaitingMemberships returns the live workspaces where the user
-- was imported and waits to be enabled, oldest first. It runs before any
-- workspace is chosen, through a security definer function.
SELECT
    m.member_id::uuid AS member_id,
    m.workspace_id::uuid AS workspace_id,
    m.workspace_name::text AS workspace_name
FROM auth_user_awaiting_memberships(@user_id) AS m;

-- name: MarkImportArrived :many
-- MarkImportArrived records that the user, imported into the workspace,
-- signed up and now waits, the first time only, and returns the
-- memberships it marked.
UPDATE workspace_members
SET import_arrived_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND user_id = @user_id AND import_pending AND status = 'deactivated'
    AND import_arrived_at IS NULL
RETURNING *;

-- name: MarkImportInvited :exec
-- MarkImportInvited records that the imported member was invited to
-- claim their account.
UPDATE workspace_members SET import_invited_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListActiveManagers :many
-- ListActiveManagers returns the workspace's active owners and admins
-- with their accounts, oldest membership first.
SELECT sqlc.embed(m), sqlc.embed(u)
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.role IN ('owner', 'admin') AND m.status = 'active'
ORDER BY m.created_at, m.id;
