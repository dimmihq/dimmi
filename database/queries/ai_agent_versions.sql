-- name: CreateAgentVersion :one
-- CreateAgentVersion keeps a published copy of an agent, numbered after
-- the last.
INSERT INTO ai_agent_versions (
    workspace_id, agent_id, version, definition, change_note, published_by_member_id, published_at
) VALUES (
    @workspace_id, @agent_id,
    (SELECT COALESCE(max(version), 0) + 1 FROM ai_agent_versions WHERE agent_id = @agent_id),
    @definition, sqlc.narg(change_note), sqlc.narg(member_id), @now
)
RETURNING *;

-- name: ListAgentVersions :many
-- ListAgentVersions returns an agent's versions, newest first, below a
-- version when one is given.
SELECT * FROM ai_agent_versions
WHERE workspace_id = @workspace_id AND agent_id = @agent_id
    AND (@before::integer = 0 OR version < @before)
ORDER BY version DESC
LIMIT @row_limit;

-- name: FindAgentVersion :one
-- FindAgentVersion returns one version of an agent by number.
SELECT * FROM ai_agent_versions
WHERE workspace_id = @workspace_id AND agent_id = @agent_id AND version = @version;

-- name: FindAgentVersionByID :one
-- FindAgentVersionByID returns one version by id.
SELECT * FROM ai_agent_versions WHERE workspace_id = @workspace_id AND id = @id;
