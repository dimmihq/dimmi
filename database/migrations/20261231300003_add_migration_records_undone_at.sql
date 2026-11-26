-- migrate: phase=expand
-- An undo keeps the ledger rows of what it removed, marked undone, so a
-- later migration of the same account can bring back the very contacts
-- and companies the undo trashed instead of clashing with their ids.

-- +goose Up
ALTER TABLE migration_records ADD COLUMN undone_at timestamptz;

-- +goose Down
ALTER TABLE migration_records DROP COLUMN undone_at;
