-- migrate: phase=contract
-- A signature may now belong to an address alone: the team's sign-off
-- for every reply from it, used when the teammate has none of their own,
-- and the only one an agent's reply carries. So the member becomes
-- optional, though every signature still names a member or an address.
-- One signature per member and address pair in a workspace, where a
-- missing member or address counts as a value of its own. A signature
-- may show one small logo, kept as an attachment; removing the file
-- leaves the signature without it.

-- +goose NO TRANSACTION
-- +goose Up
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS email_signatures_scope_key ON email_signatures
    (workspace_id, workspace_member_id, email_address_id) NULLS NOT DISTINCT;

ALTER TABLE email_signatures
    ALTER COLUMN workspace_member_id DROP NOT NULL,
    ADD COLUMN logo_attachment_id uuid REFERENCES attachments ON DELETE SET NULL,
    DROP CONSTRAINT email_signatures_workspace_member_id_email_address_id_key,
    ADD CONSTRAINT email_signatures_scope_check
        CHECK (workspace_member_id IS NOT NULL OR email_address_id IS NOT NULL) NOT VALID;

ALTER TABLE email_signatures VALIDATE CONSTRAINT email_signatures_scope_check;

-- +goose Down
DELETE FROM email_signatures WHERE workspace_member_id IS NULL;

ALTER TABLE email_signatures
    DROP CONSTRAINT email_signatures_scope_check,
    ADD CONSTRAINT email_signatures_workspace_member_id_email_address_id_key
        UNIQUE NULLS NOT DISTINCT (workspace_member_id, email_address_id),
    DROP COLUMN logo_attachment_id,
    ALTER COLUMN workspace_member_id SET NOT NULL;

DROP INDEX IF EXISTS email_signatures_scope_key;
