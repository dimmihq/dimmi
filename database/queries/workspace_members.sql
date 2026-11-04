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

-- name: ListWorkspaceMembers :many
-- ListWorkspaceMembers returns a page of the workspace's members with
-- their accounts, narrowed to the statuses and roles asked for (none
-- means all), to members of any of the teams asked for, to accounts
-- with or without a second factor when asked, and to names or emails
-- containing the search. They come by name, newest first, or most
-- recently active first with those never active last.
SELECT sqlc.embed(m), sqlc.embed(u)
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id
  AND (coalesce(cardinality(@statuses::text[]), 0) = 0 OR m.status = ANY (@statuses::text[]))
  AND (coalesce(cardinality(@roles::text[]), 0) = 0 OR m.role = ANY (@roles::text[]))
  AND (coalesce(cardinality(@team_ids::uuid[]), 0) = 0 OR EXISTS (
      SELECT 1 FROM team_members tm
      WHERE tm.workspace_member_id = m.id AND tm.team_id = ANY (@team_ids::uuid[])
  ))
  AND (sqlc.narg(two_factor)::boolean IS NULL
      OR (u.two_factor_confirmed_at IS NOT NULL) = sqlc.narg(two_factor)::boolean)
  AND (@search::text = '' OR u.name ILIKE '%' || @search::text || '%' OR u.email ILIKE '%' || @search::text || '%')
ORDER BY
    CASE WHEN @sort::text = 'created_at' THEN m.created_at END DESC,
    CASE WHEN @sort::text = 'last_active_at' THEN m.last_accessed_at END DESC NULLS LAST,
    lower(u.name),
    m.id
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: FindWorkspaceMemberWithUser :one
-- FindWorkspaceMemberWithUser returns the member with the id in the
-- workspace, with their account.
SELECT sqlc.embed(m), sqlc.embed(u)
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.id = @id;

-- name: LockWorkspaceMember :one
-- LockWorkspaceMember returns the member and holds the row until the
-- transaction ends.
SELECT * FROM workspace_members WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: CountActiveOwners :one
-- CountActiveOwners counts the workspace's active owners, the people who
-- can still run it.
SELECT count(*) FROM workspace_members WHERE workspace_id = @workspace_id AND role = 'owner' AND status = 'active';

-- name: UpdateWorkspaceMemberRole :one
-- UpdateWorkspaceMemberRole gives the member a new role.
UPDATE workspace_members SET role = @role, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DeactivateWorkspaceMember :one
-- DeactivateWorkspaceMember ends the member's access to the workspace and
-- records who ended it, keeping the row so their history stays theirs.
UPDATE workspace_members
SET status = 'deactivated', deactivated_at = @now::timestamptz, deactivated_by_member_id = @deactivated_by::uuid,
    updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ReactivateWorkspaceMember :one
-- ReactivateWorkspaceMember gives the member their access back with the
-- role.
UPDATE workspace_members
SET status = 'active', role = @role, deactivated_at = NULL, deactivated_by_member_id = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchWorkspaceMemberAccess :exec
-- TouchWorkspaceMemberAccess records that the member opened the
-- workspace, at most once per interval: a member who used it since the
-- given time is left alone, so busy members cost no write per request.
UPDATE workspace_members SET last_accessed_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND (last_accessed_at IS NULL OR last_accessed_at < @since::timestamptz);

-- name: ListUserMemberships :many
-- ListUserMemberships returns the user's active memberships, the one used
-- most recently first, through a security definer function since they
-- span workspaces. A membership never opened says so in accessed, and
-- its last_accessed_at is then when it began. Workspaces in the trash are left out unless trashed
-- asks for them ("with" adds them, "only" lists nothing else), and only
-- owners ever see one.
SELECT
    m.member_id::uuid AS member_id,
    m.role::text AS role,
    m.status::text AS status,
    coalesce(m.joined_via, '')::text AS joined_via,
    (m.last_accessed_at IS NOT NULL)::boolean AS accessed,
    coalesce(m.last_accessed_at, m.created_at)::timestamptz AS last_accessed_at,
    sqlc.embed(w)
FROM auth_list_user_memberships(@user_id) AS m
JOIN workspaces w ON w.id = m.workspace_id::uuid
WHERE m.status = 'active'
  AND CASE @trashed::text
      WHEN 'with' THEN w.deleted_at IS NULL OR m.role = 'owner'
      WHEN 'only' THEN w.deleted_at IS NOT NULL AND m.role = 'owner'
      ELSE w.deleted_at IS NULL
  END
ORDER BY accessed DESC, last_accessed_at DESC, w.created_at;

-- name: FindUserMembership :one
-- FindUserMembership returns one of the user's own memberships, in any
-- workspace and any state, through a security definer function since the
-- workspace is not known yet.
SELECT
    m.member_id::uuid AS member_id,
    m.workspace_id::uuid AS workspace_id,
    m.status::text AS status
FROM auth_list_user_memberships(@user_id) AS m
WHERE m.member_id = @member_id::uuid;

-- name: ListActiveOwners :many
-- ListActiveOwners returns the workspace's active owners with their
-- accounts, oldest membership first.
SELECT sqlc.embed(m), sqlc.embed(u)
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.role = 'owner' AND m.status = 'active'
ORDER BY m.created_at, m.id;

-- name: FindOldestActiveMember :one
-- FindOldestActiveMember returns the workspace's longest-standing active
-- member of the role.
SELECT * FROM workspace_members
WHERE workspace_id = @workspace_id AND role = @role AND status = 'active'
ORDER BY created_at, id
LIMIT 1;

-- name: FindWorkspaceMemberByEmail :one
-- FindWorkspaceMemberByEmail returns the membership of the workspace held
-- by the account with the address, active or not.
SELECT m.*
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND u.email = @email::citext;

-- name: PurgeWorkspaceRows :one
-- PurgeWorkspaceRows deletes one batch of the workspace's rows from one
-- of its tables and returns how many went; nothing left returns zero.
SELECT dimmi_purge_workspace_rows(@workspace_id::uuid, @batch_size::integer)::bigint AS removed;

-- name: ListUserActiveMemberships :many
-- ListUserActiveMemberships returns every workspace the user is an active
-- member of, those in the trash included. It runs before any workspace is
-- chosen, through a security definer function.
SELECT
    m.member_id::uuid AS member_id,
    m.workspace_id::uuid AS workspace_id,
    m.workspace_name::text AS workspace_name,
    m.workspace_slug::citext AS workspace_slug,
    m.role::text AS role,
    (m.deleted_at IS NOT NULL)::boolean AS trashed
FROM auth_user_memberships(@user_id) AS m
WHERE m.status = 'active';

-- name: CountActiveMembers :one
-- CountActiveMembers counts the people who can still work in the workspace.
SELECT count(*) FROM workspace_members WHERE workspace_id = @workspace_id AND status = 'active';

-- name: CountUserMembershipsOutsideOrganization :one
-- CountUserMembershipsOutsideOrganization counts the user's active
-- memberships in workspaces of other organizations, those in the trash
-- included, through a security definer function since they span
-- workspaces.
SELECT count(*)
FROM auth_list_user_memberships(@user_id) AS m
JOIN workspaces w ON w.id = m.workspace_id
WHERE m.status = 'active' AND w.organization_id <> @organization_id::uuid;
