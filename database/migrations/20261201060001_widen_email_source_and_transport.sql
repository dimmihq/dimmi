-- migrate: phase=expand
-- Mail now reaches the install from several providers and from connected
-- mailboxes, and replies can leave through those mailboxes. Both columns
-- keep a closed list, so a misspelled source or transport still fails at
-- the database.

-- +goose Up
ALTER TABLE inbound_emails DROP CONSTRAINT inbound_emails_source_check;
ALTER TABLE inbound_emails ADD CONSTRAINT inbound_emails_source_check CHECK (source IN (
    'ses', 'postmark', 'mailgun', 'sendgrid', 'resend', 'cloudflare', 'imap', 'gmail', 'microsoft', 'test'
));

ALTER TABLE outbound_emails DROP CONSTRAINT outbound_emails_transport_check;
ALTER TABLE outbound_emails ADD CONSTRAINT outbound_emails_transport_check CHECK (transport IN (
    'ses', 'smtp', 'postmark', 'resend', 'mailgun', 'sendgrid', 'log', 'gmail', 'microsoft'
));

-- +goose Down
ALTER TABLE outbound_emails DROP CONSTRAINT outbound_emails_transport_check;
ALTER TABLE outbound_emails ADD CONSTRAINT outbound_emails_transport_check CHECK (
    transport IN ('ses', 'smtp', 'postmark', 'resend', 'mailgun', 'sendgrid', 'log')
);

ALTER TABLE inbound_emails DROP CONSTRAINT inbound_emails_source_check;
ALTER TABLE inbound_emails ADD CONSTRAINT inbound_emails_source_check CHECK (source IN ('ses', 'test'));
