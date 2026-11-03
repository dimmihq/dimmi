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
