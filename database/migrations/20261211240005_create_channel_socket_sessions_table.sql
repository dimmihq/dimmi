-- migrate: phase=expand
-- A long-lived connection to a chat app, such as a Slack socket, must be
-- held by exactly one replica, or every event would arrive twice. A
-- replica holds it by a lease it renews every few seconds; a replica
-- that dies stops renewing and another takes the lease once it expires.
-- The row belongs to no workspace: it is the install's infrastructure,
-- and never holds credentials.

-- +goose Up
CREATE TABLE channel_socket_sessions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    kind text NOT NULL CHECK (kind IN ('discord_gateway', 'slack_socket', 'telegram_polling')),
    key text NOT NULL,
    owner text,
    lease_expires_at timestamptz,
    state jsonb NOT NULL DEFAULT '{}',
    connected_at timestamptz,
    last_event_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX channel_socket_sessions_kind_key_key ON channel_socket_sessions (kind, key);

-- +goose Down
DROP TABLE channel_socket_sessions;
