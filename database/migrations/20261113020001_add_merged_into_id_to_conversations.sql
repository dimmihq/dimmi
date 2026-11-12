-- migrate: phase=expand
-- A conversation merged into another keeps a pointer to it, so a link
-- to the old one can still lead to where its messages went.

-- +goose Up
ALTER TABLE conversations ADD COLUMN merged_into_id uuid REFERENCES conversations ON DELETE SET NULL;

-- +goose Down
ALTER TABLE conversations DROP COLUMN merged_into_id;
