-- name: CreateWorkspace :one
-- CreateWorkspace stores a workspace in its organization. The caller
-- picks the id, so it can scope the rest of the transaction to the new
-- workspace. The slug is unique without regard to case, so a taken slug
-- fails on workspaces_slug_key.
INSERT INTO workspaces (
    id, organization_id, name, slug, timezone, locale, test_mode, created_by_user_id, created_at, updated_at
) VALUES (
    @id, @organization_id, @name, @slug, @timezone, @locale, @test_mode, @created_by_user_id, @now, @now
)
RETURNING *;

-- name: FindWorkspace :one
-- FindWorkspace returns the workspace with the id, deleted or not.
SELECT * FROM workspaces WHERE id = @id;

-- name: CountWorkspacesWithSlug :one
-- CountWorkspacesWithSlug says whether a slug is taken by a workspace
-- other than the one named: as its slug, by a workspace in the trash
-- included, since a trashed workspace can be restored, or as a slug it
-- gave up less than the hold period ago, which still sends people to it.
SELECT count(*)
FROM workspaces w
WHERE w.id <> @workspace_id
  AND (
    w.slug = @slug
    OR EXISTS (
        SELECT 1
        FROM jsonb_array_elements(coalesce(w.settings -> 'previous_slugs', '[]'::jsonb)) AS previous
        WHERE lower(previous ->> 'slug') = lower(@slug::text) AND (previous ->> 'until')::timestamptz > @now::timestamptz
    )
  );

-- name: LockWorkspace :one
-- LockWorkspace returns the workspace and holds it until the transaction
-- ends, so changes to who owns it happen one after another.
SELECT * FROM workspaces WHERE id = @id FOR UPDATE;

-- name: UpdateWorkspace :one
-- UpdateWorkspace stores the workspace's name, slug, language, time zone
-- and settings.
UPDATE workspaces
SET name = @name, slug = @slug, timezone = @timezone, locale = @locale, settings = @settings, updated_at = @now
WHERE id = @id
RETURNING *;

-- name: ListWorkspaces :many
-- ListWorkspaces returns every workspace not in the trash, oldest first.
SELECT * FROM workspaces WHERE deleted_at IS NULL ORDER BY created_at, id;

-- name: FindWorkspaceBySlug :one
-- FindWorkspaceBySlug returns the workspace with the slug, deleted or
-- not.
SELECT * FROM workspaces WHERE slug = @slug;

-- name: TrashWorkspace :one
-- TrashWorkspace moves the workspace to the trash until purge_after,
-- recording who deleted it. A reminder sent for an earlier deletion is
-- forgotten, so this deletion gets its own.
UPDATE workspaces
SET deleted_at = @now::timestamptz, purge_after = @purge_after::timestamptz, deleted_by_user_id = @deleted_by,
    settings = settings - 'deletion_reminded_at', updated_at = @now::timestamptz
WHERE id = @id
RETURNING *;

-- name: RestoreWorkspace :one
-- RestoreWorkspace takes the workspace out of the trash.
UPDATE workspaces
SET deleted_at = NULL, purge_after = NULL, deleted_by_user_id = NULL,
    settings = settings - 'deletion_reminded_at', updated_at = @now
WHERE id = @id
RETURNING *;

-- name: ListWorkspacesDueForPurge :many
-- ListWorkspacesDueForPurge returns the workspaces whose time in the
-- trash ran out, the longest overdue first.
SELECT id FROM workspaces
WHERE deleted_at IS NOT NULL AND purge_after <= @now::timestamptz
ORDER BY purge_after, id;

-- name: ListWorkspacesDueForReminder :many
-- ListWorkspacesDueForReminder returns the workspaces in the trash that
-- will be purged within the notice period and whose owners were not
-- reminded yet.
SELECT * FROM workspaces
WHERE deleted_at IS NOT NULL AND purge_after > @now::timestamptz AND purge_after <= @notice_until::timestamptz
  AND NOT settings ? 'deletion_reminded_at'
ORDER BY purge_after, id;

-- name: MarkWorkspaceReminded :exec
-- MarkWorkspaceReminded records that the owners were told the workspace
-- is about to be purged, so they are told once.
UPDATE workspaces
SET settings = jsonb_set(settings, '{deletion_reminded_at}', to_jsonb(@now::timestamptz))
WHERE id = @id;

-- name: MarkWorkspacePurgeStarted :one
-- MarkWorkspacePurgeStarted records when the purge of a workspace in the
-- trash began, once; from then on it can no longer be restored. It
-- returns nothing for a workspace no longer in the trash.
UPDATE workspaces
SET settings = CASE
        WHEN settings ? 'purge_started_at' THEN settings
        ELSE jsonb_set(settings, '{purge_started_at}', to_jsonb(@now::timestamptz))
    END
WHERE id = @id AND deleted_at IS NOT NULL
RETURNING *;

-- name: DeleteWorkspace :exec
-- DeleteWorkspace removes the workspace row for good, and with it every
-- row still pointing at it.
DELETE FROM workspaces WHERE id = @id;

-- name: CountLiveWorkspaces :one
-- CountLiveWorkspaces counts the workspaces not in the trash.
SELECT count(*) FROM workspaces WHERE deleted_at IS NULL;

-- name: ListAllWorkspaces :many
-- ListAllWorkspaces returns every workspace, those in the trash
-- included, oldest first.
SELECT * FROM workspaces ORDER BY created_at, id;

-- name: SuspendWorkspace :one
-- SuspendWorkspace stops every change to the workspace, recording why.
-- A workspace already suspended keeps the time it was first suspended.
UPDATE workspaces
SET suspended_at = coalesce(suspended_at, @now::timestamptz), suspended_reason = @reason::text,
    updated_at = @now::timestamptz
WHERE id = @id
RETURNING *;

-- name: UnsuspendWorkspace :one
-- UnsuspendWorkspace lets the workspace be changed again.
UPDATE workspaces
SET suspended_at = NULL, suspended_reason = NULL, updated_at = @now
WHERE id = @id
RETURNING *;

-- name: EndWorkspaceGrace :one
-- EndWorkspaceGrace brings the purge of a workspace in the trash forward
-- to now. It returns nothing for a workspace not in the trash.
UPDATE workspaces
SET purge_after = least(purge_after, @now::timestamptz)
WHERE id = @id AND deleted_at IS NOT NULL
RETURNING *;
