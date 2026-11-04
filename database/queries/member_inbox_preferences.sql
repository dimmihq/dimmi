-- name: MemberInboxPreferences :one
-- MemberInboxPreferences returns the member's inbox preferences.
SELECT * FROM member_inbox_preferences WHERE workspace_id = @workspace_id AND member_id = @member_id;

-- name: SaveMemberInboxPreferences :one
-- SaveMemberInboxPreferences writes the member's inbox preferences whole,
-- creating them on first save.
INSERT INTO member_inbox_preferences (workspace_id, member_id, sidebar, views, pinned_attributes, updated_at)
VALUES (@workspace_id, @member_id, @sidebar, @views, @pinned_attributes, @now)
ON CONFLICT (member_id) DO UPDATE
SET sidebar = excluded.sidebar, views = excluded.views, pinned_attributes = excluded.pinned_attributes,
    updated_at = excluded.updated_at
RETURNING *;

-- name: LiveViewTargets :many
-- LiveViewTargets returns those of the ids that still name a team, an
-- active member or an inbox of the workspace, the targets a saved view
-- preference may point at.
SELECT t.id FROM teams t
WHERE t.workspace_id = @workspace_id::uuid AND t.id = ANY (@ids::uuid[]) AND t.deleted_at IS NULL
UNION ALL
SELECT m.id FROM workspace_members m
WHERE m.workspace_id = @workspace_id::uuid AND m.id = ANY (@ids::uuid[]) AND m.status = 'active'
UNION ALL
SELECT i.id FROM inboxes i
WHERE i.workspace_id = @workspace_id::uuid AND i.id = ANY (@ids::uuid[]) AND i.deleted_at IS NULL;

-- name: ActiveMembersOfUsers :many
-- ActiveMembersOfUsers returns the active members of the workspace that
-- the users are, with their names, the people a presence list names.
SELECT m.id, m.user_id, u.name
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.user_id = ANY (@user_ids::uuid[]) AND m.status = 'active'
ORDER BY u.name, m.id;
