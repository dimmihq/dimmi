-- migrate: phase=expand
-- Accounts are deleted by anonymizing the row, so members and messages
-- keep pointing at it; deleted_at marks such a row. created_via records
-- how the account was made, a registry value so a new sign-in provider
-- needs no migration. Risk flags mark cloud signups worth a closer look.

-- +goose Up
ALTER TABLE users
    ADD COLUMN deleted_at timestamptz,
    ADD COLUMN password_changed_at timestamptz,
    ADD COLUMN risk_flags text[] NOT NULL DEFAULT '{}',
    ADD COLUMN created_via text CHECK (created_via ~ '^[a-z][a-z0-9_]{1,39}$');

-- +goose Down
ALTER TABLE users
    DROP COLUMN created_via,
    DROP COLUMN risk_flags,
    DROP COLUMN password_changed_at,
    DROP COLUMN deleted_at;
