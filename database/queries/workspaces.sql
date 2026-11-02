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
-- CountWorkspacesWithSlug says whether a slug is taken, by any workspace
-- in the trash included, since a trashed workspace can be restored.
SELECT count(*) FROM workspaces WHERE slug = @slug;
