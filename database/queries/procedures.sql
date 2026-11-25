-- name: ListProcedures :many
-- ListProcedures returns the workspace's procedures by name.
SELECT * FROM procedures WHERE workspace_id = @workspace_id AND deleted_at IS NULL ORDER BY lower(name), id;

-- name: ListLiveProcedures :many
-- ListLiveProcedures returns the procedures dimmi may follow.
SELECT p.id, p.name, p.live, p.live_version_id, COALESCE(v.version, 0)::integer AS version
FROM procedures p
LEFT JOIN procedure_versions v ON v.id = p.live_version_id
WHERE p.workspace_id = @workspace_id AND p.status = 'live' AND p.deleted_at IS NULL AND p.live IS NOT NULL
ORDER BY lower(p.name), p.id;

-- name: FindProcedure :one
-- FindProcedure returns one procedure.
SELECT * FROM procedures WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: CreateProcedure :one
-- CreateProcedure stores a new draft procedure.
INSERT INTO procedures (id, workspace_id, name, draft, created_by_member_id, updated_by_member_id, created_at, updated_at)
VALUES (@id, @workspace_id, @name, @draft, @member_id, @member_id, @now, @now)
RETURNING *;

-- name: SaveProcedureDraft :one
-- SaveProcedureDraft saves the working copy when the editor started from
-- the stored lock version, and moves the version on.
UPDATE procedures SET name = @name, draft = @draft, lock_version = lock_version + 1,
    updated_by_member_id = @member_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL AND lock_version = @lock_version
RETURNING *;

-- name: PublishProcedure :one
-- PublishProcedure makes the working copy the live copy.
UPDATE procedures SET live = draft, status = 'live', published_at = @now, updated_by_member_id = @member_id,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: UnpublishProcedure :one
-- UnpublishProcedure takes a procedure out of dimmi's hands, keeping
-- its working copy.
UPDATE procedures SET live = NULL, status = 'draft', updated_by_member_id = @member_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteProcedure :exec
-- DeleteProcedure removes a procedure.
UPDATE procedures SET deleted_at = @now, status = 'draft', live = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: ProcedureNameTaken :one
-- ProcedureNameTaken reports whether another procedure has the name.
SELECT EXISTS (
    SELECT 1 FROM procedures
    WHERE workspace_id = @workspace_id AND lower(name) = lower(@name) AND id <> @id AND deleted_at IS NULL
)::boolean;

-- name: ProcedureUses :many
-- ProcedureUses counts the conversations dimmi followed each procedure
-- in since a moment.
SELECT p.id, count(DISTINCT r.conversation_id)::integer AS conversations
FROM procedures p
JOIN ai_runs r ON r.workspace_id = p.workspace_id
    AND ('procedure:' || p.id::text) = ANY (r.flags) AND r.created_at >= @since
WHERE p.workspace_id = @workspace_id AND p.deleted_at IS NULL
GROUP BY p.id;

-- name: RankLiveProcedures :many
-- RankLiveProcedures orders the live procedures embedded with a model by
-- how close their description is to a query vector, closest first.
SELECT id, (1 - (description_embedding <=> (@query::text)::halfvec))::float8 AS similarity FROM procedures
WHERE workspace_id = @workspace_id AND status = 'live' AND deleted_at IS NULL AND live IS NOT NULL
    AND description_embedding IS NOT NULL AND embedding_model_key = @model_key
ORDER BY description_embedding <=> (@query::text)::halfvec
LIMIT @row_limit;

-- name: SetProcedureEmbedding :exec
-- SetProcedureEmbedding keeps the vector of a procedure's description.
UPDATE procedures SET description_embedding = (sqlc.narg(embedding)::text)::halfvec,
    embedding_model_key = sqlc.narg(model_key)
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetProcedureLiveVersion :one
-- SetProcedureLiveVersion names the version dimmi follows.
UPDATE procedures SET live_version_id = @version_id
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: RestoreProcedureDraft :one
-- RestoreProcedureDraft puts an old version back as the working copy.
UPDATE procedures SET draft = @draft, lock_version = lock_version + 1, updated_by_member_id = @member_id,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListProcedureNames :many
-- ListProcedureNames returns every procedure's id, name and status.
SELECT id, name, status FROM procedures WHERE workspace_id = @workspace_id AND deleted_at IS NULL;
