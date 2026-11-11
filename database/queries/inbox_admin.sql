-- name: UpdateInboxDetails :one
-- UpdateInboxDetails renames the inbox, sets its default team and
-- replaces its settings.
UPDATE inboxes
SET name = @name, default_team_id = sqlc.narg(default_team_id), settings = @settings, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetInboxSchedule :one
-- SetInboxSchedule names the schedule the inbox keeps its office hours
-- in, or none to follow the workspace's default.
UPDATE inboxes SET schedule_id = sqlc.narg(schedule_id), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CountInboxConversations :one
-- CountInboxConversations counts every conversation the inbox holds,
-- in the trash or not.
SELECT count(*)::integer FROM conversations WHERE workspace_id = @workspace_id AND inbox_id = @inbox_id;

-- name: MoveInboxConversations :execrows
-- MoveInboxConversations moves every conversation of one inbox to
-- another.
UPDATE conversations SET inbox_id = @to_inbox_id, updated_at = @now
WHERE workspace_id = @workspace_id AND inbox_id = @from_inbox_id;

-- name: MoveInboxConnections :exec
-- MoveInboxConnections moves every channel connection of one inbox to
-- another, so what arrives through them lands there.
UPDATE channel_connections SET inbox_id = @to_inbox_id, updated_at = @now
WHERE workspace_id = @workspace_id AND inbox_id = @from_inbox_id;

-- name: DeleteInbox :exec
-- DeleteInbox takes the inbox out of use; conversations keep pointing
-- at it for their history.
UPDATE inboxes SET deleted_at = @now, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;
