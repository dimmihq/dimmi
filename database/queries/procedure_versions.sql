-- name: CreateProcedureVersion :one
-- CreateProcedureVersion keeps a published copy, numbered after the last.
INSERT INTO procedure_versions (
    workspace_id, procedure_id, version, name, definition, change_note, published_by_member_id, published_at
) VALUES (
    @workspace_id, @procedure_id,
    (SELECT COALESCE(max(version), 0) + 1 FROM procedure_versions WHERE procedure_id = @procedure_id),
    @name, @definition, sqlc.narg(change_note), sqlc.narg(member_id), @now
)
RETURNING *;

-- name: ListProcedureVersions :many
-- ListProcedureVersions returns a procedure's versions, newest first,
-- below a version when one is given.
SELECT * FROM procedure_versions
WHERE workspace_id = @workspace_id AND procedure_id = @procedure_id
    AND (@before::integer = 0 OR version < @before)
ORDER BY version DESC
LIMIT @row_limit;

-- name: FindProcedureVersion :one
-- FindProcedureVersion returns one version of a procedure by number.
SELECT * FROM procedure_versions
WHERE workspace_id = @workspace_id AND procedure_id = @procedure_id AND version = @version;

-- name: FindProcedureVersionByID :one
-- FindProcedureVersionByID returns one version by id.
SELECT * FROM procedure_versions WHERE workspace_id = @workspace_id AND id = @id;

-- name: ProcedureVersionStats :many
-- ProcedureVersionStats counts, for each version of a procedure, the
-- conversations that followed it since a moment and how many of them
-- were handed to the team.
SELECT (r.procedure_trace->>'version')::integer AS version,
    count(DISTINCT r.conversation_id)::integer AS conversations,
    count(DISTINCT r.conversation_id) FILTER (WHERE r.outcome = 'handoff')::integer AS handed_off
FROM ai_runs r
WHERE r.workspace_id = @workspace_id AND r.kind = 'conversation' AND r.created_at >= @since
    AND ('procedure:' || (@procedure_id::uuid)::text) = ANY (r.flags) AND r.procedure_trace ? 'version'
GROUP BY 1;
