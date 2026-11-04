-- migrate: phase=contract
-- An admin may mail a locked-out teammate a link that turns their second
-- factor off, so one-time tokens gain the two_factor_reset purpose. The
-- check only widens: every row the old release writes still passes it.
-- The wider check is added unvalidated and validated after, so writers
-- are never blocked while existing rows are read.

-- +goose Up
ALTER TABLE one_time_tokens DROP CONSTRAINT one_time_tokens_purpose_check;
ALTER TABLE one_time_tokens ADD CONSTRAINT one_time_tokens_purpose_check CHECK (purpose IN (
    'magic_link', 'two_factor_challenge', 'email_verification', 'email_change', 'confirm_code', 'two_factor_reset'
)) NOT VALID;
ALTER TABLE one_time_tokens VALIDATE CONSTRAINT one_time_tokens_purpose_check;

-- +goose Down
DELETE FROM one_time_tokens WHERE purpose = 'two_factor_reset';
ALTER TABLE one_time_tokens DROP CONSTRAINT one_time_tokens_purpose_check;
ALTER TABLE one_time_tokens ADD CONSTRAINT one_time_tokens_purpose_check CHECK (purpose IN (
    'magic_link', 'two_factor_challenge', 'email_verification', 'email_change', 'confirm_code'
));
