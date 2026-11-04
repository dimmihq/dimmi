-- migrate: phase=expand
-- A domain may be set up at the sending provider through its API, so the
-- row keeps the provider's id for it. A customer may correct our guess of
-- who hosts their DNS, and a host's automatic setup is remembered so the
-- page can say it was applied. A check needs to know when verification
-- began and when SES was last asked to look again, and a key rotation
-- holds the next key until its record is seen.

-- +goose Up
ALTER TABLE email_domains
    ADD COLUMN provider_domain_id text,
    ADD COLUMN dns_host_override text,
    ADD COLUMN domain_connect_applied_at timestamptz,
    ADD COLUMN verifying_since timestamptz,
    ADD COLUMN ses_dkim_restarted_at timestamptz,
    ADD COLUMN dkim_next_selector text,
    ADD COLUMN dkim_next_private_key_encrypted bytea;

-- +goose Down
ALTER TABLE email_domains
    DROP COLUMN dkim_next_private_key_encrypted,
    DROP COLUMN dkim_next_selector,
    DROP COLUMN ses_dkim_restarted_at,
    DROP COLUMN verifying_since,
    DROP COLUMN domain_connect_applied_at,
    DROP COLUMN dns_host_override,
    DROP COLUMN provider_domain_id;
