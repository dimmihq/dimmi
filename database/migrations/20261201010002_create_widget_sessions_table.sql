-- migrate: phase=expand
-- A widget session is a visitor's credential: only the hash of its bearer
-- token is stored. Anonymous sessions slide their expiry with activity;
-- identified ones live only as long as their last verified identity token
-- allows. The visitor's page and browser details travel with it, with the
-- address cut to a network prefix, so the full address is never kept.

-- +goose Up
CREATE TABLE widget_sessions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    widget_settings_id uuid NOT NULL REFERENCES widget_settings ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    kind text NOT NULL CHECK (kind IN ('anonymous', 'identified')),
    test boolean NOT NULL DEFAULT false,
    identity_secret_id uuid,
    external_id text,
    verified_at timestamptz,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    revoke_reason text CHECK (revoke_reason IN (
        'logout', 'identity_switch', 'contact_merged', 'contact_erased', 'secret_revoked', 'expired',
        'widget_deleted', 'bot', 'admin'
    )),
    bot_score smallint NOT NULL DEFAULT 0,
    pow_solved_at timestamptz,
    origin text,
    user_agent text,
    ip_prefix inet,
    country text,
    locale text,
    timezone text,
    current_page jsonb,
    recent_pages jsonb NOT NULL DEFAULT '[]',
    context jsonb NOT NULL DEFAULT '{}',
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX widget_sessions_token_hash_key ON widget_sessions (token_hash);
CREATE INDEX widget_sessions_workspace_id_contact_id_index ON widget_sessions (workspace_id, contact_id)
    WHERE revoked_at IS NULL;
CREATE INDEX widget_sessions_workspace_id_identity_secret_id_index ON widget_sessions (workspace_id, identity_secret_id)
    WHERE revoked_at IS NULL;
CREATE INDEX widget_sessions_workspace_id_expires_at_index ON widget_sessions (workspace_id, expires_at);

ALTER TABLE widget_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY widget_sessions_tenant ON widget_sessions USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE widget_sessions;
