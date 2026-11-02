-- migrate: phase=expand
-- lint:user-scoped
-- An identity links a user to an account at a sign-in provider. The
-- provider's own id is the key; the email it reports is kept for display
-- only, because a provider email can change hands. A user links at most
-- one account per provider. No provider tokens are stored: they are not
-- needed once the user is signed in.

-- +goose Up
CREATE TABLE oauth_identities (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,39}$'),
    provider_user_id text NOT NULL,
    email citext,
    email_verified boolean NOT NULL DEFAULT false,
    username text,
    avatar_url text,
    hosted_domain text,
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT oauth_identities_provider_provider_user_id_key UNIQUE (provider, provider_user_id),
    CONSTRAINT oauth_identities_user_id_provider_key UNIQUE (user_id, provider)
);

-- +goose Down
DROP TABLE oauth_identities;
