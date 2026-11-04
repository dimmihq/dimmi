-- migrate: phase=expand
-- An email domain is a customer's own domain verified for sending, so
-- replies come from support@acme.com instead of the shared default
-- sender. A domain may be added by several workspaces while unproven,
-- but once verified it belongs to one workspace on the whole install:
-- only that workspace may send as it. Records and warnings keep what
-- the last check expected and saw, so the page explains what to fix.

-- +goose Up
CREATE TABLE email_domains (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    domain citext NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'verifying', 'verified', 'degraded', 'failed')),
    driver text NOT NULL CHECK (driver IN ('ses', 'smtp', 'postmark', 'resend', 'mailgun', 'sendgrid')),
    provider_trusted_at timestamptz,
    ownership_token text NOT NULL,
    ownership_verified_at timestamptz,
    dkim_selector text,
    dkim_private_key_encrypted bytea,
    dkim_tokens text[],
    dkim_signing_hosted_zone text,
    mail_from_domain citext,
    ses_identity_status text,
    records jsonb NOT NULL DEFAULT '[]',
    warnings jsonb NOT NULL DEFAULT '[]',
    check_attempts integer NOT NULL DEFAULT 0,
    last_checked_at timestamptz,
    next_check_at timestamptz,
    verified_at timestamptz,
    degraded_at timestamptz,
    failed_at timestamptz,
    failure_reason text,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX email_domains_workspace_id_domain_key ON email_domains (workspace_id, domain)
    WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX email_domains_verified_domain_key ON email_domains (domain)
    WHERE status IN ('verified', 'degraded') AND deleted_at IS NULL;
CREATE INDEX email_domains_next_check_at_index ON email_domains (next_check_at)
    WHERE status IN ('verifying', 'verified', 'degraded') AND deleted_at IS NULL;
CREATE UNIQUE INDEX email_domains_ownership_token_key ON email_domains (ownership_token);

-- +goose Down
DROP TABLE email_domains;
