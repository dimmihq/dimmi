-- name: ListHelpCenters :many
-- ListHelpCenters returns the workspace's help centers, the default first.
SELECT * FROM help_centers
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
ORDER BY is_default DESC, created_at, id;

-- name: FindHelpCenter :one
-- FindHelpCenter returns one live help center of the workspace.
SELECT * FROM help_centers WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: FindDefaultHelpCenter :one
-- FindDefaultHelpCenter returns the workspace's default help center.
SELECT * FROM help_centers WHERE workspace_id = @workspace_id AND is_default AND deleted_at IS NULL;

-- name: CreateHelpCenter :one
-- CreateHelpCenter stores a new help center.
INSERT INTO help_centers (id, workspace_id, name, subdomain, locale, is_default, settings, created_at, updated_at)
VALUES (@id, @workspace_id, @name, @subdomain, @locale, @is_default, @settings, @now, @now)
RETURNING *;

-- name: UpdateHelpCenter :one
-- UpdateHelpCenter writes a help center's editable fields and moves its
-- cache version on.
UPDATE help_centers SET
    name = @name, subdomain = @subdomain, locale = @locale, status = @status, settings = @settings,
    indexing_enabled = @indexing_enabled, ask_ai_enabled = @ask_ai_enabled, cache_version = cache_version + 1,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: TouchHelpCenterCache :exec
-- TouchHelpCenterCache moves the cache version on after a change readers
-- can see.
UPDATE help_centers SET cache_version = cache_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ResolveHelpCenter :one
-- ResolveHelpCenter finds which workspace's help center answers a
-- subdomain, before any workspace is known.
SELECT r.workspace_id::uuid AS workspace_id, r.id::uuid AS id FROM help_center_resolve(@subdomain::citext) AS r;

-- name: HelpCenterSubdomainTaken :one
-- HelpCenterSubdomainTaken reports whether any workspace uses the
-- subdomain.
SELECT help_center_subdomain_taken(@subdomain::citext)::boolean AS taken;

-- name: ListCollections :many
-- ListCollections returns a help center's live collections in order.
SELECT * FROM collections
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL
ORDER BY parent_id NULLS FIRST, position, id;

-- name: FindCollection :one
-- FindCollection returns one live collection.
SELECT * FROM collections WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: FindCollectionBySlug :one
-- FindCollectionBySlug returns the live collection with the slug.
SELECT * FROM collections
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND slug = @slug AND deleted_at IS NULL;

-- name: NextCollectionPosition :one
-- NextCollectionPosition is where a new collection goes: after the last.
SELECT (coalesce(max(position), 0) + 1024)::integer AS position FROM collections
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id AND deleted_at IS NULL;

-- name: CreateCollection :one
-- CreateCollection stores a new collection.
INSERT INTO collections (
    id, workspace_id, help_center_id, parent_id, name, slug, description, icon, position, created_at, updated_at
) VALUES (
    @id, @workspace_id, @help_center_id, @parent_id, @name, @slug, @description, @icon, @position, @now, @now
)
RETURNING *;

-- name: UpdateCollection :one
-- UpdateCollection writes a collection's editable fields.
UPDATE collections SET
    parent_id = @parent_id, name = @name, slug = @slug, description = @description, icon = @icon,
    position = @position, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteCollection :exec
-- DeleteCollection moves a collection to the trash and frees its slug's
-- articles to be uncollected.
UPDATE collections SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: UncollectArticles :exec
-- UncollectArticles takes articles out of a collection going away.
UPDATE articles SET collection_id = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND collection_id = @collection_id;

-- name: CountChildCollections :one
-- CountChildCollections counts the live sub-collections of a collection.
SELECT count(*) FROM collections WHERE workspace_id = @workspace_id AND parent_id = @parent_id AND deleted_at IS NULL;

-- name: PointWidgetsAtHelpCenter :exec
-- PointWidgetsAtHelpCenter gives the workspace's messengers without a
-- help center the one given.
UPDATE widget_settings SET help_center_id = @help_center_id, updated_at = @now
WHERE workspace_id = @workspace_id AND help_center_id IS NULL;

