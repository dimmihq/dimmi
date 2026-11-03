-- name: ListWorkspacesRequiringTwoFactor :many
-- ListWorkspacesRequiringTwoFactor returns the live workspaces the user
-- is an active member of that require a second factor, by name. It runs
-- outside any workspace, through the security definer function that
-- lists the user's own memberships.
SELECT w.id, w.name
FROM auth_list_user_memberships(@user_id) AS m
JOIN workspaces w ON w.id = m.workspace_id::uuid
WHERE m.status = 'active'
  AND w.deleted_at IS NULL
  AND coalesce((w.settings -> 'security' ->> 'require_two_factor')::boolean, false)
ORDER BY w.name, w.id;

-- name: ListMembersWithoutTwoFactor :many
-- ListMembersWithoutTwoFactor returns the users of the workspace's
-- active members whose accounts have no second factor, oldest member
-- first.
SELECT sqlc.embed(u)
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id
  AND m.status = 'active'
  AND u.deleted_at IS NULL
  AND u.two_factor_confirmed_at IS NULL
ORDER BY m.created_at, m.id;
