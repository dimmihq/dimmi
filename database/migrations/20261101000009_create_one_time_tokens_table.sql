-- migrate: phase=expand
-- lint:user-scoped
-- One-time tokens are short-lived secrets sent by mail or held in a
-- cookie: magic links, two-factor challenges, email verification and
-- change, confirmation codes. Only digests are stored. A token is spent
-- by one UPDATE that also checks it is unspent and unexpired, so two
-- requests can never both spend it. The user is unknown for a magic link
-- that signs a new person up.

-- +goose Up
CREATE TABLE one_time_tokens (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    purpose text NOT NULL CHECK (purpose IN (
        'magic_link', 'two_factor_challenge', 'email_verification', 'email_change', 'confirm_code'
    )),
    user_id uuid REFERENCES users ON DELETE CASCADE,
    email citext NOT NULL,
    token_hash bytea NOT NULL,
    code_hash bytea,
    attempts smallint NOT NULL DEFAULT 0,
    data jsonb NOT NULL DEFAULT '{}',
    ip inet,
    user_agent text,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT one_time_tokens_token_hash_key UNIQUE (token_hash)
);

CREATE INDEX one_time_tokens_email_purpose_created_at_index ON one_time_tokens (email, purpose, created_at DESC);
CREATE INDEX one_time_tokens_user_id_purpose_index ON one_time_tokens (user_id, purpose);
CREATE INDEX one_time_tokens_expires_at_index ON one_time_tokens (expires_at);

-- +goose Down
DROP TABLE one_time_tokens;
