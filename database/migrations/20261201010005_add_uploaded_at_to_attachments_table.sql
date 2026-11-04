-- migrate: phase=expand
-- A visitor's upload exists as a row before its bytes arrive. uploaded_at
-- says the bytes arrived and were checked, and a message only accepts rows
-- where it is set. Everything already stored has arrived.

-- +goose Up
ALTER TABLE attachments ADD COLUMN uploaded_at timestamptz;
UPDATE attachments SET uploaded_at = created_at;

-- +goose Down
ALTER TABLE attachments DROP COLUMN uploaded_at;
