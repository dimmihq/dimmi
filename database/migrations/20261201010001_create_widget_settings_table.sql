-- migrate: phase=expand
-- A widget is one messenger install: its look, its words, who may load it
-- and which inbox it feeds. Its id is the app id customers paste in their
-- snippet. Each widget owns one channel connection, created with it, so
-- conversations reach the inbox the way every other channel's do. A deleted
-- widget keeps its row so conversations that started on it still name it.

-- +goose Up
CREATE TABLE widget_settings (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    name text NOT NULL DEFAULT 'Website',
    enabled boolean NOT NULL DEFAULT true,
    appearance jsonb NOT NULL DEFAULT '{}',
    texts jsonb NOT NULL DEFAULT '{}',
    locale text NOT NULL DEFAULT 'auto',
    reply_time text NOT NULL DEFAULT 'auto' CHECK (reply_time IN ('auto', 'minutes', 'hours', 'day')),
    show_team_avatars boolean NOT NULL DEFAULT true,
    show_availability boolean NOT NULL DEFAULT true,
    allowed_origins text[] NOT NULL DEFAULT '{}',
    allow_anonymous boolean NOT NULL DEFAULT true,
    collect_email text NOT NULL DEFAULT 'offline' CHECK (collect_email IN ('never', 'offline', 'always')),
    require_email_when_offline boolean NOT NULL DEFAULT false,
    attachments_enabled boolean NOT NULL DEFAULT true,
    max_attachment_mb integer NOT NULL DEFAULT 10 CHECK (max_attachment_mb BETWEEN 1 AND 25),
    help_center_id uuid,
    help_tab_enabled boolean NOT NULL DEFAULT false,
    show_branding boolean,
    config_version bigint NOT NULL DEFAULT 1,
    last_boot_at timestamptz,
    last_boot_origin text,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX widget_settings_channel_connection_id_key ON widget_settings (channel_connection_id);
CREATE INDEX widget_settings_workspace_id_deleted_at_index ON widget_settings (workspace_id, deleted_at);

ALTER TABLE widget_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY widget_settings_tenant ON widget_settings USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE widget_settings;
