-- name: CreateTeam :one
-- CreateTeam adds a team to the workspace.
INSERT INTO teams (workspace_id, name, created_at, updated_at)
VALUES (@workspace_id, @name, @now, @now)
RETURNING *;

-- name: ListTeams :many
-- ListTeams returns a page of the workspace's live teams by name, each
-- with how many members it has.
SELECT sqlc.embed(t), (SELECT count(*) FROM team_members tm WHERE tm.team_id = t.id)::bigint AS member_count
FROM teams t
WHERE t.workspace_id = @workspace_id AND t.deleted_at IS NULL
ORDER BY lower(t.name), t.id
LIMIT @row_limit::bigint OFFSET @row_offset::bigint;

-- name: FindTeam :one
-- FindTeam returns the live team with the id in the workspace, with how
-- many members it has. A deleted team is not found, so nothing new can
-- be assigned to it.
SELECT sqlc.embed(t), (SELECT count(*) FROM team_members tm WHERE tm.team_id = t.id)::bigint AS member_count
FROM teams t
WHERE t.workspace_id = @workspace_id AND t.id = @id AND t.deleted_at IS NULL;

-- name: LockTeam :one
-- LockTeam returns the live team and holds its row until the transaction
-- ends, so it cannot be deleted while it changes.
SELECT * FROM teams WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL FOR UPDATE;

-- name: ListLiveTeamIDs :many
-- ListLiveTeamIDs returns which of the ids name live teams of the
-- workspace, for callers that must skip deleted or foreign teams.
SELECT id FROM teams WHERE workspace_id = @workspace_id AND id = ANY (@ids::uuid[]) AND deleted_at IS NULL;

-- name: RenameTeam :one
-- RenameTeam gives the team a new name.
UPDATE teams SET name = @name, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SoftDeleteTeam :one
-- SoftDeleteTeam hides the team and frees its name at once; the row
-- stays until its conversations are unassigned and it is purged.
UPDATE teams SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: PurgeTeam :execrows
-- PurgeTeam removes a team that was deleted, with its memberships.
DELETE FROM teams WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NOT NULL;
