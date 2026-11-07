-- migrate: phase=expand
-- An anonymous messenger visitor can prove they own an email address by
-- typing a code mailed to it. The proof belongs to the session, not the
-- visitor's word: it records which address was proven and when, and
-- lasts only a while, so agent actions bound to the customer's email run
-- for someone who showed they read that inbox recently.

-- +goose Up
ALTER TABLE widget_sessions
    ADD COLUMN verified_email citext,
    ADD COLUMN email_verified_at timestamptz,
    ADD CONSTRAINT widget_sessions_email_proof_check
        CHECK ((verified_email IS NULL) = (email_verified_at IS NULL));

-- +goose Down
ALTER TABLE widget_sessions
    DROP CONSTRAINT widget_sessions_email_proof_check,
    DROP COLUMN email_verified_at,
    DROP COLUMN verified_email;
