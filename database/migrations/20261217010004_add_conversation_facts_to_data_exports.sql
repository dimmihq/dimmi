-- migrate: phase=expand
-- A data export can also be every conversation's report facts, as one
-- spreadsheet, for the range and filters of the reports.

-- +goose Up
ALTER TABLE data_exports DROP CONSTRAINT data_exports_kind_check;
ALTER TABLE data_exports ADD CONSTRAINT data_exports_kind_check
    CHECK (kind IN ('conversations', 'contacts', 'conversation_facts')) NOT VALID;
ALTER TABLE data_exports VALIDATE CONSTRAINT data_exports_kind_check;

-- +goose Down
DELETE FROM data_exports WHERE kind = 'conversation_facts';
ALTER TABLE data_exports DROP CONSTRAINT data_exports_kind_check;
ALTER TABLE data_exports ADD CONSTRAINT data_exports_kind_check CHECK (kind IN ('conversations', 'contacts'));
