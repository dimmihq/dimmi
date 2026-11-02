-- migrate: phase=expand
-- lint:user-scoped
-- A session belongs to the browser that holds its cookie and, once the
-- visitor logs in, to a user. It belongs to no tenant: one login reaches
-- every tenant the user is a member of.

-- +goose Up
CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    token_hash bytea NOT NULL UNIQUE,
    user_id uuid,
    data jsonb NOT NULL DEFAULT '{}',
    flash jsonb NOT NULL DEFAULT '{}',
    ip text,
    user_agent text,
    authenticated_at timestamptz,
    confirmed_at timestamptz,
    last_activity_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX sessions_user_id_last_activity_at_index ON sessions (user_id, last_activity_at DESC);
CREATE INDEX sessions_expires_at_index ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
