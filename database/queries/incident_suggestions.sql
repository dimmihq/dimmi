-- name: ListIncidentSuggestions :many
-- ListIncidentSuggestions returns the workspace's open spike
-- suggestions, newest first.
SELECT * FROM incident_suggestions
WHERE workspace_id = @workspace_id AND status = 'open'
ORDER BY updated_at DESC
LIMIT 20;

-- name: FindIncidentSuggestion :one
-- FindIncidentSuggestion returns one suggestion.
SELECT * FROM incident_suggestions WHERE workspace_id = @workspace_id AND id = @id;

-- name: FindLatestSuggestionBySignature :one
-- FindLatestSuggestionBySignature returns the newest suggestion of a
-- group, open or decided.
SELECT * FROM incident_suggestions WHERE workspace_id = @workspace_id AND signature = @signature
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateIncidentSuggestion :one
-- CreateIncidentSuggestion stores a new spike suggestion.
INSERT INTO incident_suggestions (workspace_id, signature, title, summary, area, conversation_ids, conversation_count,
    window_minutes, created_at, updated_at)
VALUES (@workspace_id, @signature, @title, @summary, sqlc.narg(area), @conversation_ids, @conversation_count,
    @window_minutes, @now, @now)
RETURNING *;

-- name: GrowIncidentSuggestion :one
-- GrowIncidentSuggestion adds the conversations of a spike still going.
UPDATE incident_suggestions SET conversation_ids = @conversation_ids, conversation_count = @conversation_count,
    title = @title, summary = @summary, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: DecideIncidentSuggestion :one
-- DecideIncidentSuggestion accepts or dismisses a suggestion.
UPDATE incident_suggestions SET status = @status, muted_until = sqlc.narg(muted_until), ticket_id = sqlc.narg(ticket_id),
    decided_by_member_id = sqlc.narg(decided_by_member_id), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND status = 'open'
RETURNING *;

-- name: ListRecentWidgetConversations :many
-- ListRecentWidgetConversations returns the conversations started in the
-- window with their first customer words, for spike detection. The
-- system groups them, no member is asking.
SELECT c.id, c.subject, c.created_at,
    (SELECT m.body FROM messages m WHERE m.conversation_id = c.id AND m.author_type = 'contact'
     ORDER BY m.created_at LIMIT 1)::text AS first_message
FROM conversations c
WHERE c.workspace_id = @workspace_id AND c.created_at >= @since AND c.deleted_at IS NULL AND NOT c.spam
ORDER BY c.created_at
LIMIT 500;

-- name: ListWorkspacesWithRecentConversations :many
-- ListWorkspacesWithRecentConversations returns the workspaces where
-- conversations started since the time, for the spike sweep.
SELECT w.workspace_id::uuid AS workspace_id FROM recent_conversation_workspaces(@since::timestamptz) AS w (workspace_id);
