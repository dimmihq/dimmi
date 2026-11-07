-- migrate: phase=expand
-- Teammates hear about conversation activity in the dashboard's bell, by
-- email and by browser push. The bell's rows live in the framework's
-- notifications table, which carries the workspace as its tenant; here it
-- gets row level security, and the next migration the bell's indexes. Preferences
-- store only what a member changed from the defaults, which live in code.
-- Settings are one optional row per member. Push subscriptions belong to
-- the browser session that made them, so signing out stops pushes to that
-- device; their auth secret is sealed.

-- +goose Up
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
CREATE POLICY notifications_tenant ON notifications
    USING (tenant_id IS NOT NULL AND tenant_id::uuid = ANY (dimmi_visible_workspaces()));

CREATE TABLE notification_preferences (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    workspace_member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    trigger text NOT NULL CHECK (trigger ~ '^[a-z][a-z0-9_]{1,39}$'),
    channel text NOT NULL CHECK (channel IN ('in_app', 'email', 'push')),
    enabled boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, workspace_member_id, trigger, channel)
);

ALTER TABLE notification_preferences ENABLE ROW LEVEL SECURITY;
CREATE POLICY notification_preferences_tenant ON notification_preferences
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE notification_settings (
    workspace_member_id uuid PRIMARY KEY REFERENCES workspace_members ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    timezone text,
    quiet_hours_enabled boolean NOT NULL DEFAULT false,
    quiet_hours_start text NOT NULL DEFAULT '22:00' CHECK (quiet_hours_start ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    quiet_hours_end text NOT NULL DEFAULT '08:00' CHECK (quiet_hours_end ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    quiet_hours_days smallint NOT NULL DEFAULT 127 CHECK (quiet_hours_days BETWEEN 0 AND 127),
    quiet_hours_bypass text[] NOT NULL DEFAULT '{}',
    paused_until timestamptz,
    show_previews boolean NOT NULL DEFAULT true,
    include_ai_handled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE notification_settings ENABLE ROW LEVEL SECURITY;
CREATE POLICY notification_settings_tenant ON notification_settings
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE push_subscriptions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    session_id uuid REFERENCES sessions ON DELETE CASCADE,
    endpoint text NOT NULL,
    endpoint_hash bytea NOT NULL UNIQUE,
    p256dh text NOT NULL,
    auth_secret_encrypted bytea NOT NULL,
    vapid_key_id text NOT NULL,
    platform text NOT NULL DEFAULT 'other' CHECK (platform IN ('apple', 'google', 'mozilla', 'microsoft', 'other')),
    user_agent text,
    device_label text,
    last_success_at timestamptz,
    last_failure_at timestamptz,
    consecutive_failures integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX push_subscriptions_user_id_index ON push_subscriptions (user_id);

-- +goose Down
DROP TABLE push_subscriptions;
DROP TABLE notification_settings;
DROP TABLE notification_preferences;
DROP POLICY notifications_tenant ON notifications;
ALTER TABLE notifications DISABLE ROW LEVEL SECURITY;
