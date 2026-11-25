-- migrate: phase=contract
-- A data export can also be the whole workspace: one zip of every
-- record the workspace owns, as newline-delimited JSON, with its files.

-- +goose Up
ALTER TABLE data_exports DROP CONSTRAINT data_exports_kind_check;
ALTER TABLE data_exports ADD CONSTRAINT data_exports_kind_check
    CHECK (kind IN ('conversations', 'contacts', 'conversation_facts', 'workspace')) NOT VALID;
ALTER TABLE data_exports VALIDATE CONSTRAINT data_exports_kind_check;
ALTER TABLE data_exports DROP CONSTRAINT data_exports_format_check;
ALTER TABLE data_exports ADD CONSTRAINT data_exports_format_check
    CHECK (format IN ('txt', 'pdf', 'csv', 'json', 'zip')) NOT VALID;
ALTER TABLE data_exports VALIDATE CONSTRAINT data_exports_format_check;

-- +goose Down
DELETE FROM data_exports WHERE kind = 'workspace';
ALTER TABLE data_exports DROP CONSTRAINT data_exports_kind_check;
ALTER TABLE data_exports ADD CONSTRAINT data_exports_kind_check
    CHECK (kind IN ('conversations', 'contacts', 'conversation_facts'));
ALTER TABLE data_exports DROP CONSTRAINT data_exports_format_check;
ALTER TABLE data_exports ADD CONSTRAINT data_exports_format_check CHECK (format IN ('txt', 'pdf', 'csv', 'json'));
