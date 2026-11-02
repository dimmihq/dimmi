-- migrate: phase=expand
-- lint:user-scoped
-- Security events are a user's own account log: sign-ins, password and
-- factor changes, revoked sessions. Signing in happens before any
-- workspace is chosen, so the log belongs to the user, not a tenant.
-- Types are dotted registry values such as login.succeeded.

-- +goose Up
CREATE TABLE security_events (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    type text NOT NULL CHECK (type ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$'),
    ip inet,
    user_agent text,
    device_label text,
    country text,
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX security_events_user_id_created_at_index ON security_events (user_id, created_at DESC);

-- +goose Down
DROP TABLE security_events;
