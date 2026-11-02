-- migrate: phase=expand
-- lint:user-scoped
-- A user has at most one live password reset. Asking again replaces the
-- row, so the earlier link and its id stop working at once. Wrong
-- guesses are counted on the row, which is deleted at the fifth.

-- +goose Up
CREATE TABLE password_reset_tokens (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    requested_ip inet,
    attempts smallint NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT password_reset_tokens_user_id_key UNIQUE (user_id),
    CONSTRAINT password_reset_tokens_token_hash_key UNIQUE (token_hash)
);

-- +goose Down
DROP TABLE password_reset_tokens;
