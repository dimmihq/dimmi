-- name: CountWorkspaceProcedures :one
-- CountWorkspaceProcedures counts how many of some ids are procedures of
-- the workspace.
SELECT count(*)::integer FROM procedures
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;

-- name: CountWorkspaceActions :one
-- CountWorkspaceActions counts how many of some ids are actions of the
-- workspace.
SELECT count(*)::integer FROM ai_actions
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;

-- name: CountWorkspaceKnowledgeSources :one
-- CountWorkspaceKnowledgeSources counts how many of some ids are
-- knowledge sources of the workspace.
SELECT count(*)::integer FROM knowledge_sources
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: CountWorkspaceHelpCenters :one
-- CountWorkspaceHelpCenters counts how many of some ids are help centers
-- of the workspace.
SELECT count(*)::integer FROM help_centers
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;

-- name: CountWorkspaceConnections :one
-- CountWorkspaceConnections counts how many of some ids are channel
-- connections of the workspace.
SELECT count(*)::integer FROM channel_connections
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;

-- name: CountWorkspaceRooms :one
-- CountWorkspaceRooms counts how many of some ids are Slack channels of
-- the workspace.
SELECT count(*)::integer FROM channel_rooms
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]);

-- name: CountWorkspaceInboxes :one
-- CountWorkspaceInboxes counts how many of some ids are inboxes of the
-- workspace.
SELECT count(*)::integer FROM inboxes
WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;
