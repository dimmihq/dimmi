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

-- name: FindDefaultInbox :one
-- FindDefaultInbox returns the live inbox the workspace's settings name
-- as its default, where conversations land when nothing routes them
-- elsewhere.
SELECT i.* FROM inboxes AS i
JOIN workspaces AS w ON w.id = i.workspace_id
WHERE i.workspace_id = @workspace_id
  AND i.deleted_at IS NULL
  AND i.id::text = w.settings ->> 'default_inbox_id';

-- name: SetDefaultInbox :exec
-- SetDefaultInbox makes the inbox the workspace's default, keeping every
-- other setting.
UPDATE workspaces
SET settings = jsonb_set(settings, '{default_inbox_id}', to_jsonb(@inbox_id::uuid)), updated_at = @now
WHERE id = @workspace_id;
