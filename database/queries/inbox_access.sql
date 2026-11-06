-- name: ListHiddenInboxIDs :many
-- ListHiddenInboxIDs returns the restricted inboxes the member may not
-- open: neither they nor a team they are in is on the inbox's list.
SELECT i.id FROM inboxes AS i
WHERE i.workspace_id = @workspace_id AND i.access = 'restricted' AND i.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM inbox_access AS a
      WHERE a.workspace_id = i.workspace_id AND a.inbox_id = i.id
        AND (a.member_id = @member_id::uuid OR a.team_id IN (
            SELECT tm.team_id FROM team_members AS tm
            WHERE tm.workspace_id = i.workspace_id AND tm.workspace_member_id = @member_id::uuid
        ))
  )
ORDER BY i.id;

-- name: ListHiddenInboxIDsOfMembers :many
-- ListHiddenInboxIDsOfMembers returns, for each of the members, the
-- restricted inboxes they may not open.
SELECT m.id AS member_id, i.id AS inbox_id
FROM workspace_members AS m
JOIN inboxes AS i ON i.workspace_id = m.workspace_id AND i.access = 'restricted' AND i.deleted_at IS NULL
WHERE m.workspace_id = @workspace_id AND m.id = ANY (@member_ids::uuid[])
  AND NOT EXISTS (
      SELECT 1 FROM inbox_access AS a
      WHERE a.workspace_id = i.workspace_id AND a.inbox_id = i.id
        AND (a.member_id = m.id OR a.team_id IN (
            SELECT tm.team_id FROM team_members AS tm
            WHERE tm.workspace_id = m.workspace_id AND tm.workspace_member_id = m.id
        ))
  )
ORDER BY m.id, i.id;

-- name: HasRestrictedInbox :one
-- HasRestrictedInbox reports whether any live inbox of the workspace is
-- restricted.
SELECT EXISTS (
    SELECT 1 FROM inboxes WHERE workspace_id = @workspace_id AND access = 'restricted' AND deleted_at IS NULL
)::boolean;

-- name: ListInboxAccess :many
-- ListInboxAccess returns the teams and teammates on the lists of the
-- workspace's inboxes.
SELECT * FROM inbox_access WHERE workspace_id = @workspace_id ORDER BY inbox_id, created_at, team_id, member_id;

-- name: ListInboxAccessOf :many
-- ListInboxAccessOf returns the teams and teammates on the inbox's list.
SELECT * FROM inbox_access WHERE workspace_id = @workspace_id AND inbox_id = @inbox_id
ORDER BY created_at, team_id, member_id;

-- name: SetInboxAccessMode :one
-- SetInboxAccessMode opens the inbox to everyone or restricts it to its
-- list.
UPDATE inboxes SET access = @access, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ClearInboxAccess :exec
-- ClearInboxAccess empties the inbox's list.
DELETE FROM inbox_access WHERE workspace_id = @workspace_id AND inbox_id = @inbox_id;

-- name: AddInboxAccessTeams :exec
-- AddInboxAccessTeams puts the teams on the inbox's list.
INSERT INTO inbox_access (workspace_id, inbox_id, team_id, created_at)
SELECT @workspace_id, @inbox_id, team_id, @now FROM unnest(@team_ids::uuid[]) AS team_id;

-- name: AddInboxAccessMembers :exec
-- AddInboxAccessMembers puts the teammates on the inbox's list.
INSERT INTO inbox_access (workspace_id, inbox_id, member_id, created_at)
SELECT @workspace_id, @inbox_id, member_id, @now FROM unnest(@member_ids::uuid[]) AS member_id;

-- name: MentionsMember :one
-- MentionsMember reports whether a note in the conversation mentions the
-- member.
SELECT EXISTS (
    SELECT 1 FROM message_mentions
    WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND workspace_member_id = @member_id
)::boolean;
