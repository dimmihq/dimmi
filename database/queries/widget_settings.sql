-- name: ResolveWidgetApp :one
-- ResolveWidgetApp finds whose live widget an app id names, before any
-- workspace is known.
SELECT workspace_id::uuid, id::uuid FROM widget_resolve_app(@app);

-- name: CreateWidgetSettings :one
-- CreateWidgetSettings stores a new widget on its channel connection.
INSERT INTO widget_settings (workspace_id, channel_connection_id, name, created_at, updated_at)
VALUES (@workspace_id, @channel_connection_id, @name, @now, @now)
RETURNING *;

-- name: FindWidgetSettings :one
-- FindWidgetSettings returns the live widget with the id in the workspace.
SELECT * FROM widget_settings WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: FindWidgetSettingsByConnection :one
-- FindWidgetSettingsByConnection returns the widget that owns the channel
-- connection, deleted or not.
SELECT * FROM widget_settings WHERE workspace_id = @workspace_id AND channel_connection_id = @channel_connection_id;

-- name: ListWidgetSettings :many
-- ListWidgetSettings returns the workspace's live widgets, oldest first.
SELECT * FROM widget_settings WHERE workspace_id = @workspace_id AND deleted_at IS NULL ORDER BY created_at, id;

-- name: CountWidgetSettings :one
-- CountWidgetSettings counts the workspace's live widgets.
SELECT count(*) FROM widget_settings WHERE workspace_id = @workspace_id AND deleted_at IS NULL;

-- name: DeleteWidgetSettings :execrows
-- DeleteWidgetSettings soft deletes a live widget.
UPDATE widget_settings SET deleted_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: SaveWidgetSettings :one
-- SaveWidgetSettings writes a widget's whole settings and moves its version
-- on, so a cached config of the old version is never served again.
UPDATE widget_settings SET
    name = @name, enabled = @enabled, appearance = @appearance, texts = @texts, locale = @locale,
    reply_time = @reply_time, show_team_avatars = @show_team_avatars, show_availability = @show_availability,
    allowed_origins = @allowed_origins, allow_anonymous = @allow_anonymous, collect_email = @collect_email,
    require_email_when_offline = @require_email_when_offline, attachments_enabled = @attachments_enabled,
    max_attachment_mb = @max_attachment_mb, help_center_id = @help_center_id, help_tab_enabled = @help_tab_enabled,
    changelog_tab_enabled = @changelog_tab_enabled, intercom_shim = @intercom_shim,
    native_apps_enabled = @native_apps_enabled, show_branding = @show_branding,
    config_version = config_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: BumpWidgetConfigVersion :one
-- BumpWidgetConfigVersion moves a widget's version on when something outside
-- its settings changes what its config answers.
UPDATE widget_settings SET config_version = config_version + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING config_version;

-- name: UpdateWidgetLastBoot :exec
-- UpdateWidgetLastBoot records when and where the widget was last seen,
-- keeping the newest of what every replica saw.
UPDATE widget_settings SET last_boot_at = @seen_at::timestamptz, last_boot_origin = @origin::text
WHERE workspace_id = @workspace_id AND id = @id AND (last_boot_at IS NULL OR last_boot_at < @seen_at::timestamptz);

-- name: ListWidgetTeam :many
-- ListWidgetTeam returns the workspace's active members as a visitor may see
-- them, those who changed something latest first.
SELECT m.user_id, m.availability, u.name, u.avatar_path
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = @workspace_id AND m.status = 'active'
ORDER BY m.updated_at DESC, m.id
LIMIT 200;

-- name: SetChannelConnectionStatus :exec
-- SetChannelConnectionStatus marks a connection, such as a deleted widget's
-- now disconnected one.
UPDATE channel_connections SET status = @status, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: MoveChannelConnectionToInbox :exec
-- MoveChannelConnectionToInbox sends a connection's new conversations to
-- another inbox; those it already started stay where they are.
UPDATE channel_connections SET inbox_id = @inbox_id, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;
