-- name: ListProcedures :many
-- ListProcedures returns the workspace's procedures by name.
SELECT * FROM procedures WHERE workspace_id = @workspace_id AND deleted_at IS NULL ORDER BY lower(name), id;

-- name: ListLiveProcedures :many
-- ListLiveProcedures returns the procedures dimmi may follow.
SELECT id, name, live FROM procedures
WHERE workspace_id = @workspace_id AND status = 'live' AND deleted_at IS NULL AND live IS NOT NULL
ORDER BY lower(name), id;

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
