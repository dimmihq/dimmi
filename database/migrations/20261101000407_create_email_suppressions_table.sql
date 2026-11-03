-- migrate: phase=expand
-- A suppressed address is never mailed again for its scope: it bounced,
-- complained, unsubscribed or was blocked by hand. A hard bounce stops
-- every kind of mail, while a complaint about one kind of mail stops
-- only that kind. Soft bounces lapse on their own.

-- +goose Up
CREATE TABLE email_suppressions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    email citext NOT NULL,
    reason text NOT NULL
        CHECK (reason IN ('hard_bounce', 'soft_bounce', 'complaint', 'unsubscribed', 'manual', 'invalid')),
    scope text NOT NULL DEFAULT 'all' CHECK (scope IN ('all', 'notifications', 'changelog', 'feedback')),
    source text NOT NULL CHECK (source IN ('ses', 'provider', 'dsn', 'arf', 'one_click', 'link', 'api', 'manual')),
    details jsonb,
    outbound_email_id uuid REFERENCES outbound_emails ON DELETE SET NULL,
    expires_at timestamptz,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT email_suppressions_workspace_id_email_scope_key UNIQUE (workspace_id, email, scope)
);

CREATE INDEX email_suppressions_workspace_id_created_at_index ON email_suppressions (workspace_id, created_at DESC);

-- +goose Down
DROP TABLE email_suppressions;
