-- name: CountOpenConversationsByTeam :many
-- CountOpenConversationsByTeam counts the open conversations of each team
-- that holds any, among those the reader may see.
SELECT assignee_team_id::uuid AS team_id, count(*) AS open
FROM conversations
WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND NOT spam AND status = 'open' AND assignee_team_id IS NOT NULL
  AND dimmi_conversation_visible(
      assignee_member_id, assignee_team_id, @access_mode::text, sqlc.narg(access_viewer)::uuid,
      @access_teams::uuid[], @access_unassigned::boolean
  )
GROUP BY assignee_team_id;

-- name: CountOpenConversationsByAssignee :many
-- CountOpenConversationsByAssignee counts the open conversations each
-- member holds, among those the reader may see.
SELECT assignee_member_id::uuid AS member_id, count(*) AS open
FROM conversations
WHERE workspace_id = @workspace_id AND deleted_at IS NULL AND NOT spam AND status = 'open'
  AND assignee_member_id IS NOT NULL
  AND dimmi_conversation_visible(
      assignee_member_id, assignee_team_id, @access_mode::text, sqlc.narg(access_viewer)::uuid,
      @access_teams::uuid[], @access_unassigned::boolean
  )
GROUP BY assignee_member_id;

-- name: ListConversationViews :many
-- ListConversationViews returns the workspace's shared views in their
-- order and the member's own views by name, optionally only those whose
-- name holds the search.
SELECT * FROM conversation_views
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
  AND (scope = 'shared' OR owner_member_id = @member_id::uuid)
  AND (@search::text = '' OR name ILIKE '%' || @search::text || '%')
ORDER BY scope DESC, position, lower(name), id;

-- name: FindConversationView :one
-- FindConversationView returns a live view the member may see: shared,
-- or their own.
SELECT * FROM conversation_views
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
  AND (scope = 'shared' OR owner_member_id = @member_id::uuid);

-- name: CountConversationViews :one
-- CountConversationViews counts the live views of a scope and owner.
SELECT count(*) FROM conversation_views
WHERE workspace_id = @workspace_id AND scope = @scope
  AND owner_member_id IS NOT DISTINCT FROM sqlc.narg(owner_member_id)::uuid AND deleted_at IS NULL;

-- name: CreateConversationView :one
-- CreateConversationView saves a new view, shared views at the end of the
-- order.
INSERT INTO conversation_views (
    workspace_id, name, icon, scope, owner_member_id, definition, sort, status_filter, created_by_member_id,
    position, created_at, updated_at
)
VALUES (
    @workspace_id, @name, sqlc.narg(icon), @scope, sqlc.narg(owner_member_id), @definition, sqlc.narg(sort),
    sqlc.narg(status_filter), @created_by_member_id,
    (SELECT coalesce(max(v.position) + 1, 0) FROM conversation_views v
     WHERE v.workspace_id = @workspace_id AND v.scope = @scope AND v.deleted_at IS NULL),
    @now, @now
)
RETURNING *;

-- name: UpdateConversationView :one
-- UpdateConversationView writes a view's changed fields.
UPDATE conversation_views
SET name = @name, icon = sqlc.narg(icon), definition = @definition, sort = sqlc.narg(sort),
    status_filter = sqlc.narg(status_filter), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteConversationView :exec
-- DeleteConversationView moves a view to the trash.
UPDATE conversation_views SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: MoveSharedConversationView :exec
-- MoveSharedConversationView puts a shared view at a place in the order.
UPDATE conversation_views SET position = @position, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND scope = 'shared' AND deleted_at IS NULL;

-- name: LiveReferences :many
-- LiveReferences returns those of the ids that still name a live tag,
-- team, inbox, channel connection, member, contact or company of the
-- workspace, for warning about views whose conditions name deleted rows.
SELECT t.id FROM teams t
WHERE t.workspace_id = @workspace_id::uuid AND t.id = ANY (@ids::uuid[]) AND t.deleted_at IS NULL
UNION ALL
SELECT i.id FROM inboxes i
WHERE i.workspace_id = @workspace_id::uuid AND i.id = ANY (@ids::uuid[]) AND i.deleted_at IS NULL
UNION ALL
SELECT g.id FROM tags g WHERE g.workspace_id = @workspace_id::uuid AND g.id = ANY (@ids::uuid[])
UNION ALL
SELECT cc.id FROM channel_connections cc
WHERE cc.workspace_id = @workspace_id::uuid AND cc.id = ANY (@ids::uuid[]) AND cc.deleted_at IS NULL
UNION ALL
SELECT m.id FROM workspace_members m
WHERE m.workspace_id = @workspace_id::uuid AND m.id = ANY (@ids::uuid[]) AND m.status = 'active'
UNION ALL
SELECT ct.id FROM contacts ct
WHERE ct.workspace_id = @workspace_id::uuid AND ct.id = ANY (@ids::uuid[]) AND ct.deleted_at IS NULL
UNION ALL
SELECT co.id FROM companies co
WHERE co.workspace_id = @workspace_id::uuid AND co.id = ANY (@ids::uuid[]) AND co.deleted_at IS NULL;
