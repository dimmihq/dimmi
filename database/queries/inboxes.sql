-- name: CreateInbox :one
-- CreateInbox stores an inbox in the workspace.
INSERT INTO inboxes (workspace_id, name, default_team_id, settings, created_at, updated_at)
VALUES (@workspace_id, @name, @default_team_id, @settings, @now, @now)
RETURNING *;

-- name: FindInbox :one
-- FindInbox returns the inbox with the id in the workspace, deleted or
-- not.
SELECT * FROM inboxes WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListInboxes :many
-- ListInboxes returns the workspace's live inboxes, oldest first.
SELECT * FROM inboxes WHERE workspace_id = @workspace_id AND deleted_at IS NULL ORDER BY created_at, id;
