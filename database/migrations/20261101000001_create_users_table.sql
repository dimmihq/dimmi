-- migrate: phase=expand
-- lint:global
-- A user is one person who signs in. People belong to workspaces through
-- workspace_members, so the user row itself belongs to no tenant. Email
-- is compared without regard to case, so Ada@acme.com and ada@acme.com
-- are one account.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name text NOT NULL,
    email citext NOT NULL,
    email_verified_at timestamptz,
    password_hash text,
    two_factor_confirmed_at timestamptz,
    avatar_path text,
    locale text,
    timezone text,
    last_seen_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_key UNIQUE (email)
);

-- +goose Down
DROP TABLE users;
