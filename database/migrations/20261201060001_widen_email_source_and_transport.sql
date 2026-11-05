-- migrate: phase=contract
-- Mail now reaches the install from several providers and from connected
-- mailboxes, and replies can leave through those mailboxes. Both columns
-- keep a closed list, so a misspelled source or transport still fails at
-- the database. The checks only widen: every row the old release writes
-- still passes them. Each wider check is added unvalidated and validated
-- after, so writers are never blocked while existing rows are read. Going
-- down restores the narrow checks unvalidated, so rows from the new
-- sources are kept rather than refused.

-- +goose Up
ALTER TABLE inbound_emails DROP CONSTRAINT inbound_emails_source_check;
ALTER TABLE inbound_emails ADD CONSTRAINT inbound_emails_source_check CHECK (source IN (
    'ses', 'postmark', 'mailgun', 'sendgrid', 'resend', 'cloudflare', 'imap', 'gmail', 'microsoft', 'test'
)) NOT VALID;
ALTER TABLE inbound_emails VALIDATE CONSTRAINT inbound_emails_source_check;

ALTER TABLE outbound_emails DROP CONSTRAINT outbound_emails_transport_check;
ALTER TABLE outbound_emails ADD CONSTRAINT outbound_emails_transport_check CHECK (transport IN (
    'ses', 'smtp', 'postmark', 'resend', 'mailgun', 'sendgrid', 'log', 'gmail', 'microsoft'
)) NOT VALID;
ALTER TABLE outbound_emails VALIDATE CONSTRAINT outbound_emails_transport_check;

-- +goose Down
ALTER TABLE outbound_emails DROP CONSTRAINT outbound_emails_transport_check;
ALTER TABLE outbound_emails ADD CONSTRAINT outbound_emails_transport_check CHECK (
    transport IN ('ses', 'smtp', 'postmark', 'resend', 'mailgun', 'sendgrid', 'log')
) NOT VALID;

ALTER TABLE inbound_emails DROP CONSTRAINT inbound_emails_source_check;
ALTER TABLE inbound_emails ADD CONSTRAINT inbound_emails_source_check CHECK (source IN ('ses', 'test')) NOT VALID;
