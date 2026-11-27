-- name: NextHelpCenterThemeVersion :one
-- NextHelpCenterThemeVersion is the number the next upload of a help
-- center's theme gets, locking the help center so two uploads never take
-- the same one.
SELECT coalesce(max(t.version), 0)::integer + 1 AS version
FROM help_center_themes t
WHERE t.workspace_id = @workspace_id AND t.help_center_id = @help_center_id;

-- name: CreateHelpCenterTheme :one
-- CreateHelpCenterTheme stores a version of a help center's theme.
INSERT INTO help_center_themes (
    id, workspace_id, help_center_id, version, name, manifest, content_hash, file_count, size_bytes, created_by,
    created_at, updated_at
) VALUES (
    @id, @workspace_id, @help_center_id, @version, @name, @manifest, @content_hash, @file_count, @size_bytes,
    sqlc.narg(created_by), @now::timestamptz, @now::timestamptz
)
RETURNING *;

-- name: CreateHelpCenterThemeFile :exec
-- CreateHelpCenterThemeFile stores one file of a theme version.
INSERT INTO help_center_theme_files (theme_id, workspace_id, path, body)
VALUES (@theme_id, @workspace_id, @path, @body);

-- name: ListHelpCenterThemes :many
-- ListHelpCenterThemes lists a help center's theme versions, newest
-- first.
SELECT * FROM help_center_themes
WHERE workspace_id = @workspace_id AND help_center_id = @help_center_id
ORDER BY version DESC
LIMIT 100;

-- name: FindHelpCenterTheme :one
-- FindHelpCenterTheme returns one theme version of the workspace.
SELECT * FROM help_center_themes WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListHelpCenterThemeFiles :many
-- ListHelpCenterThemeFiles returns every file of a theme version.
SELECT path, body FROM help_center_theme_files
WHERE workspace_id = @workspace_id AND theme_id = @theme_id
ORDER BY path;

-- name: UpdateHelpCenterThemeSettings :one
-- UpdateHelpCenterThemeSettings stores the team's values for a theme's
-- settings.
UPDATE help_center_themes SET settings = @settings, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SetHelpCenterTheme :one
-- SetHelpCenterTheme makes a theme version the one a help center's pages
-- wear, or the default theme when theme_id is null, and starts its
-- readers' pages over.
UPDATE help_centers SET theme_id = sqlc.narg(theme_id), cache_version = cache_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: TouchHelpCentersWearingTheme :exec
-- TouchHelpCentersWearingTheme starts over the pages of the help center
-- wearing a theme version whose settings changed.
UPDATE help_centers SET cache_version = cache_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND theme_id = @theme_id;
