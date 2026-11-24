-- migrate: phase=contract
-- A suppression kept after its customer was erased holds a hash and no
-- address, so the address may now be empty; every suppression holds one
-- or the other, never both. The check is added unvalidated and
-- validated after, so writers are never blocked while rows are read.

-- +goose Up
ALTER TABLE email_suppressions ALTER COLUMN email DROP NOT NULL;
ALTER TABLE email_suppressions ADD CONSTRAINT email_suppressions_address_check
    CHECK ((email IS NULL) <> (email_hash IS NULL)) NOT VALID;
ALTER TABLE email_suppressions VALIDATE CONSTRAINT email_suppressions_address_check;

-- +goose Down
DELETE FROM email_suppressions WHERE email IS NULL;
ALTER TABLE email_suppressions DROP CONSTRAINT email_suppressions_address_check;
ALTER TABLE email_suppressions ALTER COLUMN email SET NOT NULL;
